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

// Enhanced DCTData struct
type EnhancedPropertyStatus struct {
	Timestamp 	string	
	Properties 	[]TimestampPropertyStatus
}

// Timestamp DC Prop Status
type TimestampPropertyStatus struct {
	reportsgtm.DCTDRow
        Enabled bool
}

// DCStatusDetail represents individual data center traffic status.
type DCStatusDetail struct {
	DatacenterId       int
	DatacenterNickname string
	ReportInterval     string
	DCStatusByProperty []EnhancedPropertyStatus
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
	DCEnabled	      bool 
}

var defaultPeriod time.Duration = 15 * 60 * 1000 * 1000

// Calc period start and end. Input string specifying duration, e.g. 15m. Returns formatted strings consumable by GTM Reports API.
func calcPeriodStartandEnd(trafficType string, periodLen string) (string, string, error) {

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
		err := &configgtm.CommonError{}
                err.SetItem("entityName", "Window")
                err.SetItem("name", "Data Window")
                err.SetItem("apiErrorMessage", "Traffic Type "+trafficType+" not supported")
	}
	if err != nil {
		// return invalid dates
		return "", "", err
	}
	end := window.EndTime
	start := end.Add(-dur)

	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil

}

type trafficTargetEnabledStatus struct {
        ttDCID int
        ttEnabled bool
}

// Build DC Properties enabled list
func buildDCPropertiesEnabledList(domain *configgtm.Domain, dcID int) map[string]trafficTargetEnabledStatus {

	propEnabledMap := make(map[string]trafficTargetEnabledStatus)
	for _, prop := range domain.Properties {
        	for _, tgt := range prop.TrafficTargets {
                	// collect enabled status
			if tgt.DatacenterId == dcID {
                		ttMapEntry := trafficTargetEnabledStatus{ttDCID: tgt.DatacenterId, ttEnabled: tgt.Enabled}
                		propEnabledMap[prop.Name] = ttMapEntry
			}
		}
	}
	return propEnabledMap
        
}

// Build DC property List
func buildDCPropertyList(domain *configgtm.Domain, dcID int) []EnhancedPropertyStatus {

	var dcPropList []EnhancedPropertyStatus
	var dcProps []TimestampPropertyStatus
       	dcStat := EnhancedPropertyStatus{Timestamp: time.Now().Format(time.RFC3339)}
       	for _, propPtr := range domain.Properties {
                        for _, traffTarg := range propPtr.TrafficTargets {
                                if traffTarg.DatacenterId == dcID {
					dcTimedProp := TimestampPropertyStatus{}
					dcTimedProp.Name = propPtr.Name
					dcTimedProp.Requests = 0 
					dcTimedProp.Status = "0"
					dcTimedProp.Enabled = traffTarg.Enabled
                                        dcProps = append(dcProps, dcTimedProp)
                                        break
                                }
                        }
	}
	dcStat.Properties = dcProps
	dcPropList = append(dcPropList, dcStat)
	return dcPropList
}

// Retrieve a Datacenter
func findDatacenterInDomain(domain *configgtm.Domain, dcID int) (*configgtm.Datacenter, bool) {

	for _, dc := range domain.Datacenters {
		if dcID == dc.DatacenterId {
			return dc, true
		}
	}
	return nil, false
}

// Populate a list of empty DCStatusDetail structures
func populateEmptyDCStatusList() ([]*DCStatusDetail, error) {

	var dcStatDetailList []*DCStatusDetail
	dom, err := configgtm.GetDomain(domainName)
	if err != nil {
		return dcStatDetailList, err
	}
	for _, dcID := range qsDatacenters.flagList {
		dcEntry := &DCStatusDetail{DatacenterId: dcID} // Do we need the nickname also?
		if dc, ok := findDatacenterInDomain(dom, dcID); ok {
			dcEntry.DatacenterNickname = dc.Nickname
		}
		dcEntry.DCStatusByProperty = buildDCPropertyList(dom, dcID)
		dcStatDetailList = append(dcStatDetailList, dcEntry)
	}

	return dcStatDetailList, nil
}	

