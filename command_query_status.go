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
        "github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_3"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

var domainName string
var qsProperty string
var qsDatacenters *arrayFlags
var statusPeriodLen = "15m"
var qsNicknames []string

// DCTrafficStati  represents Data Center Traffic Status returned structure. Contains a list of individual DC stati.
type DCTrafficStati struct {
	Domain             string
	PeriodStart        string
	PeriodEnd          string
	StatusByDatacenter []*DCStatusDetail
}

// DCStatusDetail represents individual data center traffic status.
type DCStatusDetail struct {
	DatacenterId       int
	DatacenterNickname string
	ReportInterval     string
	DCStatusByProperty []*reportsgtm.DCTData
}

// PropertyStatus represents returned Property Status structure.
type PropertyStatus struct {
	Domain                   string
	PropertyName             string
	PeriodStart              string
	PeriodEnd                string
	ReportInterval           string
	StatusSummary            *PropertyStatusSummary
	DatacenterIntervalStatus []*reportsgtm.PropertyTData
}

// PropertyStatusSummary represents Property IP Status Summary struct
type PropertyStatusSummary struct {
	LastUpdate       string
	CutOff           float64
	PropertyDCStatus []*PropertyDCStatus
}

// PropertyDCStatus represents Property DC Status Summary struct
type PropertyDCStatus struct {
	reportsgtm.IpStatPerPropDRow
	DCTotalPeriodRequests int64
	DCPropertyUsage       string
}

var defaultPeriod time.Duration = 15 * 60 * 1000 * 1000
var isoStart string = "2019-08-01T00:00:00Z"
var isoEnd string = "2019-08-07T00:00:00Z"

