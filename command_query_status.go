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
	"strconv"
	"strings"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/reportsgtm-v1"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

var domainname string
var qsProperty string
var qsDatacenters *arrayFlags
var statusPeriodLen = "15m"
var qsNicknames []string

// Data Center Traffic Status returned structure. Contains a list of individual DC stati.
type DCTrafficStati struct {
	Domain             string
	PeriodStart        string
	PeriodEnd          string
	StatusByDatacenter []*DCStatusDetail
}

// Individual data center traffic status.
type DCStatusDetail struct {
	DatacenterId       int
	DatacenterNickname string
	ReportInterval     string
	DCStatusByProperty []*reportsgtm.DCTData
}

// Property Status returned structure. Contains a ...
type PropertyStatus struct {
	Domain                   string
	PropertyName             string
	PeriodStart              string
	PeriodEnd                string
	ReportInterval           string
	StatusSummary            *PropertyStatusSummary
	DatacenterIntervalStatus []*reportsgtm.PropertyTData
}

// Property IP Status Summary struct
type PropertyStatusSummary struct {
	LastUpdate       string
	CutOff           float64
	PropertyDCStatus []*PropertyDCStatus
}

// Property DC Status Summary struct
type PropertyDCStatus struct {
	reportsgtm.IpStatPerPropDRow
	DCTotalPeriodRequests int64
	DCPropertyUsage       string
}

var defaultPeriod time.Duration = 15 * 60 * 1000 * 1000

// Calc period start and end. Input string specifying duration, e.g. 15m. Returns formatted strings consumable by GTM Reports API.
func calcPeriodStartandEnd(trafficType string, periodLen string) (string, string) {

	isoStart := "2019-08-01T00:00:00Z"
	isoEnd := "2019-08-07T00:00:00Z"

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
		reqs      int64
		totalreqs int64
		perc      float64
	}
	var dcReqMap map[int]dcReqs
	var propertyTotalReqs int64
	dcReqMap = make(map[int]dcReqs)
	for _, tData := range propertyTraffic.DataRows {
		for _, dcData := range tData.Datacenters {
			dcRow, ok := dcReqMap[dcData.DatacenterId] // try to get entry
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
		dcRow.perc = (float64(dcRow.reqs) / float64(propertyTotalReqs)) * 100
		dcReqMap[k] = dcRow
	}

	// Build PropertyResponse struct
	propStat.Domain = propertyIpAvail.Metadata.Domain
	propStat.PropertyName = propertyIpAvail.Metadata.Property
	propStat.PeriodStart = propertyTraffic.Metadata.Start
	propStat.PeriodEnd = propertyTraffic.Metadata.End
	propStat.ReportInterval = propertyTraffic.Metadata.Interval
	propStat.DatacenterIntervalStatus = propertyTraffic.DataRows

	var statusSummary *PropertyStatusSummary
        statusSummary = &PropertyStatusSummary{}
	if len(propertyIpAvail.DataRows) > 0 {
		statusSummary.LastUpdate = propertyIpAvail.DataRows[0].Timestamp
		statusSummary.CutOff = propertyIpAvail.DataRows[0].CutOff
	} else {
               statusSummary.LastUpdate = "Not Available"
	}
	var propertyDCStatusArray []*PropertyDCStatus
	if len(propertyIpAvail.DataRows) > 0 {
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

	if c.IsSet("datacenterid") {
		fmt.Println("... Collecting DC status")
		objStatus, err = gatherDatacenterStatus()
		if err != nil {
			akamai.StopSpinnerFail()
			if verboseStatus {
				return cli.NewExitError(color.RedString("Unable to retrieve datacenter status. "+err.Error()), 1)
			} else {
				return cli.NewExitError(color.RedString("Unable to retrieve datacenter status."), 1)
			}
		}
	} else if c.IsSet("property") {
		fmt.Println("... Collecting Property status")
		objStatus, err = gatherPropertyStatus()
		if err != nil {
			akamai.StopSpinnerFail()
			if verboseStatus {
				return cli.NewExitError(color.RedString("Unable to retrieve property status. "+err.Error()), 1)
			} else {
				return cli.NewExitError(color.RedString("Unable to retrieve property status."), 1)
			}
		}
	} else {
		// Shouldn't be able to get here but ....
		akamai.StopSpinnerFail()
		return cli.NewExitError(color.RedString("Unknown object status requested"), 1)
	}

	if c.IsSet("json") && c.Bool("json") {
		json, err := json.MarshalIndent(objStatus, "", "  ")
		if err != nil {
			akamai.StopSpinnerFail()
			return cli.NewExitError(color.RedString("Unable to display status results"), 1)
		}
		fmt.Fprintln(c.App.Writer, string(json))

		akamai.StopSpinnerOk()

	} else {
		akamai.StopSpinnerOk()

		fmt.Fprintln(c.App.Writer, "")
		if c.IsSet("datacenterid") {

			fmt.Fprintln(c.App.Writer, renderDatacenterTable(objStatus.(*DCTrafficStati), c))

		} else if c.IsSet("property") {

			fmt.Fprintln(c.App.Writer, renderPropertyTable(objStatus.(*PropertyStatus), c))

		}
	}

	return nil

}

func renderDatacenterTable(objStatus *DCTrafficStati, c *cli.Context) string {

	var outstring string
	outstring += fmt.Sprintln("Domain: ", objStatus.Domain)
	outstring += fmt.Sprintln("Period Start: ", objStatus.PeriodStart)
	outstring += fmt.Sprintln("Period End: ", objStatus.PeriodEnd)
	outstring += fmt.Sprintln(" ")
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)

	table.SetHeader([]string{"Datacenter", "Nickname", "Timestamp", "Property", "Requests", "Status"})
	table.SetReflowDuringAutoWrap(false)
	table.SetCenterSeparator(" ")
	table.SetColumnSeparator(" ")
	table.SetRowSeparator(" ")
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid := " "
	dcln := " "
	dcptl := " "
        if len(objStatus.StatusByDatacenter) == 0 {
		rowdata := []string{"No datacenter status available", " ", " ", " ", " ", " "}
		table.Append(rowdata)
	} else {
		for _, dc := range objStatus.StatusByDatacenter {
			for pk, dcprop := range dc.DCStatusByProperty {
				for k, prop := range dcprop.Properties {
					if k == 0 {
						dcptl = dcprop.Timestamp
						if pk == 0 {
							dclid = strconv.Itoa(dc.DatacenterId)
							dcln = dc.DatacenterNickname
						} else {
							dclid = " "
							dcln = " "
						}
					} else {
						dcptl = " "
						dclid = " "
						dcln = " "
					}
					rowdata := []string{dclid, dcln,
						dcptl, prop.Name, strconv.FormatInt(prop.Requests, 10), prop.Status}
					table.Append(rowdata)
				}
			}
		}
	}

	table.Render()

	outstring += fmt.Sprintln(tableString.String())
	return outstring

}

