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
	"strconv"
	"strings"
	"time"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

var dcEnabled bool = true
var dcComplete bool = false
var dcDatacenters *arrayFlags

var succShortArray []*SuccUpdateShort
var succVerboseArray []*SuccUpdateVerbose
var failedArray []*FailUpdate

// worker function for update-datacenter
func cmdUpdateDatacenter(c *cli.Context) error {

	config, err := akamai.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

	configgtm.Init(config)

	if c.NArg() == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain name is required"), 1)
	}

	domainName := c.Args().First()
	dcDatacenters = c.Generic("datacenter").(*arrayFlags)
        if c.IsSet("enable") && c.IsSet("disable") {
                return cli.NewExitError(color.RedString("must specified either enable or disable."), 1)
        } else if c.IsSet("enable") {
                dcEnabled = true
        } else if c.IsSet("disable") {
                dcEnabled = false
        }
        if c.IsSet("verbose") {
                verboseStatus = true
        }
        if c.IsSet("complete") {
                dcComplete = true
        }
	// if nicknames specified, add to dcFlags
        err = ParseNicknames(dcDatacenters.nicknamesList, domainName)
	if err != nil {
                if verboseStatus {
                        return cli.NewExitError(color.RedString("Unable to retrieve datacenter list. "+err.Error()), 1)
                } else {
                        return cli.NewExitError(color.RedString("Unable to retrieve datacenter."), 1)
                }
        }
	if !c.IsSet("datacenter") || len(dcDatacenters.flagList) == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("One or more datacenters is required"), 1)
	}

	if c.IsSet("json") {
		akamai.StartSpinner("", "")
	} else {
	akamai.StartSpinner(
		"Fetching data...",
		fmt.Sprintf("Fetching domain properties ...... [%s]", color.GreenString("OK")),)
	}

	// get domain. Serves two purposes. Validates domain exists and retrieves all the properties
        if !c.IsSet("json") {
		fmt.Println("Domain: ", domainName)
	}
	dom, err := configgtm.GetDomain(domainName)

	if err != nil {
		akamai.StopSpinnerFail()
		return cli.NewExitError(color.RedString("Domain "+domainName+" not found "), 1)
	}
	properties := dom.Properties
	propmsg := fmt.Sprintf("%s contains %s properties", domainName, strconv.Itoa(len(properties)))
	fmt.Sprintf(propmsg)
        if !c.IsSet("json") {
		fmt.Println(propmsg)
	}
	for _, propPtr := range properties {
		changes_made := false
        	if !c.IsSet("json") {
			fmt.Println(fmt.Sprintf("Property: %s", propPtr.Name))
		}
		trafficTargets := propPtr.TrafficTargets
		targetsmsg := fmt.Sprintf("%s contains %s targets", propPtr.Name, strconv.Itoa(len(trafficTargets)))
	        if !c.IsSet("json") {
			fmt.Println(targetsmsg)
		}
		fmt.Sprintf(targetsmsg)
		for _, traffTarg := range trafficTargets {
			dcs := dcDatacenters
			for _, dcID := range dcs.flagList {
				if traffTarg.DatacenterId == dcID && (c.IsSet("enable") || c.IsSet("disable")) && traffTarg.Enabled != dcEnabled {
					fmt.Sprintf("%s contains dc %s", traffTarg.Name, strconv.Itoa(dcID))
					traffTarg.Enabled = dcEnabled
					changes_made = true
				}
			}
		}
		if changes_made {
			stat, err := propPtr.Update(domainName)
			if err != nil {
				propError := &FailUpdate{PropName: propPtr.Name, FailMsg: err.Error()}
				failedArray = append(failedArray, propError)
			} else {
				if c.IsSet("verbose") && verboseStatus {
					verbStat := &SuccUpdateVerbose{PropName: propPtr.Name, RespStat: stat}
					succVerboseArray = append(succVerboseArray, verbStat)
				} else {
					shortStat := &SuccUpdateShort{PropName: propPtr.Name, ChangeId: stat.ChangeId}
					succShortArray = append(succShortArray, shortStat)
				}
			}
		}
	}

	if dcComplete && (len(succVerboseArray) > 0 || len(succShortArray) > 0) {
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
                        dStat, err := configgtm.GetDomainStatus(domainName)
			if err != nil {
				fmt.Println(" ")
				fmt.Println("Unable to retrieve domain status.")
                                break
                        }
		        if !c.IsSet("json") {
                		fmt.Printf(".")
                        }
			time.Sleep(sleepInterval * time.Second)
                        sleepTimeout -= sleepInterval
                        if dStat.PropagationStatus == "COMPLETE" {
			        if !c.IsSet("json") {
					fmt.Println(" ")
                                	fmt.Println("Change deployed")
                                }
				break
                        } else if dStat.PropagationStatus == "DENIED" {
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
                }
	}

	if len(properties) == 1 && len(failedArray) > 0 {
		akamai.StopSpinnerFail()
		return cli.NewExitError(color.RedString(fmt.Sprintf("Error updating property %s: %s", failedArray[0].PropName, failedArray[0].FailMsg)), 1)
	}

	updateSum := UpdateSummary{}
	if c.IsSet("verbose") && verboseStatus && len(succVerboseArray) > 0 {
		updateSum.Updated_Properties = succVerboseArray
	} else if len(succShortArray) > 0 {
		updateSum.Updated_Properties = succShortArray
	}
	if len(failedArray) > 0 {
		updateSum.Failed_Updates = failedArray
	}

	if updateSum.Failed_Updates == nil && updateSum.Updated_Properties == nil {
		if c.IsSet("json") {
			akamai.StopSpinner("", true)
		} else {
			fmt.Fprintln(c.App.Writer, "No property updates were needed.")
			akamai.StopSpinnerOk()
		}
	} else {
		if c.IsSet("json") && c.Bool("json") {
			json, err := json.MarshalIndent(updateSum, "", "  ")
			if err != nil {
				akamai.StopSpinnerFail()
				return cli.NewExitError(color.RedString("Unable to display status results"), 1)
			}
			fmt.Fprintln(c.App.Writer, string(json))
			akamai.StopSpinner("", true)
		} else {
			fmt.Fprintln(c.App.Writer, "")
			fmt.Fprintln(c.App.Writer, renderDCStatus(updateSum, c))
			akamai.StopSpinnerOk()
		}
	}

	return nil

}

