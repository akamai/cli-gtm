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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cli-gtm/edgegrid"
	"cli-gtm/reportsgtm"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg/gtm"
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg/session"
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
	Timestamp  string
	Properties []TimestampPropertyStatus
}

// Timestamp DC Prop Status
type TimestampPropertyStatus struct {
	reportsgtm.DCTDRow
	Enabled bool
	Weight  float64
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
	DCEnabled             bool
	DCWeight              float64
}

var defaultPeriod time.Duration = 15 * 60 * 1000 * 1000

type SessionAdapter struct {
	s session.Session
}

func (a *SessionAdapter) Exec(req *http.Request, v interface{}) (*http.Response, error) {
	// This matches edgegrid.Session's Exec method signature
	return a.s.Exec(req, v)
}

// Calc period start and end. Input string specifying duration, e.g. 15m. Returns formatted strings consumable by GTM Reports API.
func calcPeriodStartandEnd(c *cli.Context, trafficType, periodLen string) (string, string, error) {
	dur, err := time.ParseDuration(periodLen)
	if err != nil {
		dur = defaultPeriod
	}

	ctx := context.Background()

	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return "", "", err
	}
	adaptedSess := &SessionAdapter{s: sess}

	var window *reportsgtm.WindowResponse

	switch trafficType {
	case "datacenter":
		window, err = reportsgtm.GetDatacentersTrafficWindow(ctx, adaptedSess)
	case "property":
		window, err = reportsgtm.GetPropertiesTrafficWindow(ctx, adaptedSess)
	default:
		return "", "", fmt.Errorf("trafficType %q not supported", trafficType)
	}

	if err != nil {
		return "", "", err
	}

	end := window.EndTime
	start := end.Add(-dur)

	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

type trafficTargetEnabledStatus struct {
	ttDCID    int
	ttEnabled bool
	ttWeight  float64
}

// Build DC Properties enabled Map
func buildDCPropertiesEnabledMap(domain *gtm.Domain, dcID int) map[string]*trafficTargetEnabledStatus {

	propEnabledMap := make(map[string]*trafficTargetEnabledStatus)
	for _, prop := range domain.Properties {
		for _, tgt := range prop.TrafficTargets {
			// collect enabled status
			if tgt.DatacenterID == dcID {
				ttMapEntry := &trafficTargetEnabledStatus{ttDCID: tgt.DatacenterID, ttEnabled: tgt.Enabled,
					ttWeight: tgt.Weight}
				propEnabledMap[prop.Name] = ttMapEntry
			}
		}
	}
	return propEnabledMap

}

