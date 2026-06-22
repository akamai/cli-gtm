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
	"cli-gtm/edgegrid"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg/gtm"
	"github.com/fatih/color"
	"github.com/urfave/cli"
)

var dcTimeout int = defaultTimeout
var dcDryrun bool = false
var dcEnabled bool = true
var dcComplete bool = false
var dcDatacenters *arrayFlags

var succShortArray []*SuccUpdateShort
var succVerboseArray []*SuccUpdateVerbose
var failedArray []*FailUpdate
var dryrunArray []json.RawMessage

// worker function for update-datacenter
func cmdUpdateDatacenter(c *cli.Context) error {
	failStep := func(step, message string, args ...interface{}) error {
		if !c.IsSet("json") {
			fmt.Printf("%s ... %s\n", step, color.RedString("[FAIL]"))
		}
		return cli.NewExitError(color.RedString(message, args...), 1)
	}
	printOK := func(step string) {
		if !c.IsSet("json") {
			fmt.Printf("%s ... %s\n", step, color.GreenString("[OK]"))
		}
	}

	//Initialize Edgegrid session and context
	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return fmt.Errorf("session failed %v", err)
	}
	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(edgegrid.GetSession(ctx))

	// Validate domain name
	if c.NArg() == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain name is required"), 1)
	}

	// Get domain name and CLI flags
	domainName := c.Args().First()
	updateStep := fmt.Sprintf("Updating Datacenter(s) in domain %s", domainName)
	dcDatacenters = c.Generic("datacenter").(*arrayFlags)
	if c.IsSet("enable") && c.IsSet("disable") {
		return failStep(updateStep, "must specify either enable or disable.")
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
	if c.IsSet("dryrun") {
		dcDryrun = true
	}
	if c.IsSet("timeout") {
		dcTimeout = c.Int("timeout")
	}

	// Resolve nicknames to datacenter Ids
	if err := ParseNicknames(c, dcDatacenters.nicknamesList, domainName); err != nil {
		msg := "Unable to retrieve datacenter."
		if verboseStatus {
			msg += " " + err.Error()
		}
		return failStep(updateStep, msg)
	}
	if len(dcDatacenters.flagList) == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return failStep(updateStep, "One or more datacenters is required")
	}

	// 1. List all properties in given domain
	properties, err := gtmClient.ListProperties(ctx, gtm.ListPropertiesRequest{DomainName: domainName})
	if err != nil {
		return failStep(updateStep, "Unable to list properties: "+err.Error())
	}
	printOK(updateStep)
	propmsg := fmt.Sprintf("%s contains %d properties", domainName, len(properties))
	if !c.IsSet("json") {
		fmt.Println(propmsg)
	}

	// 2. Iterate each property & apply changes
	for _, prop := range properties {
		changesMade := false
		propertyStep := fmt.Sprintf("Updating Property: %s", prop.Name)
		targetsmsg := fmt.Sprintf("%s contains %d targets", prop.Name, len(prop.TrafficTargets))
		if !c.IsSet("json") {
			fmt.Println(targetsmsg)
		}

		for i, tt := range prop.TrafficTargets {
			for _, dcID := range dcDatacenters.flagList {
				if tt.DatacenterID == dcID && (!tt.Enabled && dcEnabled || tt.Enabled && !dcEnabled) {
					prop.TrafficTargets[i].Enabled = dcEnabled
					changesMade = true
				}
			}
		}

		// If changes were made, apply them or record in dry-run
		if changesMade {
			if dcDryrun {
				b, err := json.MarshalIndent(prop, "", "  ")
				if err != nil {
					failedArray = append(failedArray, &FailUpdate{PropName: prop.Name, FailMsg: err.Error()})
					if !c.IsSet("json") {
						fmt.Printf("%s ... %s\n", propertyStep, color.RedString("[FAIL]"))
					}
				} else {
					dryrunArray = append(dryrunArray, json.RawMessage(b))
					printOK(propertyStep)
				}
				continue
			}

			resp, err := gtmClient.UpdateProperty(ctx, gtm.UpdatePropertyRequest{
				DomainName: domainName, Property: &prop,
			})
			if err != nil {
				failedArray = append(failedArray, &FailUpdate{PropName: prop.Name, FailMsg: err.Error()})
				if !c.IsSet("json") {
					fmt.Printf("%s ... %s\n", propertyStep, color.RedString("[FAIL]"))
				}
			} else {
				if c.IsSet("verbose") && verboseStatus {
					succVerboseArray = append(succVerboseArray, &SuccUpdateVerbose{PropName: prop.Name, RespStat: resp.Status})
				} else {
					succShortArray = append(succShortArray, &SuccUpdateShort{PropName: prop.Name, ChangeId: resp.Status.ChangeID})
				}
				printOK(propertyStep)
			}
		} else {
			printOK(propertyStep)
		}
	}

	// 3. Wait for propagation if requested
	if dcComplete && (len(succShortArray)+len(succVerboseArray) > 0) {
		timeout := time.Duration(dcTimeout) * time.Second
		interval := time.Duration(defaultInterval) * time.Second
		completionStep := "Waiting for completion"
		completionOK := true
		for timeout > 0 {
			domainStatus, err := gtmClient.GetDomainStatus(ctx, gtm.GetDomainStatusRequest{DomainName: domainName})
			if err != nil {
				if !c.IsSet("json") {
					fmt.Printf("%s ... %s\n", completionStep, color.RedString("[FAIL]"))
					fmt.Printf("Error getting domain status: %v\n", err)
				}
				completionOK = false
				break
			}
			if domainStatus.PropagationStatus == "COMPLETE" || domainStatus.PropagationStatus == "DENIED" {
				break
			}
			time.Sleep(interval)
			timeout -= interval
		}
		if completionOK {
			printOK(completionStep)
		}
	}

	// 4. Prepare summary & output
	updateSum := UpdateSummary{}
	if dcDryrun {
		updateSum.Updated_Properties = dryrunArray
		updateSum.Failed_Updates = failedArray
		b, _ := json.MarshalIndent(updateSum, "", "  ")
		fmt.Fprintln(c.App.Writer, string(b))
		return nil
	}
	if len(succVerboseArray) > 0 {
		updateSum.Updated_Properties = succVerboseArray
	} else if len(succShortArray) > 0 {
		updateSum.Updated_Properties = succShortArray
	}
	updateSum.Failed_Updates = failedArray

	// Output summary in JSON or table format
	if updateSum.Updated_Properties == nil && updateSum.Failed_Updates == nil {
		if !c.IsSet("json") {
			fmt.Fprintln(c.App.Writer, "No property updates were needed.")
		}
	} else if c.IsSet("json") {
		b, _ := json.MarshalIndent(updateSum, "", "  ")
		fmt.Fprintln(c.App.Writer, string(b))
	} else {
		fmt.Fprintln(c.App.Writer, "\n"+renderDCStatus(c))
	}
	return nil
}