// Calc period start and end. Input string specifying duration, e.g. 15m. Returns formatted strings consumable by GTM Reports API.
func calcPeriodStartandEnd(trafficType string, periodLen string) (string, string) {

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

// Retrieve Datacenter status for domain
func gatherDatacenterStatus() (*DCTrafficStati, error) {

	dcTrafficStati := &DCTrafficStati{Domain: domainName}
	// calc period start and end
	pstart, pend := calcPeriodStartandEnd("datacenter", statusPeriodLen)
        if pstart == isoStart || pend == isoEnd {
        	// default dates. return MT struct
		return dcTrafficStati, nil
        }
	dcTrafficStati.PeriodStart = pstart
	dcTrafficStati.PeriodEnd = pend
	optArgs := make(map[string]string)
	optArgs["start"] = pstart
	optArgs["end"] = pend

	// Looping on DCs
	for _, dcID := range qsDatacenters.flagList {
		dcTStatus, err := reportsgtm.GetTrafficPerDatacenter(domainName, dcID, optArgs)
		if err != nil {
			return nil, err
		}
		dcEntry := &DCStatusDetail{DatacenterId: dcID, DatacenterNickname: dcTStatus.Metadata.DatacenterNickname, ReportInterval: dcTStatus.Metadata.Interval, DCStatusByProperty: dcTStatus.DataRows}
		dcTrafficStati.StatusByDatacenter = append(dcTrafficStati.StatusByDatacenter, dcEntry)
	}

	return dcTrafficStati, nil

}

// Retrieve Domain status
func getDomainStatus() (*configgtm.ResponseStatus, error) {

	domStatus, err := configgtm.GetDomainStatus(domainName)
        if err != nil {
                return nil, err
        }

	return domStatus, nil

}

// Retrieve Status and DC status for property
func gatherPropertyStatus() (*PropertyStatus, error) {

	propStat := &PropertyStatus{PropertyName: qsProperty}
	// calc traffic period start and end
	pstart, pend := calcPeriodStartandEnd("property", statusPeriodLen)

	optArgs := make(map[string]string)

	// Retrieve IP Availability status
	optArgs["mostRecent"] = "true"
	propertyIpAvail, err := reportsgtm.GetIpStatusPerProperty(domainName, qsProperty, optArgs)
	if err != nil {
		akamai.StopSpinnerFail()
		return nil, err
	}
        var propertyTraffic *reportsgtm.PropertyTrafficResponse 
        if pstart == isoStart || pend == isoEnd {
                // default dates. create MT struct
                propertyTraffic = &reportsgtm.PropertyTrafficResponse{}
        } else {
		// Retrieve Property Traffic status
		delete(optArgs, "mostRecent")
		optArgs["start"] = pstart
		optArgs["end"] = pend
		propertyTraffic, err = reportsgtm.GetTrafficPerProperty(domainName, qsProperty, optArgs)
		if err != nil {
			akamai.StopSpinnerFail()
			return nil, err
		}
        }
	// Calc requests % per datacenter
	type dcReqs struct {
		reqs      int64
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
        if pstart == isoStart || pend == isoEnd {
        	propStat.PeriodStart = "Not Available"
        	propStat.PeriodEnd = "Not Available"
        	propStat.ReportInterval = " "
        } else {
		propStat.PeriodStart = propertyTraffic.Metadata.Start
		propStat.PeriodEnd = propertyTraffic.Metadata.End
		propStat.ReportInterval = propertyTraffic.Metadata.Interval
	}
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

// worker function for query-status
func cmdQueryStatus(c *cli.Context) error {

	config, err := akamai.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

	reportsgtm.Init(config)
        configgtm.Init(config)

	if c.NArg() == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain is required"), 1)
	}

	domainName = c.Args().Get(0)

	qsProperty = c.String("property")
	qsDatacenters = (c.Generic("datacenter")).(*arrayFlags)
        if c.IsSet("verbose") {
		verboseStatus = true
	}

	if c.IsSet("property") && c.IsSet("datacenter") {
		return cli.NewExitError(color.RedString("property OR datacenter(s) must be specified"), 1)
	}
	ParseNicknames(qsDatacenters.nicknamesList, domainName)
	akamai.StartSpinner(
		"Querying status ...",
		fmt.Sprintf("Fetching status ...... [%s]", color.GreenString("OK")),
	)

	var objStatus interface{}

	if c.IsSet("datacenter") {
		fmt.Println("... Collecting DC status")
		objStatus, err = gatherDatacenterStatus()
	} else if c.IsSet("property") {
		fmt.Println("... Collecting Property status")
		objStatus, err = gatherPropertyStatus()
	} else {
                fmt.Println("... Collecting Domain status")
		objStatus, err = getDomainStatus()
	}
        // check for failure
        if err != nil {
                akamai.StopSpinnerFail()
                if verboseStatus {
                        return cli.NewExitError(color.RedString("Unable to retrieve status. "+err.Error()), 1)
                } else {
                        return cli.NewExitError(color.RedString("Unable to retrieve status."), 1)
                }       
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
		if c.IsSet("datacenter") {

			fmt.Fprintln(c.App.Writer, renderDatacenterTable(objStatus.(*DCTrafficStati), c))

		} else if c.IsSet("property") {

			fmt.Fprintln(c.App.Writer, renderPropertyTable(objStatus.(*PropertyStatus), c))

		} else {
			fmt.Fprintln(c.App.Writer, renderDomainTable(objStatus.(*configgtm.ResponseStatus), c))
                }
	}

	return nil

}

// Generate pretty print DC status
func renderDatacenterTable(objStatus *DCTrafficStati, c *cli.Context) string {

	var outString string
	outString += fmt.Sprintln("Domain: ", objStatus.Domain)
	outString += fmt.Sprintln("Period Start: ", objStatus.PeriodStart)
	outString += fmt.Sprintln("Period End: ", objStatus.PeriodEnd)
	outString += fmt.Sprintln(" ")
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
		rowData := []string{"No datacenter status available", " ", " ", " ", " ", " "}
		table.Append(rowData)
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
					rowData := []string{dclid, dcln,
						dcptl, prop.Name, strconv.FormatInt(prop.Requests, 10), prop.Status}
					table.Append(rowData)
				}
			}
		}
	}

	table.Render()

	outString += fmt.Sprintln(tableString.String())
	return outString

}

