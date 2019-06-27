// Copyright 2019. Akamai Technologies, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
        "time"

        "github.com/akamai/AkamaiOPEN-edgegrid-golang/reportsgtm-v1"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	//"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

var domainname       string
var qsProperty       string
var qsDatacenters    *arrayFlags
var statusPeriodLen = "15m"
var qsNicknames      []string

// Data Center Traffic Status returned structure. Contains a list of individual DC stati.
type DCTrafficStati struct {
        Domain                    string
        PeriodStart               string
        PeriodEnd                 string
        StatusByDatacenter        []*DCStatusDetail
}

// Individual data center traffic status. 
type DCStatusDetail struct {
        DatacenterId              int
        DatacenterNickname        string
        ReportInterval            string
        DCStatusByProperty        []*reportsgtm.DCTData
}

// Property Status returned structure. Contains a ...
type PropertyStatus struct { 
        Domain                    string
        PropertyName              string
        WindowStart               string
        WindowEnd                 string
        ReportInterval            string
        StatusSummary             *PropertyStatusSummary
        DatacenterIntervalStatus  []*reportsgtm.PropertyTData
}

// Property IP Status Summary struct
type PropertyStatusSummary struct {
        LastUpdate                string
        CutOff                    float64
        PropertyDCStatus          []*PropertyDCStatus
}

// Property DC Status Summary struct
type PropertyDCStatus struct {
        reportsgtm.IpStatPerPropDRow
        DCTotalPeriodRequests     int64
        DCPropertyUsage           string
}

var defaultPeriod time.Duration = 15*60*1000*1000

// Calc period start and end. Input string specifying duration, e.g. 15m. Returns formatted strings consumable by GTM Reports API.
func calcPeriodStartandEnd(trafficType string, periodLen string) (string, string) {

        isoStart := "01-01-1980T00:00:00Z"
        isoEnd := "01-01-1980T00:00:00Z"

        dur, err := time.ParseDuration(periodLen)
        if err != nil {
		dur = defaultPeriod
        }

        var window *reportsgtm.WindowResponse

        if trafficType == "datacenter" {
                window, err = reportsgtm.GetDatacentersTrafficWindow()
        } else if trafficType == "property" {
                window, err = reportsgtm.GetPropertiesTrafficWindow()
        } else {
                // shouldn't get here. If so, return invalid date 
                return isoStart, isoEnd
        }
        if err != nil {
               // return invalid dates
               return isoStart, isoEnd
       }
       end := window.EndTime
       start := end.Add(-dur)

       return start.Format(time.RFC3339), end.Format(time.RFC3339)

}

func gatherDatacenterStatus() (*DCTrafficStati, error) {

        dcTrafficStati := &DCTrafficStati{Domain: domainname}
        // calc period start and end
        pstart, pend := calcPeriodStartandEnd("datacenter", statusPeriodLen)
        dcTrafficStati.PeriodStart = pstart
        dcTrafficStati.PeriodEnd = pend 
        optArgs := make(map[string]string)
        optArgs["start"] = pstart
        optArgs["end"] = pend

        // Looping on DCs
        for _, dcid := range qsDatacenters.flagList {
                dcTStatus, err := reportsgtm.GetTrafficPerDatacenter(domainname, dcid, optArgs)
                if err != nil {
                        return nil, err 
                }
                dcEntry := &DCStatusDetail{DatacenterId: dcid, DatacenterNickname: dcTStatus.Metadata.DatacenterNickname, ReportInterval: dcTStatus.Metadata.Interval, DCStatusByProperty: dcTStatus.DataRows}
                dcTrafficStati.StatusByDatacenter = append(dcTrafficStati.StatusByDatacenter, dcEntry) 
        }

        return dcTrafficStati, nil

}

