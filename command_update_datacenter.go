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
	"github.com/olekukonko/tablewriter"
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
	dcDatacenters = c.Generic("datacenter").(*arrayFlags)
	if c.IsSet("enable") && c.IsSet("disable") {
		return cli.NewExitError(color.RedString("must specify either enable or disable."), 1)
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
		return cli.NewExitError(color.RedString(msg), 1)
	}
	if len(dcDatacenters.flagList) == 0 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("One or more datacenters is required"), 1)
	}

	if !c.IsSet("json") {
		fmt.Printf("Updating Datacenter(s) in domain %s\n", domainName)
	}

	// 1. List all properties in given domain
	properties, err := gtmClient.ListProperties(ctx, gtm.ListPropertiesRequest{DomainName: domainName})
	if err != nil {
		return cli.NewExitError(color.RedString("Unable to list properties: "+err.Error()), 1)
	}
	propmsg := fmt.Sprintf("%s contains %d properties", domainName, len(properties))
	if !c.IsSet("json") {
		fmt.Println(propmsg)
	}

	// 2. Iterate each property & apply changes
	for _, prop := range properties {
		changesMade := false
		if !c.IsSet("json") {
			fmt.Printf("Updating Property: %s\n", prop.Name)
		}
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
				} else {
					dryrunArray = append(dryrunArray, json.RawMessage(b))
				}
				continue
			}

			resp, err := gtmClient.UpdateProperty(ctx, gtm.UpdatePropertyRequest{
				DomainName: domainName, Property: &prop,
			})
			if err != nil {
				failedArray = append(failedArray, &FailUpdate{PropName: prop.Name, FailMsg: err.Error()})
			} else {
				if c.IsSet("verbose") && verboseStatus {
					succVerboseArray = append(succVerboseArray, &SuccUpdateVerbose{PropName: prop.Name, RespStat: resp.Status})
				} else {
					succShortArray = append(succShortArray, &SuccUpdateShort{PropName: prop.Name, ChangeId: resp.Status.ChangeID})
				}
			}
		}
	}

	// 3. Wait for propagation if requested
	if dcComplete && (len(succShortArray)+len(succVerboseArray) > 0) {
		timeout := time.Duration(dcTimeout) * time.Second
		interval := time.Duration(defaultInterval) * time.Second
		if !c.IsSet("json") {
			fmt.Println("Waiting for completion...")
		}
		for timeout > 0 {
			domainStatus, err := gtmClient.GetDomainStatus(ctx, gtm.GetDomainStatusRequest{DomainName: domainName})
			if err != nil {
				fmt.Printf("Error getting domain status: %v\n", err)
				break
			}
			if domainStatus.PropagationStatus == "COMPLETE" || domainStatus.PropagationStatus == "DENIED" {
				break
			}
			time.Sleep(interval)
			timeout -= interval
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

// Renders datacenter update summary in table format
func renderDCStatus(c *cli.Context) string {
	var outString strings.Builder
	outString.WriteString("\nDatacenter Update Summary\n\n")

	tableString := &strings.Builder{}
	table := tablewriter.NewWriter(tableString)

	// Completed Updates
	table.Append([]string{"Completed Updates", " ", " ", " "})

	if c.IsSet("verbose") && verboseStatus {
		if len(succVerboseArray) == 0 {
			table.Append([]string{" ", "No successful updates", " ", " "})
		} else {
			for _, prop := range succVerboseArray {
				table.Append([]string{" ", prop.PropName, "ChangeId", prop.RespStat.ChangeID})
				table.Append([]string{" ", " ", "Message", prop.RespStat.Message})
				table.Append([]string{" ", " ", "Passing Validation", strconv.FormatBool(prop.RespStat.PassingValidation)})
				table.Append([]string{" ", " ", "Propagation Status", prop.RespStat.PropagationStatus})
				table.Append([]string{" ", " ", "Propagation Status Date", prop.RespStat.PropagationStatusDate})
			}
		}
	} else {
		if len(succShortArray) == 0 {
			table.Append([]string{" ", "No successful updates", " ", " "})
		} else {
			for _, prop := range succShortArray {
				table.Append([]string{" ", prop.PropName, "ChangeId", prop.ChangeId})
			}
		}
	}

	// Failed Updates
	table.Append([]string{"Failed Updates", " ", " ", " "})
	if len(failedArray) == 0 {
		table.Append([]string{" ", "No failed property updates", " ", " "})
	} else {
		for _, prop := range failedArray {
			table.Append([]string{" ", prop.PropName, "Failure Message", prop.FailMsg})
		}
	}

	table.Render()
	outString.WriteString(tableString.String())
	return outString.String()
}