func renderPropertyTable(objStatus *PropertyStatus, c *cli.Context) string {

	var outString string
	outString += fmt.Sprintln("Domain: ", objStatus.Domain)
	outString += fmt.Sprintln("Property: ", objStatus.PropertyName)
	outString += fmt.Sprintln("Period Start: ", objStatus.PeriodStart)
	outString += fmt.Sprintln("Period End: ", objStatus.PeriodEnd)
	outString += fmt.Sprintln(" ")

	// Build Summary Table
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)
	outString += fmt.Sprintln("Status Summary -- Last Update: ", objStatus.StatusSummary.LastUpdate, ", CutOff: ", objStatus.StatusSummary.CutOff)
	outString += fmt.Sprintln(" ")
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
		rowData := []string{"No status summary data available", " ", " ", " ", " ", " ", " ", " "}
		table.Append(rowData)
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
				rowData := []string{dclid, dcln, dctn, dcptr, dcpperc, dcip, fmt.Sprintf("HandedOut: %s", strconv.FormatBool(ip.HandedOut))}
				table.Append(rowData)
				rowData = []string{"", "", "", "", "", "", fmt.Sprintf("Score: %s", fmt.Sprintf("%.2f", ip.Score))}
				table.Append(rowData)
				rowData = []string{"", "", "", "", "", "", fmt.Sprintf("Alive: %s", strconv.FormatBool(ip.Alive))}
				table.Append(rowData)
			}
		}
	}
	table.Render()
	outString += fmt.Sprintln(tableString.String())

	// Build Datacenter Status table
	outString += fmt.Sprintln(" ")
	outString += fmt.Sprintln("Datacenter Status")
	outString += fmt.Sprintln(" ")
	tableString = &strings.Builder{}
	dcTable := tablewriter.NewWriter(tableString)

	dcTable.SetHeader([]string{"Timestamp", "Datacenter", "Nickname", "Requests", "Status"})
	dcTable.SetReflowDuringAutoWrap(false)
	dcTable.SetCenterSeparator(" ")
	dcTable.SetColumnSeparator(" ")
	dcTable.SetRowSeparator(" ")
	dcTable.SetBorder(false)
	dcTable.SetAutoWrapText(false)
	dcTable.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	dcTable.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid = " "
	dcln = " "
	dcptl = " "
	if len(objStatus.DatacenterIntervalStatus) == 0 {
		rowData := []string{"No datacenter interval status available", " ", " ", " ", " "}
		dcTable.Append(rowData)
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
				rowData := []string{dcptl, dclid, dcln, strconv.FormatInt(dc.Requests, 10), dc.Status}
				dcTable.Append(rowData)
			}
		}
	}
	dcTable.Render()

	outString += fmt.Sprintln(tableString.String())
	return outString

}

// Pretty print output
func renderDomainTable(status *configgtm.ResponseStatus, c *cli.Context) string {

        var outString string
        outString += fmt.Sprintln(" ")
        outString += fmt.Sprintln(fmt.Sprintf("Domain: %s", domainName))
        outString += fmt.Sprintln("Current Status")
        outString += fmt.Sprintln(" ")
        tableString := &strings.Builder{}
        table := tablewriter.NewWriter(tableString)

        table.SetReflowDuringAutoWrap(false)
        table.SetCenterSeparator(" ")
        table.SetColumnSeparator(" ")
        table.SetRowSeparator(" ")
        table.SetBorder(false)
        table.SetAutoWrapText(false)
        table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
        table.SetAlignment(tablewriter.ALIGN_CENTER)

        // Build status table. Exclude Links.
        rowData := []string{"ChangeId", status.ChangeId}
        table.Append(rowData)
        rowData = []string{"Message", status.Message}
        table.Append(rowData)
        rowData = []string{"Passing Validation", strconv.FormatBool(status.PassingValidation)}
        table.Append(rowData)
        rowData = []string{"Propagation Status", status.PropagationStatus}
        table.Append(rowData)
        rowData = []string{"Propagation Status Date", status.PropagationStatusDate}
        table.Append(rowData)

        table.Render()
        outString += fmt.Sprintln(tableString.String())

        return outString

}