func renderDCStatus(upSum UpdateSummary, c *cli.Context) string {

	var outString string
	outString += fmt.Sprintln(" ")
	outString += fmt.Sprintln("Datacenter Update Summary")
	outString += fmt.Sprintln(" ")
	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)

	table.SetReflowDuringAutoWrap(false)
	table.SetCenterSeparator(" ")
	table.SetColumnSeparator(" ")
	table.SetRowSeparator(" ")
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetColumnAlignment([]int{tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT, tablewriter.ALIGN_LEFT})
	table.SetAlignment(tablewriter.ALIGN_CENTER)

	// Build summary table. Exclude Links in status.
	rowData := []string{"Completed Updates", " ", " ", " "}
	table.Append(rowData)
	if c.IsSet("verbose") && verboseStatus {
		if len(succVerboseArray) == 0 {
			rowData := []string{" ", "No successful updates", " ", " "}
			table.Append(rowData)
		} else {
			for _, prop := range succVerboseArray {
				rowData := []string{" ", prop.PropName, "ChangeId", prop.RespStat.ChangeId}
				table.Append(rowData)
				rowData = []string{" ", " ", "Message", prop.RespStat.Message}
				table.Append(rowData)
				rowData = []string{" ", " ", "Passing Validation", strconv.FormatBool(prop.RespStat.PassingValidation)}
				table.Append(rowData)
				rowData = []string{" ", " ", "Propagation Status", prop.RespStat.PropagationStatus}
				table.Append(rowData)
				rowData = []string{" ", " ", "Propagation Status Date", prop.RespStat.PropagationStatusDate}
				table.Append(rowData)
			}
		}
	} else {
		if len(succShortArray) == 0 {
			rowData := []string{" ", "No successful updates", " ", " "}
			table.Append(rowData)
		} else {
			for _, prop := range succShortArray {
				rowData := []string{" ", prop.PropName, "ChangeId", prop.ChangeId}
				table.Append(rowData)
			}
		}
	}

	rowData = []string{"Failed Updates", " ", " ", " "}
	table.Append(rowData)
	if len(failedArray) == 0 {
		rowData := []string{" ", "No failed property updates", " ", " "}
		table.Append(rowData)
	} else {
		for _, prop := range failedArray {
			rowData := []string{" ", prop.PropName, "Failure Message", prop.FailMsg}
			table.Append(rowData)
		}
	}

	table.Render()
	outString += fmt.Sprintln(tableString.String())

	return outString

}
