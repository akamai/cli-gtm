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
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_3"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
	"strconv"
	"strings"
)

// worker function for update-property
func cmdUpdateProperty(c *cli.Context) error {

	var dcWeight float64
	var dcServers []string
	var dcEnabled bool
	var dcDatacenters *arrayFlags
	var verboseStatus bool
	var dcNicknames []string

	config, err := akamai.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

	configgtm.Init(config)

	if c.NArg() < 2 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain and property are required"), 1)
	}

	domainName := c.Args().Get(0)
	propertyName := c.Args().Get(1)

	// Changes may be to enabled, weight or servers
	dcWeight = c.Float64("weight")
	dcServers = c.StringSlice("server")
	dcEnabled = c.BoolT("enabled")
	dcDatacenters = (c.Generic("datacenterid")).(*arrayFlags)
	dcNicknames = c.StringSlice("dcnickname")
	verboseStatus = c.Bool("verbose")

	if !c.IsSet("datacenterid") && !c.IsSet("dcnickname") {
		return cli.NewExitError(color.RedString("datacenter(s) must be specified"), 1)
	}
	// if nicknames specified, add to dcFlags
	if c.IsSet("dcnickname") {
		ParseNicknames(dcNicknames, domainName)
	}
	if c.IsSet("server") && len(dcDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("server update may only apply to one datacenter"), 1)
	}
	if c.IsSet("weight") && len(dcDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("weight update may only apply to one datacenter"), 1)
	}

	akamai.StartSpinner(
		"Updating property ...",
		fmt.Sprintf("Fetching "+propertyName+" ...... [%s]", color.GreenString("OK")),
	)

	property, err := configgtm.GetProperty(propertyName, domainName)
	if err != nil {
		akamai.StopSpinnerFail()
		return cli.NewExitError(color.RedString("Property not found"), 1)
	}

	changes_made := false

	fmt.Println(fmt.Sprintf("Property: %s", property.Name))
	trafficTargets := property.TrafficTargets
	targetsmsg := fmt.Sprintf("%s contains %s targets", property.Name, strconv.Itoa(len(trafficTargets)))
	fmt.Println(targetsmsg)
	fmt.Sprintf(targetsmsg)
	for _, traffTarg := range trafficTargets {
		for _, dcID := range dcDatacenters.flagList {
			if traffTarg.DatacenterId == dcID {
				fmt.Sprintf("%s contains dc %s", traffTarg.Name, strconv.Itoa(dcID))
				if c.IsSet("enabled") && traffTarg.Enabled != dcEnabled {
					traffTarg.Enabled = dcEnabled
					changes_made = true
				}
				if c.IsSet("weight") && traffTarg.Weight != dcWeight {
					// Note: weight will be ignored for a number of property types
					traffTarg.Weight = dcWeight
					changes_made = true
				}
				if c.IsSet("server") {
					traffTarg.Servers = dcServers
					changes_made = true

					/*
					   // See if we really are updating ...
					   if len(dcServers) != len(traffTarg.Servers) {
					           traffTarg.Servers = dcServers
					           changes_made = true
					   } else {
					           sort.Sort(dcServers)
					           sort.Sort(traffTarg.Servers)
					           for i, v := range traffTarg.Servers {
					                   if v != dcServers[i] {
					                           traffTarg.Servers = dcServers
					                           changes_made = true
					                   }
					           }
					   }
					*/
				}
			}
		}
	}
	if changes_made {
		propStat, err := property.Update(domainName)
		if err != nil {
			akamai.StopSpinnerFail()
			return cli.NewExitError(color.RedString(fmt.Sprintf("Error updating property %s. %s", propertyName, err.Error())), 1)
		}
		fmt.Fprintln(c.App.Writer, fmt.Sprintf("Property %s updated", propertyName))

		var status interface{}

		if c.IsSet("verbose") && verboseStatus {
			status = propStat
		} else {
			status = fmt.Sprintf("ChangeId: %s", propStat.ChangeId)
		}

		if c.IsSet("json") && c.Bool("json") {
			json, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				akamai.StopSpinnerFail()
				return cli.NewExitError(color.RedString("Unable to display status results"), 1)
			}
			fmt.Fprintln(c.App.Writer, string(json))
		} else {
			fmt.Fprintln(c.App.Writer, "")
			if c.IsSet("verbose") && verboseStatus {
				fmt.Fprintln(c.App.Writer, renderStatus(status.(*configgtm.ResponseStatus), c))
			} else {
				fmt.Fprintln(c.App.Writer, "Response Status")
				fmt.Fprintln(c.App.Writer, " ")
				fmt.Fprintln(c.App.Writer, status)
			}
		}
	} else {
		fmt.Fprintln(c.App.Writer, fmt.Sprintf("No update required for Property %s", propertyName))
	}

	akamai.StopSpinnerOk()

	return nil

}

// Pretty print output
func renderStatus(status *configgtm.ResponseStatus, c *cli.Context) string {

	var outString string
	outString += fmt.Sprintln(" ")
	outString += fmt.Sprintln("Response Status")
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