func renderPropertyTable(objStatus *PropertyStatus, c *cli.Context) string {

	var outstring string
	outstring += fmt.Sprintln("Domain: ", objStatus.Domain)
	outstring += fmt.Sprintln("Property: ", objStatus.PropertyName)
	outstring += fmt.Sprintln("Period Start: ", objStatus.PeriodStart)
	outstring += fmt.Sprintln("Period End: ", objStatus.PeriodEnd)
	outstring += fmt.Sprintln(" ")

	// Build Summary Table
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	outstring += fmt.Sprintln("Status Summary -- Last Update: ", objStatus.StatusSummary.LastUpdate, ", CutOff: ", objStatus.StatusSummary.CutOff)
	outstring += fmt.Sprintln(" ")
	table.SetHeader([]string{"Datacenter", "Nickname", "Target Name", "Total Requests", "Property Usage", "IP", "State"})
	table.SetReflowDuringAutoWrap(false)
	table.SetCenterSeparator(" ")
	table.SetColumnSeparator(" ")
	table.SetRowSeparator(" ")
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid := " "
	dcln := " "
	dctn := " "
	dcptl := " "
	dcptr := " "
	dcpperc := " "
        dcip := " "
	if len(objStatus.StatusSummary.PropertyDCStatus) == 0 {
		rowdata := []string{"No status summary data available", " ", " ", " ", " ", " ", " ", " "}
		table.Append(rowdata)
	} else {
		for _, dc := range objStatus.StatusSummary.PropertyDCStatus {
			dcln = dc.Nickname
			dclid = strconv.Itoa(dc.DatacenterId)
			dctn = dc.TrafficTargetName
			dcpperc = dc.DCPropertyUsage
			dcptr = strconv.FormatInt(dc.DCTotalPeriodRequests, 10)
			for k, ip := range dc.IPs {
				if k == 0 {
					dcip = ip.Ip
				} else {
					dctn = " "
					dclid = " "
					dcln = " "
					dcip = " "
					dcpperc = " "
					dcptr = " "
				}
                                rowdata := []string{dclid, dcln, dctn, dcptr, dcpperc, dcip, fmt.Sprintf("HandedOut: %s", strconv.FormatBool(ip.HandedOut))}
                                table.Append(rowdata)
                                rowdata = []string{"", "", "", "", "", "", fmt.Sprintf("Score: %s", fmt.Sprintf("%.2f", ip.Score))}
                                table.Append(rowdata)
                                rowdata = []string{"", "", "", "", "", "", fmt.Sprintf("Alive: %s", strconv.FormatBool(ip.Alive))}
				table.Append(rowdata)
			}
		}
	}
	table.Render()
	outstring += fmt.Sprintln(tableString.String())

	// Build Datacenter Status table
	outstring += fmt.Sprintln(" ")
        outstring += fmt.Sprintln("Datacenter Status")
	outstring += fmt.Sprintln(" ")
	tableString = &strings.Builder{}
	dctable := tablewriter.NewWriter(tableString)

	dctable.SetHeader([]string{"Timestamp", "Datacenter", "Nickname", "Requests", "Status"})
	dctable.SetReflowDuringAutoWrap(false)
	dctable.SetCenterSeparator(" ")
	dctable.SetColumnSeparator(" ")
	dctable.SetRowSeparator(" ")
	dctable.SetBorder(false)
	dctable.SetAutoWrapText(false)
	dctable.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	dctable.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid = " "
	dcln = " "
	dcptl = " "
	if len(objStatus.DatacenterIntervalStatus) == 0 {
		rowdata := []string{"No datacenter interval status available", " ", " ", " ", " "}
                dctable.Append(rowdata)
	} else {
		for _, dcis := range objStatus.DatacenterIntervalStatus {
			dcptl = dcis.Timestamp
			for k, dc := range dcis.Datacenters {
				if k == 0 {
					dcptl = dcis.Timestamp
				} else {
					dcptl = " "
				}
                        	dcln = dc.Nickname
	                	dclid = strconv.Itoa(dc.DatacenterId)
				rowdata := []string{dcptl, dclid, dcln, strconv.FormatInt(dc.Requests, 10), dc.Status}
				dctable.Append(rowdata)
			}
		}
	}
	dctable.Render()

	outstring += fmt.Sprintln(tableString.String())
	return outstring

}
