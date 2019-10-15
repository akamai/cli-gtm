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
	"time"
)

// worker function for update-property
func cmdUpdateProperty(c *cli.Context) error {

	var pWeight float64
	var pServers []string
	var pEnabled bool = true
	var pDatacenters *arrayFlags
        var pComplete bool = false
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
        if c.IsSet("enable") && c.IsSet("disable") {
		return cli.NewExitError(color.RedString("must specified either enable or disable."), 1)
        } else if c.IsSet("enable") {
		pEnabled = true 
	} else if c.IsSet("disable") {
                pEnabled = false 
        }
	pDatacenters = (c.Generic("datacenter")).(*arrayFlags)
        if c.IsSet("verbose") {
		verboseStatus = true
	}
        if c.IsSet("complete") {
		pComplete = true
	}
	if !c.IsSet("datacenter") {
		return cli.NewExitError(color.RedString("datacenter(s) must be specified"), 1)
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
	if c.IsSet("server") && len(pDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("server update may only apply to one datacenter"), 1)
	}
	if c.IsSet("weight") && len(pDatacenters.flagList) > 1 {
		return cli.NewExitError(color.RedString("weight update may only apply to one datacenter"), 1)
	}
        if c.IsSet("json") {
		akamai.StartSpinner("", "")
	} else {
		akamai.StartSpinner(
			"Updating property ...",
			fmt.Sprintf("Fetching "+propertyName+" ...... [%s]", color.GreenString("OK")),)
	}

	property, err := configgtm.GetProperty(propertyName, domainName)
	if err != nil {
		akamai.StopSpinnerFail()
		return cli.NewExitError(color.RedString("Property not found"), 1)
	}

	changes_made := false
        if !c.IsSet("json") {
		fmt.Println(fmt.Sprintf("Property: %s", property.Name))
	}
	trafficTargets := property.TrafficTargets
	targetsmsg := fmt.Sprintf("%s contains %s targets", property.Name, strconv.Itoa(len(trafficTargets)))
        if !c.IsSet("json") {
		fmt.Println(targetsmsg)
	}
	fmt.Sprintf(targetsmsg)
	for _, traffTarg := range trafficTargets {
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

					/*
					   // See if we really are updating ...
					   if len(pServers) != len(traffTarg.Servers) {
					           traffTarg.Servers = pServers
					           changes_made = true
					   } else {
					           sort.Sort(pServers)
					           sort.Sort(traffTarg.Servers)
					           for i, v := range traffTarg.Servers {
					                   if v != pServers[i] {
					                           traffTarg.Servers = pServers
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

                // wait to complete?
		if pComplete && propStat.PropagationStatus == "PENDING" {
                        var defaultInterval int = 5
  			var defaultTimeout int = 300
			var sleepInterval time.Duration = 1 // seconds. TODO:Should be configurable by user ...
			var sleepTimeout time.Duration = 1 // seconds. TODO: Should be configurable by user ... 
                        sleepInterval *= time.Duration(defaultInterval)
			sleepTimeout *= time.Duration(defaultTimeout) 
		        if !c.IsSet("json") {
				fmt.Println(" ")
				fmt.Printf("Waiting for completion .")
			}
                        for {
			        if !c.IsSet("json") {
					fmt.Printf(".")
				}
				time.Sleep(sleepInterval * time.Second) 
				sleepTimeout -= sleepInterval
				if propStat.PropagationStatus == "COMPLETE" {
				        if !c.IsSet("json") {
						fmt.Println(" ")
						fmt.Println("Change deployed")
					}
					break
	                        } else if propStat.PropagationStatus == "DENIED" {
        	                        if !c.IsSet("json") {
                	                        fmt.Println(" ")
                        	                fmt.Println("Change denied")
                                	}
                                	break
				}
                                if sleepTimeout <= 0 {
				        if !c.IsSet("json") {
						fmt.Println(" ")
                                        	fmt.Println("Maximum wait time elapsed. Use query-status confirm successful deployment")
                                        }
					break
                                }       
				propStat, err = configgtm.GetDomainStatus(domainName)
				if err != nil {
				        if !c.IsSet("json") {
						fmt.Println(" ")
                                        	fmt.Println("Unable to retrieve domain status")
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
				akamai.StopSpinnerFail()
				return cli.NewExitError(color.RedString("Unable to display status results"), 1)
			}
			fmt.Fprintln(c.App.Writer, string(json))
			akamai.StopSpinner("", true)
		} else {
			fmt.Fprintln(c.App.Writer, "")
			if c.IsSet("verbose") && verboseStatus {
				fmt.Fprintln(c.App.Writer, renderStatus(status.(*configgtm.ResponseStatus), c))
			} else {
				fmt.Fprintln(c.App.Writer, "Response Status")
				fmt.Fprintln(c.App.Writer, " ")
				fmt.Fprintln(c.App.Writer, status)
			}
			akamai.StopSpinnerOk()
		}
	} else {
		if c.IsSet("json") {
			akamai.StopSpinner("", true)
		} else {
			fmt.Fprintln(c.App.Writer, fmt.Sprintf("No update required for Property %s", propertyName))
			akamai.StopSpinnerOk()
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