// Build DC property List
func buildDCPropertyList(domain *gtm.Domain, dcID int) []EnhancedPropertyStatus {

	var dcPropList []EnhancedPropertyStatus
	var dcProps []TimestampPropertyStatus
	dcStat := EnhancedPropertyStatus{Timestamp: time.Now().Format(time.RFC3339)}
	for _, propPtr := range domain.Properties {
		for _, traffTarg := range propPtr.TrafficTargets {
			if traffTarg.DatacenterID == dcID {
				dcTimedProp := TimestampPropertyStatus{}
				dcTimedProp.Name = propPtr.Name
				dcTimedProp.Requests = 0
				dcTimedProp.Status = "0"
				dcTimedProp.Enabled = traffTarg.Enabled
				dcTimedProp.Weight = traffTarg.Weight
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
func findDatacenterInDomain(domain *gtm.Domain, dcID int) (*gtm.Datacenter, bool) {
	for i := range domain.Datacenters {
		if dcID == domain.Datacenters[i].DatacenterID {
			return &domain.Datacenters[i], true
		}
	}
	return nil, false
}

// Populate a list of empty DCStatusDetail structures
func populateEmptyDCStatusList(c *cli.Context) ([]*DCStatusDetail, error) {
	ctx := context.Background()

	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("session failed %v", err)
	}

	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(edgegrid.GetSession(ctx))

	// Fetch domain object using gtmClient
	domainResp, err := gtmClient.GetDomain(ctx, gtm.GetDomainRequest{DomainName: domainName})
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}

	// Extract the actual Domain from response
	dom := (*gtm.Domain)(domainResp)

	var dcStatDetailList []*DCStatusDetail

	for _, dcID := range qsDatacenters.flagList {
		dcEntry := &DCStatusDetail{DatacenterId: dcID}

		if dc, ok := findDatacenterInDomain(dom, dcID); ok {
			dcEntry.DatacenterNickname = dc.Nickname
		}

		dcEntry.DCStatusByProperty = buildDCPropertyList(dom, dcID)
		dcStatDetailList = append(dcStatDetailList, dcEntry)
	}

	return dcStatDetailList, nil
}

// Retrieve Datacenter status for domain
func gatherDatacenterStatus(c *cli.Context) (*DCTrafficStati, error) {
	dcTrafficStati := &DCTrafficStati{Domain: domainName}

	// Initialize context and session
	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}
	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(sess)

	// calc period start and end
	pstart, pend, err := calcPeriodStartandEnd(c, "datacenter", statusPeriodLen)
	if err != nil {
		return nil, err
	}
	dcTrafficStati.PeriodStart = pstart
	dcTrafficStati.PeriodEnd = pend

	// get the domain struct via gtmClient
	domainResp, err := gtmClient.GetDomain(ctx, gtm.GetDomainRequest{DomainName: domainName})
	if err != nil {
		return nil, fmt.Errorf("failed to get domain: %w", err)
	}
	dom := (*gtm.Domain)(domainResp)

	optArgs := map[string]string{
		"start": pstart,
		"end":   pend,
	}

	// Looping on DCs
	for _, dcID := range qsDatacenters.flagList {
		// Using your reportsgtm package, adapted for v11 with context and session
		dcTStatus, err := reportsgtm.GetTrafficPerDatacenter(c, domainName, dcID, optArgs)
		if err != nil {
			return nil, err
		}

		// Build DC Properties enabled list
		enabledPropertiesMap := buildDCPropertiesEnabledMap(dom, dcID)
		dcEntry := &DCStatusDetail{
			DatacenterId:       dcID,
			DatacenterNickname: dcTStatus.Metadata.DatacenterNickname,
			ReportInterval:     dcTStatus.Metadata.Interval,
		}

		if len(dcTStatus.DataRows) == 0 {
			dcEntry.DCStatusByProperty = buildDCPropertyList(dom, dcID)
		} else {
			enhPropList := make([]EnhancedPropertyStatus, 0)
			for _, dctData := range dcTStatus.DataRows {
				enabledPropMapCopy := make(map[string]*trafficTargetEnabledStatus)
				for k, v := range enabledPropertiesMap {
					enabledPropMapCopy[k] = v
				}

				enhPropStat := EnhancedPropertyStatus{Timestamp: dctData.Timestamp}
				propsList := make([]TimestampPropertyStatus, 0)

				for _, props := range dctData.Properties {
					propRowData := TimestampPropertyStatus{
						DCTDRow: reportsgtm.DCTDRow{
							Name:     props.Name,
							Requests: props.Requests,
							Status:   props.Status,
						},
					}
					if dcEnb, ok := enabledPropMapCopy[props.Name]; ok {
						propRowData.Enabled = dcEnb.ttEnabled
						propRowData.Weight = dcEnb.ttWeight
					}
					delete(enabledPropMapCopy, props.Name)
					propsList = append(propsList, propRowData)
				}

				// Add missing properties
				for propName, eInfo := range enabledPropMapCopy {
					propRowData := TimestampPropertyStatus{
						DCTDRow: reportsgtm.DCTDRow{
							Name:     propName,
							Requests: 0,
							Status:   "0",
						},
						Enabled: eInfo.ttEnabled,
						Weight:  eInfo.ttWeight,
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
func getDomainStatus(c *cli.Context) (*gtm.GetDomainStatusResponse, error) {

	ctx := context.Background()

	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(edgegrid.GetSession(ctx))

	statusResp, err := gtmClient.GetDomainStatus(ctx, gtm.GetDomainStatusRequest{
		DomainName: domainName,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve domain status: %w", err)
	}

	return statusResp, nil
}

func uniqueIPs(ips []*reportsgtm.IpStatIp) []*reportsgtm.IpStatIp {
	seen := map[string]bool{}
	result := []*reportsgtm.IpStatIp{}
	for _, ip := range ips {
		if ip != nil && ip.Ip != "" && !seen[ip.Ip] {
			result = append(result, ip)
			seen[ip.Ip] = true
		}
	}
	return result
}

func gatherPropertyStatus(c *cli.Context) (*PropertyStatus, error) {
	propStat := &PropertyStatus{PropertyName: qsProperty}

	// Calculate period
	pstart, pend, err := calcPeriodStartandEnd(c, "property", statusPeriodLen)
	if err != nil {
		return nil, err
	}

	// Initialize session and clients
	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("session failed %v", err)
	}
	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(edgegrid.GetSession(ctx))

	// Get IP status
	optArgs := map[string]string{"mostRecent": "true"}
	propertyIpAvail, err := reportsgtm.GetIpStatusPerProperty(c, domainName, qsProperty, optArgs)
	if err != nil {
		return nil, err
	}

	// Get traffic
	delete(optArgs, "mostRecent")
	optArgs["start"] = pstart
	optArgs["end"] = pend
	propertyTraffic, err := reportsgtm.GetTrafficPerProperty(c, domainName, qsProperty, optArgs)
	if err != nil {
		return nil, err
	}

	// Aggregate requests
	type dcReqs struct {
		reqs int64
		perc float64
	}
	dcReqMap := make(map[int]dcReqs)
	var totalReqs int64

	for _, t := range propertyTraffic.DataRows {
		for _, dc := range t.Datacenters {
			r := dcReqMap[dc.DatacenterId]
			r.reqs += dc.Requests
			dcReqMap[dc.DatacenterId] = r
			totalReqs += dc.Requests
		}
	}

	// Prepare mapping for traffic targets
	type ttEnabled struct {
		ttName, ttNickname string
		ttEnabled          bool
		ttWeight           float64
	}
	ttEnabledMap := map[int]ttEnabled{}

	// Get domain config
	domResp, err := gtmClient.GetDomain(ctx, gtm.GetDomainRequest{DomainName: domainName})
	if err != nil {
		return nil, fmt.Errorf("fetching domain failed: %w", err)
	}
	domain := (*gtm.Domain)(domResp)

	// Extract traffic target config
	for _, prop := range domain.Properties {
		if prop.Name != qsProperty {
			continue
		}
		for _, tgt := range prop.TrafficTargets {
			entry := ttEnabled{
				ttName:    tgt.Name,
				ttEnabled: tgt.Enabled,
				ttWeight:  tgt.Weight,
			}
			if dc, ok := findDatacenterInDomain(domain, tgt.DatacenterID); ok {
				entry.ttNickname = dc.Nickname
			}
			ttEnabledMap[tgt.DatacenterID] = entry
		}
	}

	// Calculate request percentages
	for k, r := range dcReqMap {
		if totalReqs > 0 {
			r.perc = float64(r.reqs) / float64(totalReqs) * 100
		}
		dcReqMap[k] = r
	}

	// Group IPs by datacenter ID
	ipByDc := make(map[int][]*reportsgtm.IpStatIp)
	for _, dr := range propertyIpAvail.DataRows {
		for _, dc := range dr.Datacenters {
			ipByDc[dc.DatacenterId] = append(ipByDc[dc.DatacenterId], dc.IPs...)
		}
	}

	// Ensure all datacenters from domain config are present in ipByDc map
	for _, prop := range domain.Properties {
		if prop.Name != qsProperty {
			continue
		}
		for _, tgt := range prop.TrafficTargets {
			if _, exists := ipByDc[tgt.DatacenterID]; !exists {
				ipByDc[tgt.DatacenterID] = []*reportsgtm.IpStatIp{} // ensure presence
			}
		}
	}

	// Prepare metadata
	propStat.Domain = propertyIpAvail.Metadata.Domain
	propStat.PeriodStart = propertyTraffic.Metadata.Start
	propStat.PeriodEnd = propertyTraffic.Metadata.End
	propStat.ReportInterval = propertyTraffic.Metadata.Interval
	propStat.DatacenterIntervalStatus = propertyTraffic.DataRows

	summary := &PropertyStatusSummary{}
	if len(propertyIpAvail.DataRows) > 0 {
		summary.LastUpdate = propertyIpAvail.DataRows[0].Timestamp
		summary.CutOff = propertyIpAvail.DataRows[0].CutOff
	} else {
		summary.LastUpdate = "Not Available"
	}

	// Initialize full DC status list from domain-config targets
	var dcStatusList []*PropertyDCStatus
	for dcID, target := range ttEnabledMap {
		reqs := dcReqMap[dcID]
		ips := uniqueIPs(ipByDc[dcID]) // Get IPs even if empty

		dcStatus := &PropertyDCStatus{
			IpStatPerPropDRow: reportsgtm.IpStatPerPropDRow{
				DatacenterId:      dcID,
				TrafficTargetName: target.ttName,
				Nickname:          target.ttNickname,
				IPs:               ips,
			},
			DCTotalPeriodRequests: reqs.reqs,
			DCPropertyUsage:       fmt.Sprintf("%.2f%%", reqs.perc),
			DCEnabled:             target.ttEnabled,
			DCWeight:              target.ttWeight,
		}
		dcStatusList = append(dcStatusList, dcStatus)

		//  Debugging
		//fmt.Printf("[DEBUG] Added DC %d (%s) with %d IPs\n", dcID, target.ttName, len(ips))
	}

	summary.PropertyDCStatus = dcStatusList
	propStat.StatusSummary = summary

	return propStat, nil
}

// worker function for query-status
func cmdQueryStatus(c *cli.Context) error {

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
	err := ParseNicknames(c, qsDatacenters.nicknamesList, domainName)
	if err != nil {
		if verboseStatus {
			return cli.NewExitError(color.RedString("Unable to retrieve datacenter list. "+err.Error()), 1)
		} else {
			return cli.NewExitError(color.RedString("Unable to retrieve datacenter."), 1)
		}
	}
	if !c.IsSet("json") {
		fmt.Println("Querying status")
	}

	var objStatus interface{}

	if c.IsSet("datacenter") {
		if !c.IsSet("json") {
			fmt.Println("Collecting DC status ", "")
		}
		objStatus, err = gatherDatacenterStatus(c)
	} else if c.IsSet("property") {
		if !c.IsSet("json") {
			fmt.Println("Collecting Property status ", "")
		}
		objStatus, err = gatherPropertyStatus(c)
	} else {
		if !c.IsSet("json") {
			fmt.Println("Collecting Domain status ", "")
		}
		objStatus, err = getDomainStatus(c)
	}
	// check for failure
	if err != nil {
		if verboseStatus {
			return cli.NewExitError(color.RedString("Unable to retrieve status. "+err.Error()), 1)
		} else {
			return cli.NewExitError(color.RedString("Unable to retrieve status."), 1)
		}
	}

	if c.IsSet("json") && c.Bool("json") {
		json, err := json.MarshalIndent(objStatus, "", "  ")
		if err != nil {
			return cli.NewExitError(color.RedString("Unable to display status results"), 1)
		}
		fmt.Fprintln(c.App.Writer, string(json))
	} else {

		fmt.Fprintln(c.App.Writer, "")
		if c.IsSet("datacenter") {

			fmt.Fprintln(c.App.Writer, renderDatacenterTable(objStatus.(*DCTrafficStati)))

		} else if c.IsSet("property") {

			fmt.Fprintln(c.App.Writer, renderPropertyTable(objStatus.(*PropertyStatus)))

		} else {
			//fmt.Fprintln(c.App.Writer, renderDomainTable(objStatus.(*gtm.ResponseStatus), c))
			statusResp := objStatus.(*gtm.GetDomainStatusResponse)
			fmt.Fprintln(c.App.Writer, renderDomainTable((*gtm.ResponseStatus)(statusResp)))
		}
	}

	return nil

}

// Generate pretty print DC status
func renderDatacenterTable(objStatus *DCTrafficStati) string {

	var outString string
	outString += fmt.Sprintln("Domain: ", objStatus.Domain)
	outString += fmt.Sprintln("Period Start: ", objStatus.PeriodStart)
	outString += fmt.Sprintln("Period End: ", objStatus.PeriodEnd)
	outString += fmt.Sprintln(" ")
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)

	table.Header("Datacenter", "Nickname", "Timestamp", "Property", "Enabled", "Requests", "Status")

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

func renderPropertyTable(objStatus *PropertyStatus) string {
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

	table.Header("Datacenter", "Nickname", "Target Name", "Enabled", "Weight", "Total Requests", "Property Usage", "IP", "State")

	if len(objStatus.StatusSummary.PropertyDCStatus) == 0 {
		rowData := []string{"No status summary data available", " ", " ", " ", " ", " ", " ", " ", " "}
		table.Append(rowData)
	} else {
		for _, dc := range objStatus.StatusSummary.PropertyDCStatus {
			//fmt.Printf("[DEBUG] Processing Datacenter #%d: ID=%d, Nickname=%q, TargetName=%q\n", i, dc.DatacenterId, dc.Nickname, dc.TrafficTargetName)
			datacenterId := strconv.Itoa(dc.DatacenterId)
			nickname := dc.Nickname
			targetName := dc.TrafficTargetName
			enabled := strconv.FormatBool(dc.DCEnabled)
			weight := strconv.FormatFloat(dc.DCWeight, 'f', 1, 64)
			totalReqs := strconv.FormatInt(dc.DCTotalPeriodRequests, 10)
			usage := dc.DCPropertyUsage

			// Ensure at least one row even if there are no IPs
			ips := dc.IPs
			if len(ips) == 0 {
				//fmt.Printf("[DEBUG] Datacenter %d has no IPs, adding empty placeholder\n", dc.DatacenterId)
				ips = []*reportsgtm.IpStatIp{{}} // single empty IP entry
			}

			for k, ip := range ips {
				if ip == nil {
					//fmt.Printf("[WARN] Nil IP in Datacenter %d at index %d — skipping\n", dc.DatacenterId, k)
					continue
				}
				/*fmt.Printf("[DEBUG] IP #%d for Datacenter %d: %q (HandedOut=%v, Alive=%v, Score=%.2f)\n",
				k, dc.DatacenterId, ip.Ip, ip.HandedOut, ip.Alive, ip.Score)*/
				ipAddr := ip.Ip
				handedOut := fmt.Sprintf("HandedOut: %t", ip.HandedOut)
				score := fmt.Sprintf("Score: %.2f", ip.Score)
				alive := fmt.Sprintf("Alive: %t", ip.Alive)

				if k == 0 {
					row := []string{datacenterId, nickname, targetName, enabled, weight, totalReqs, usage, ipAddr, handedOut}
					//fmt.Printf("[DEBUG] First IP row: %v\n", row)
					table.Append(row)
				} else {
					row := []string{"", "", "", "", "", "", "", ipAddr, handedOut}
					//fmt.Printf("[DEBUG] Additional IP row: %v\n", row)
					table.Append(row)
				}
				table.Append([]string{"", "", "", "", "", "", "", score, ""})
				table.Append([]string{"", "", "", "", "", "", "", alive, ""})
			}
		}
	}
	table.Render()
	outString += fmt.Sprintln(tableString.String())

	// Build Datacenter Status Table
	outString += fmt.Sprintln(" ")
	outString += fmt.Sprintln("Datacenter Status")
	outString += fmt.Sprintln(" ")

	tableString = &strings.Builder{}
	dcTable := tablewriter.NewWriter(tableString)

	dcTable.Header("Timestamp", "Datacenter", "Nickname", "Requests", "Status")

	if len(objStatus.DatacenterIntervalStatus) == 0 {
		dcTable.Append([]string{"No datacenter interval status available", " ", " ", " ", " "})
	} else {
		for _, dcis := range objStatus.DatacenterIntervalStatus {
			for k, dc := range dcis.Datacenters {
				timestamp := dcis.Timestamp
				if k > 0 {
					timestamp = " "
				}
				row := []string{
					timestamp,
					strconv.Itoa(dc.DatacenterId),
					dc.Nickname,
					strconv.FormatInt(dc.Requests, 10),
					dc.Status,
				}
				dcTable.Append(row)
			}
		}
	}
	dcTable.Render()
	outString += fmt.Sprintln(tableString.String())

	return outString
}

// Pretty print output
func renderDomainTable(status *gtm.ResponseStatus) string {

	var outString string
	outString += fmt.Sprintln(" ")
	outString += fmt.Sprintf("Domain: %s\n", domainName)
	outString += fmt.Sprintln("Current Status")
	outString += fmt.Sprintln(" ")
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)

	// Build status table. Exclude Links.
	rowData := []string{"ChangeId", status.ChangeID}
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