// Retrieve Datacenter status for domain
func gatherDatacenterStatus() (*DCTrafficStati, error) {

	dcTrafficStati := &DCTrafficStati{Domain: domainName}
	// calc period start and end
	pstart, pend, err := calcPeriodStartandEnd("datacenter", statusPeriodLen)
        if err != nil {
		return nil, err
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
		// Build DC Properties enabled list
		dom, err := configgtm.GetDomain(domainName)
                if err != nil {
			return nil, err
		}
		enabledPropertiesList := buildDCPropertiesEnabledList(dom, dcID)
		dcEntry := &DCStatusDetail{DatacenterId: dcID, DatacenterNickname: dcTStatus.Metadata.DatacenterNickname, ReportInterval: dcTStatus.Metadata.Interval}
		if (dcTStatus.DataRows == nil || len(dcTStatus.DataRows) < 1) {
			dcEntry.DCStatusByProperty = buildDCPropertyList(dom, dcID)
		} else {
			// Need to poulate dc properties status structure by timestamp
			enhPropList := make([]EnhancedPropertyStatus,0)
			for _, dctData := range dcTStatus.DataRows {
				enhPropStat := EnhancedPropertyStatus{Timestamp: dctData.Timestamp}
				propsList := make([]TimestampPropertyStatus,0)
				for _, props := range dctData.Properties {
					propRowData := TimestampPropertyStatus{}
					propRowData.Name = props.Name
					propRowData.Requests = props.Requests
					propRowData.Status = props.Status
					if dcEnb, ok := enabledPropertiesList[props.Name]; ok {
						propRowData.Enabled = dcEnb.ttEnabled
					} 
					propsList = append(propsList, propRowData)
				}
				enhPropStat.Properties = propsList
				enhPropList = append(enhPropList, enhPropStat)
				
			}
			dcEntry.DCStatusByProperty = enhPropList
		}
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
	pstart, pend, err := calcPeriodStartandEnd("property", statusPeriodLen)
	if err != nil {
		return nil, err
	}
	optArgs := make(map[string]string)
	// Retrieve IP Availability status
	optArgs["mostRecent"] = "true"
	propertyIpAvail, err := reportsgtm.GetIpStatusPerProperty(domainName, qsProperty, optArgs)
	if err != nil {
		return nil, err
	}
        var propertyTraffic *reportsgtm.PropertyTrafficResponse 
	// Retrieve Property Traffic status
	delete(optArgs, "mostRecent")
	optArgs["start"] = pstart
	optArgs["end"] = pend
	propertyTraffic, err = reportsgtm.GetTrafficPerProperty(domainName, qsProperty, optArgs)
	if err != nil {
		return nil, err
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
	// find any missing datacenters. as aside,build map of targets capturing name and enabled flag.
	var disabledDCPeriodList []*reportsgtm.PropertyDRow
	type trafficTargetEnabledStatus struct {
		ttName string
		ttNickname string
		ttEnabled bool
	}
	ttEnabledMap :=  make(map[int]trafficTargetEnabledStatus)
	prop, err := configgtm.GetProperty(qsProperty, domainName)
	// if error, can't find disabled targets ... results in incomplete set
	if err != nil {
		return nil, err
	}
	for _, tgt := range prop.TrafficTargets {
                // collect enabled status for later use
                ttMapEntry := trafficTargetEnabledStatus{ttName: tgt.Name, ttEnabled: tgt.Enabled}
                // need info from DC, e.g. nickname
                dc, err := configgtm.GetDatacenter(tgt.DatacenterId, domainName)
                if err == nil {
                        ttMapEntry.ttNickname = dc.Nickname
                }
                ttEnabledMap[tgt.DatacenterId] = ttMapEntry
		if _, ok := dcReqMap[tgt.DatacenterId]; !ok {
			// not in map so not in returned results ...
			//create and populate 
			disabledDCP := &reportsgtm.PropertyDRow{DatacenterId: tgt.DatacenterId, TrafficTargetName: tgt.Name, Requests: 0} 
			disabledDCP.Status = "0" // if wasn't reported on, likely not responding to requests
			// collect enabled status for later use
			disabledDCP.Nickname = ttMapEntry.ttNickname
			disabledDCPeriodList = append(disabledDCPeriodList, disabledDCP)
			// add an entry to dcReqMap
			dcReqMap[tgt.DatacenterId] = dcReqs{reqs:0, perc:0.0}
		} 
	}
	// append to datarow dc lists
	for _, drEntry := range propertyTraffic.DataRows {
		drEntry.Datacenters = append(drEntry.Datacenters, disabledDCPeriodList...)
	} 	
	// Calculate percentage for WHOLE period
	for k, dcRow := range dcReqMap {
		if propertyTotalReqs > 0 {
			dcRow.perc = (float64(dcRow.reqs) / float64(propertyTotalReqs)) * 100
		} else {
			dcRow.perc = float64(0)
		}
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
		for _, dr := range propertyIpAvail.DataRows {
			for _, dc := range dr.Datacenters {
				propertyDCStatus := &PropertyDCStatus{}
				propertyDCStatus.Nickname = dc.Nickname
				propertyDCStatus.DatacenterId = dc.DatacenterId
				propertyDCStatus.TrafficTargetName = dc.TrafficTargetName
				propertyDCStatus.IPs = dc.IPs
				propertyDCStatus.DCPropertyUsage = fmt.Sprintf("%.2f", dcReqMap[dc.DatacenterId].perc) + "%"
				propertyDCStatus.DCTotalPeriodRequests = dcReqMap[dc.DatacenterId].reqs
				propertyDCStatus.DCEnabled = ttEnabledMap[dc.DatacenterId].ttEnabled
				propertyDCStatusArray = append(propertyDCStatusArray, propertyDCStatus)
				// remove entry from map ...
				delete(ttEnabledMap, dc.DatacenterId)	
			}
		}
	}
	// Add in disabled DCs
	var disabledDCSumList []*PropertyDCStatus
	for dcId, eMap := range ttEnabledMap {
                disabledDCSum := &PropertyDCStatus{}
                disabledDCSum.DatacenterId = dcId
                disabledDCSum.TrafficTargetName = eMap.ttName
		disabledDCSum.DCEnabled = eMap.ttEnabled 
                disabledDCSum.Nickname = eMap.ttNickname
                disabledDCSum.DCPropertyUsage = "0.00%"
		disabledDCSum.IPs = make([]*reportsgtm.IpStatIp, 0)
		disabledDCSumList = append(disabledDCSumList, disabledDCSum)
	}
	propertyDCStatusArray = append(propertyDCStatusArray, disabledDCSumList...)
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
	err = ParseNicknames(qsDatacenters.nicknamesList, domainName)
        if err != nil {
                if verboseStatus {
                        return cli.NewExitError(color.RedString("Unable to retrieve datacenter list. "+err.Error()), 1)
                } else {
                        return cli.NewExitError(color.RedString("Unable to retrieve datacenter."), 1)
                }
        }
	if c.IsSet("json") {
		akamai.StartSpinner("", "")
		} else {
		akamai.StartSpinner(
			"Querying status ...",
			fmt.Sprintf("Fetching status ...... [%s]", color.GreenString("OK")),
		)
	}

	var objStatus interface{}

	if c.IsSet("datacenter") {
		if !c.IsSet("json") { fmt.Println("... Collecting DC status") }
		objStatus, err = gatherDatacenterStatus()
	} else if c.IsSet("property") {
		if !c.IsSet("json") { fmt.Println("... Collecting Property status") }
		objStatus, err = gatherPropertyStatus()
	} else {
                if !c.IsSet("json") { fmt.Println("... Collecting Domain status") }
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

		akamai.StopSpinner("", true)

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

	table.SetHeader([]string{"Datacenter", "Nickname", "Timestamp", "Property", "Enabled", "Requests", "Status"})
	table.SetReflowDuringAutoWrap(false)
	table.SetCenterSeparator(" ")
	table.SetColumnSeparator(" ")
	table.SetRowSeparator(" ")
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid := " "
	dcln := " "
	dcptl := " "
	if len(objStatus.StatusByDatacenter) == 0 {
		rowData := []string{"No datacenter status available", " ", " ", " ", " ", " ", " "}
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
						dcptl, prop.Name, strconv.FormatBool(prop.Enabled), strconv.FormatInt(prop.Requests, 10), prop.Status}
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
	table.SetHeader([]string{"Datacenter", "Nickname", "Target Name", "Enabled", "Total Requests", "Property Usage", "IP", "State"})
	table.SetReflowDuringAutoWrap(false)
	table.SetCenterSeparator(" ")
	table.SetColumnSeparator(" ")
	table.SetRowSeparator(" ")
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_CENTER, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER, tablewriter.ALIGN_CENTER})
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	dclid := " "
	dcln := " "
	dctn := " "
	dcptl := " "
	dcptr := " "
	dcpperc := " "
	dcip := " "
	dcenabled := " "
	if len(objStatus.StatusSummary.PropertyDCStatus) == 0 {
		rowData := []string{"No status summary data available", " ", " ", " ", " ", " ", " ", " ", " "}
		table.Append(rowData)
	} else {
		for _, dc := range objStatus.StatusSummary.PropertyDCStatus {
			dcln = dc.Nickname
			dclid = strconv.Itoa(dc.DatacenterId)
			dctn = dc.TrafficTargetName
			dcpperc = dc.DCPropertyUsage
			dcenabled = strconv.FormatBool(dc.DCEnabled)
			dcptr = strconv.FormatInt(dc.DCTotalPeriodRequests, 10)
			if len(dc.IPs) < 1 {
				dc.IPs = []*reportsgtm.IpStatIp{&reportsgtm.IpStatIp{}} 
			}
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
					dcenabled = " "
				}
				rowData := []string{dclid, dcln, dctn, dcenabled, dcptr, dcpperc, dcip, fmt.Sprintf("HandedOut: %s", strconv.FormatBool(ip.HandedOut))}
				table.Append(rowData)
				rowData = []string{"", "", "", "", "", "", " ", fmt.Sprintf("Score: %s", fmt.Sprintf("%.2f", ip.Score))}
				table.Append(rowData)
				rowData = []string{"", "", "", "", "", "", " ", fmt.Sprintf("Alive: %s", strconv.FormatBool(ip.Alive))}
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