// Renders datacenter update summary in tabular format
func renderDCStatus(c *cli.Context) string {
	var outString strings.Builder
	outString.WriteString("Datacenter Update Summary:\n")

	// Completed Updates
	outString.WriteString("Completed Updates:\n")
	if c.IsSet("verbose") && verboseStatus {
		if len(succVerboseArray) == 0 {
			outString.WriteString("  (none)\n")
		} else {
			for _, prop := range succVerboseArray {
				outString.WriteString(fmt.Sprintf("  %s\n", prop.PropName))
				outString.WriteString(fmt.Sprintf("    ChangeId: %s\n", prop.RespStat.ChangeID))
				outString.WriteString(fmt.Sprintf("    Message: %s\n", prop.RespStat.Message))
				outString.WriteString(fmt.Sprintf("    Passing Validation: %s\n", strconv.FormatBool(prop.RespStat.PassingValidation)))
				outString.WriteString(fmt.Sprintf("    Propagation Status: %s\n", prop.RespStat.PropagationStatus))
				outString.WriteString(fmt.Sprintf("    Status Date: %s\n", prop.RespStat.PropagationStatusDate))
			}
		}
	} else {
		if len(succShortArray) == 0 {
			outString.WriteString("  (none)\n")
		} else {
			for _, prop := range succShortArray {
				outString.WriteString(fmt.Sprintf("  %s (%s)\n", prop.PropName, prop.ChangeId))
			}
		}
	}

	// Failed Updates
	outString.WriteString("Failed Updates:\n")
	if len(failedArray) == 0 {
		outString.WriteString("  (none)\n")
	} else {
		for _, prop := range failedArray {
			outString.WriteString(fmt.Sprintf("  %s: %s\n", prop.PropName, prop.FailMsg))
		}
	}

	return outString.String()
}