func gatherPropertyStatus() (*PropertyStatus, error) {

        // Use PropertyTraffic data window as period basis.
        // Maybe do Ip Avail - most recent IPs

        propStat := &PropertyStatus{PropertyName: qsProperty}
        // calc traffic period start and end
        pstart, pend := calcPeriodStartandEnd("property", statusPeriodLen)

        optArgs := make(map[string]string)

        // Retrieve IP Availability status
        optArgs["mostRecent"] = "true"
        propertyIpAvail, err := reportsgtm.GetIpStatusPerProperty(domainname, qsProperty, optArgs)
        if err != nil {
                akamai.StopSpinnerFail()
                return nil, err
        }
        // Retrieve Property Traffic status
        delete(optArgs, "mostRecent")
        optArgs["start"] = pstart
        optArgs["end"] = pend
        propertyTraffic, err := reportsgtm.GetTrafficPerProperty(domainname, qsProperty, optArgs)
        if err != nil {
                akamai.StopSpinnerFail()
                return nil, err
        }

	// Calc requests % per datacenter
        type dcReqs struct {
                reqs       int64
                totalreqs  int64
                perc       float64
        }
        var dcReqMap map[int]dcReqs
        var propertyTotalReqs int64
        dcReqMap = make(map[int]dcReqs)
        for _, tData := range propertyTraffic.DataRows {
                for _, dcData := range tData.Datacenters {
                        dcRow, ok := dcReqMap[dcData.DatacenterId]   // try to get entry
                        if !ok {
                                dcRow = dcReqs{}
                        }      
                        dcRow.reqs += dcData.Requests
                        dcReqMap[dcData.DatacenterId] = dcRow
                        propertyTotalReqs += dcData.Requests
                }
        }
        // Calculate percentage for WHOLE period
        for k, dcRow := range dcReqMap {
                dcRow.perc = (float64(dcRow.reqs)/float64(propertyTotalReqs))*100
                dcReqMap[k] = dcRow
        }

       // Build PropertyResponse struct
       propStat.Domain = propertyIpAvail.Metadata.Domain
       propStat.PropertyName = propertyIpAvail.Metadata.Property
       propStat.WindowStart = propertyTraffic.Metadata.Start      
       propStat.WindowEnd = propertyTraffic.Metadata.End 
       propStat.ReportInterval = propertyTraffic.Metadata.Interval
       propStat.DatacenterIntervalStatus = propertyTraffic.DataRows
       statusSummary := &PropertyStatusSummary{LastUpdate: propertyIpAvail.DataRows[0].Timestamp, CutOff: propertyIpAvail.DataRows[0].CutOff} 

       var propertyDCStatusArray []*PropertyDCStatus
       for _, dc := range propertyIpAvail.DataRows[0].Datacenters {
               propertyDCStatus := &PropertyDCStatus{}
               propertyDCStatus.Nickname = dc.Nickname  
               propertyDCStatus.DatacenterId = dc.DatacenterId
               propertyDCStatus.TrafficTargetName = dc.TrafficTargetName  
               propertyDCStatus.IPs = dc.IPs
               propertyDCStatus.DCPropertyUsage = fmt.Sprintf("%.2f", dcReqMap[dc.DatacenterId].perc) + "%"
               propertyDCStatus.DCTotalPeriodRequests = dcReqMap[dc.DatacenterId].reqs
               propertyDCStatusArray = append(propertyDCStatusArray, propertyDCStatus)
        }
        statusSummary.PropertyDCStatus = propertyDCStatusArray
        propStat.StatusSummary = statusSummary

        return propStat, nil

}

func cmdQueryStatus(c *cli.Context) error {

	config, err := akamai.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

        reportsgtm.Init(config)

	if c.NArg() == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain is required"), 1)
	}

	domainname = c.Args().Get(0) 

        qsProperty = c.String("property")
        qsDatacenters = (c.Generic("datacenterid")).(*arrayFlags)
        qsNicknames = c.StringSlice("dcnickname")
        verboseStatus = c.Bool("verbose")

        if (!c.IsSet("datacenterid") && !c.IsSet("dcnickname") && !c.IsSet("property")) || (c.IsSet("property") && (c.IsSet("datacenterid") || c.IsSet("dcnickname"))) {
                return cli.NewExitError(color.RedString("property OR datacenter(s) must be specified"), 1)
        }
        // if nicknames specified, add to dcFlags
        if c.IsSet("dcnickname") {
                ParseNicknames(qsNicknames, domainname)
        } 
        akamai.StartSpinner(
                "Querying status ...",
                fmt.Sprintf("Fetching status ...... [%s]", color.GreenString("OK")),
        )

        var objStatus interface{}

        if c.IsSet("datacenter") {
                fmt.Println("... Collecting DC status")
                objStatus, err = gatherDatacenterStatus()
                if err != nil {
                        akamai.StopSpinnerFail()
                        return cli.NewExitError(color.RedString("Unable to retrieve datacenter status. "+err.Error()), 1)
                }
        } else if c.IsSet("property") { 
                fmt.Println("... Collecting Property status")
                objStatus, err = gatherPropertyStatus()
                if err != nil {
                        akamai.StopSpinnerFail()
                        return cli.NewExitError(color.RedString("Unable to retrieve property status. "+err.Error()), 1)
                }
        } else {
                // Shouldn't be able to get here but ....
                akamai.StopSpinnerFail()
                return cli.NewExitError(color.RedString("Unknown object status requested"), 1)
        }

        json, err := json.MarshalIndent(objStatus, "", "  ")
        if err != nil {
                akamai.StopSpinnerFail()
                return cli.NewExitError(color.RedString("Unable to display status results"), 1)
        }
        fmt.Fprintln(c.App.Writer, string(json))

        akamai.StopSpinnerOk()

        return nil

}

