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
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_4"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultInterval int = 5
const defaultTimeout int = 300

// worker function for update-property
func cmdUpdateProperty(c *cli.Context) error {

	var pWeight float64
	var pTargets *TargetFlags
	var pServers []string
	var pLivenessTests []string
	var pEnabled bool = true
	var pDatacenters *arrayFlags
	var pComplete bool = false
	var pTimeout int = defaultTimeout
	var pDryrun bool = false
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
	pWeight = c.Float64("weight")
	pServers = c.StringSlice("server")
	pLivenessTests = c.StringSlice("liveness_test")
	fmt.Println("pLivenessTests: ", pLivenessTests)
	if c.IsSet("enable") && c.IsSet("disable") {
		return cli.NewExitError(color.RedString("must specified either enable or disable."), 1)
	} else if c.IsSet("enable") {
		pEnabled = true
	} else if c.IsSet("disable") {
		pEnabled = false
	}
	pDatacenters = (c.Generic("datacenter")).(*arrayFlags)
	pTargets = (c.Generic("target")).(*TargetFlags)
	if c.IsSet("verbose") {
		verboseStatus = true
	}
	if c.IsSet("complete") {
		pComplete = true
	}
	if c.IsSet("dryrun") {
		pDryrun = true
	}
	if c.IsSet("timeout") {
		pTimeout = c.Int("timeout")
	}
	if c.IsSet("datacenter") && c.IsSet("liveness_test") && (c.IsSet("enable") || c.IsSet("disable")) {
		return cli.NewExitError(color.RedString("enable/disable can only be applied to either datacenter(s) OR liveness_test(s)"), 1)
	}
	if !c.IsSet("target") && !c.IsSet("datacenter") && !c.IsSet("liveness_test") {
		return cli.NewExitError(color.RedString("datacenter(s), target(s) and/or liveness_test(s)s must be specified"), 1)
	}
	// if nicknames specified, add to dcFlags
	err = ParseNicknames(pDatacenters.nicknamesList, domainName)
	if err != nil {
		if verboseStatus {
			return cli.NewExitError(color.RedString("Unable to retrieve datacenter list. "+err.Error()), 1)
		} else {
			return cli.NewExitError(color.RedString("Unable to retrieve datacenter."), 1)
		}
	}
	if !c.IsSet("datacenter") && !c.IsSet("liveness_test") && (c.IsSet("enable") || c.IsSet("disable")) {
		return cli.NewExitError(color.RedString("datacenter(s) or liveness_test(s) must be specified when enable or disable are specified"), 1)
	}
	if !c.IsSet("datacenter") && (c.IsSet("server") || c.IsSet("weight")) {
		return cli.NewExitError(color.RedString("datacenter(s) must be specified when server or weight field changes are specified"), 1)
	}
	if c.IsSet("liveness_test") && !(c.IsSet("enable") || c.IsSet("disable")) {
		return cli.NewExitError(color.RedString("liveness_test(s) specified without enable or disable directive"), 1)
	}
	if c.IsSet("datacenter") && !(c.IsSet("server") || c.IsSet("weight") || c.IsSet("enable") || c.IsSet("disable")) {
		return cli.NewExitError(color.RedString("datacenter(s) specified with no field changes"), 1)
	}
	for _, dcID := range pDatacenters.flagList {
		if _, ok := pTargets.targetList[dcID]; ok {
			return cli.NewExitError(color.RedString("datacenters and targets cannot be the same"), 1)
		}
	}
	if c.IsSet("server") && len(pDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("server update may only apply to one datacenter"), 1)
	}
	if c.IsSet("weight") && len(pDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("weight update may only apply to one datacenter"), 1)
	}
	if c.IsSet("json") {
		fmt.Println(fmt.Sprintf("Updating property %s", propertyName))
	}

	property, err := configgtm.GetProperty(propertyName, domainName)
	if err != nil {
		return cli.NewExitError(color.RedString("Property not found"), 1)
	}

	changes_made := false
	trafficTargets := property.TrafficTargets
	targetsmsg := fmt.Sprintf("%s contains %s targets", property.Name, strconv.Itoa(len(trafficTargets)))
	if !c.IsSet("json") {
		fmt.Println(targetsmsg)
	}
	fmt.Sprintf(targetsmsg)
	akamai.StartSpinner("Updating Traffic Targets ", "")
	var propTargets = map[int]string{}
	for _, traffTarg := range trafficTargets {
		// Al traffic target fields can be updated via target.
		if c.IsSet("target") {
			for _, targ := range pTargets.targets {
				propTargets[traffTarg.DatacenterId] = ""
				if traffTarg.DatacenterId == targ.DatacenterId {
					// required
					if traffTarg.Weight != targ.Weight {
						traffTarg.Weight = targ.Weight
						changes_made = true
					}
					// required
					if traffTarg.Enabled != targ.Enabled {
						traffTarg.Enabled = targ.Enabled
						changes_made = true
					}
					// optional
					if len(targ.Servers) > 0 {
						if len(targ.Servers) != len(traffTarg.Servers) {
							traffTarg.Servers = targ.Servers
							changes_made = true
						} else {
							sort.Strings(targ.Servers)
							sort.Strings(traffTarg.Servers)
							for i, v := range traffTarg.Servers {
								if v != targ.Servers[i] {
									traffTarg.Servers = targ.Servers
									changes_made = true
								}
							}
						}
					}
					// optional
					if traffTarg.HandoutCName != targ.HandoutCName && targ.HandoutCName != "" {
						traffTarg.HandoutCName = targ.HandoutCName
						changes_made = true
					}
					// optional
					if traffTarg.Name != targ.Name && targ.Name != "" {
						traffTarg.Name = targ.Name
						changes_made = true
					}
				}
			}
		}

		for _, dcID := range pDatacenters.flagList {
			if traffTarg.DatacenterId == dcID {
				fmt.Sprintf("%s contains dc %s", traffTarg.Name, strconv.Itoa(dcID))
				if (c.IsSet("enable") || c.IsSet("disable")) && traffTarg.Enabled != pEnabled {
					traffTarg.Enabled = pEnabled
					changes_made = true
				}
				if c.IsSet("weight") && traffTarg.Weight != pWeight {
					// Note: weight will be ignored for a number of property types
					traffTarg.Weight = pWeight
					changes_made = true
				}
				if c.IsSet("server") {
					traffTarg.Servers = pServers
					changes_made = true
				}
			}
		}
	}

	if c.IsSet("target") {
		// Any new target?
		for cmdTarget, _ := range pTargets.targetList {
			if _, ok := propTargets[cmdTarget]; !ok {
				// New target. Find it
				for _, t := range pTargets.targets {
					if t.DatacenterId == cmdTarget {
						property.TrafficTargets = append(property.TrafficTargets, &t)
						changes_made = true
						break
					}
				}
			}
		}
	}

	// enable/disable property liveness tests?
	if len(pLivenessTests) > 0 {
		testList := strings.Join(pLivenessTests, " ")
		fmt.Println("livesness tests: ", testList)
		for _, test := range property.LivenessTests {
			fmt.Println("Processing livesness test: ", test.Name)
			if strings.Contains(testList, test.Name) {
				fmt.Println("Livesness test match!")
				fmt.Println("pEnabled: ", pEnabled)
				fmt.Println("test.Disabled: ", test.Disabled)
				if (c.IsSet("enable") || c.IsSet("disable")) && test.Disabled != !pEnabled {
					// logic is reversed.
					test.Disabled = !pEnabled
					changes_made = true
				}
			}
		}
	}

	if changes_made {

		if pDryrun {
			json, err := json.MarshalIndent(property, "", "  ")
			if err != nil {
				return cli.NewExitError(color.RedString("Unable to display proposed property update"), 1)
			}
			fmt.Fprintln(c.App.Writer, "Proposed Property Update")
			fmt.Fprintln(c.App.Writer, string(json))

			if !c.IsSet("json") {
				akamai.StopSpinnerOk()
			}

			return nil
		}

		propStat, err := property.Update(domainName)
		if err != nil {
			akamai.StopSpinnerFail()
			return cli.NewExitError(color.RedString(fmt.Sprintf("Error updating property %s. %s", propertyName, err.Error())), 1)
		}
		if !c.IsSet("json") {
			akamai.StopSpinnerOk()
		}
		// wait to complete?
		if pComplete && propStat.PropagationStatus == "PENDING" {
			var sleepInterval time.Duration = 1 // seconds. TODO:Should be configurable by user ...
			var sleepTimeout time.Duration = 1  // seconds. TODO: Should be configurable by user ...
			sleepInterval *= time.Duration(defaultInterval)
			sleepTimeout *= time.Duration(pTimeout)
			if !c.IsSet("json") {
				fmt.Println(" ")
				akamai.StartSpinner("Waiting for completion ", "")
			}
			for {
				time.Sleep(sleepInterval * time.Second)
				sleepTimeout -= sleepInterval
				if propStat.PropagationStatus == "COMPLETE" {
					if !c.IsSet("json") {
						akamai.StopSpinner("[Change deployed]", true)
					}
					break
				} else if propStat.PropagationStatus == "DENIED" {
					if !c.IsSet("json") {
						akamai.StopSpinner("[Change denied]", true)
					}
					break
				}
				if sleepTimeout <= 0 {
					if !c.IsSet("json") {
						akamai.StopSpinner("[Maximum wait time elapsed. Use query-status confirm successful deployment]", true)
					}
					break
				}
				propStat, err = configgtm.GetDomainStatus(domainName)
				if err != nil {
					if !c.IsSet("json") {
						akamai.StopSpinner("[Unable to retrieve domain status]", true)
					}
					break
				}
			}
		}
		if c.IsSet("json") {
			fmt.Fprintln(c.App.Writer, fmt.Sprintf("Property %s updated", propertyName))
		}
		var status interface{}

		if c.IsSet("verbose") && verboseStatus {
			status = propStat
		} else {
			status = fmt.Sprintf("ChangeId: %s", propStat.ChangeId)
		}

		if c.IsSet("json") && c.Bool("json") {
			json, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
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
		if !c.IsSet("json") {
			akamai.StopSpinnerOk()
			fmt.Fprintln(c.App.Writer, fmt.Sprintf("No update required for Property %s", propertyName))
		}
	}

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
