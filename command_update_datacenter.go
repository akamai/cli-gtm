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
  
        "github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1"
        akamai "github.com/akamai/cli-common-golang"

        "github.com/fatih/color"
        //"github.com/olekukonko/tablewriter"
        "github.com/urfave/cli" 
)

var dcEnabled bool
var dcDatacenters *arrayFlags
var dcNicknames []string

var succShortArray []*SuccUpdateShort
var succVerboseArray []*SuccUpdateVerbose
var failedArray []*FailUpdate

func cmdUpdateDatacenter(c *cli.Context) error {

        config, err := akamai.GetEdgegridConfig(c)
        if err != nil {
                return err
        }

        configgtm.Init(config)

        if c.NArg() == 0 {
                cli.ShowCommandHelp(c, c.Command.Name)
                return cli.NewExitError(color.RedString("domainname is required"), 1)
        }

        domainname := c.Args().First()
        dcDatacenters = c.Generic("datacenterid").(*arrayFlags)
        dcEnabled = c.BoolT("enabled")
        verboseStatus = c.Bool("verbose")
        dcNicknames = c.StringSlice("dcnickname")

        if !c.IsSet("datacenterid") || len(dcDatacenters.flagList) == 0 {
                cli.ShowCommandHelp(c, c.Command.Name)
                return cli.NewExitError(color.RedString("One or more datacenters is required"), 1)
        }
        if !c.IsSet("enabled") {
                cli.ShowCommandHelp(c, c.Command.Name)
                return cli.NewExitError(color.RedString("new enabled state is required"), 1)
        }
        // if nicknames specified, add to dcFlags 
        if c.IsSet("dcnickname") {
                ParseNicknames(dcNicknames, domainname)
        } 
        akamai.StartSpinner(
                "Fetching data...",
                fmt.Sprintf("Fetching domain properties ...... [%s]", color.GreenString("OK")),
        )

        // get domain. Serves two purposes. Validates domain exists and retrieves all the properties
        fmt.Println("Domain: ", domainname)
        dom, err := configgtm.GetDomain(domainname)

        if err != nil {
                akamai.StopSpinnerFail()
                return cli.NewExitError(color.RedString("Domain " + domainname + " not found "), 1)
        }

        properties := dom.Properties
        propmsg := domainname + " contains " + strconv.Itoa(len(properties)) + " properties"
        fmt.Sprintf(propmsg)
        fmt.Println(propmsg)
        for _, propPtr := range properties {
                changes_made := false
                fmt.Println("Property: "+propPtr.Name)
                trafficTargets := propPtr.TrafficTargets
                targetsmsg := propPtr.Name + " contains " + strconv.Itoa(len(trafficTargets)) + " targets"
                fmt.Println(targetsmsg)
                fmt.Sprintf(targetsmsg)
                for _, traffTarg := range trafficTargets {
                        dcs := dcDatacenters
                        for _, dcid := range dcs.flagList {
                                if traffTarg.DatacenterId == dcid && c.IsSet("enabled") && traffTarg.Enabled != dcEnabled {
                                        fmt.Sprintf(traffTarg.Name + " contains dc " + strconv.Itoa(dcid))
                                        traffTarg.Enabled = dcEnabled
                                        changes_made = true
                                }
                        }
                }
                if changes_made {
                        stat, err := propPtr.Update(domainname)
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

        if len(properties) == 1 && len(failedArray) > 0 {
                akamai.StopSpinnerFail()
                return cli.NewExitError(color.RedString("Error updating property "+failedArray[0].PropName+": "+failedArray[0].FailMsg), 1)
        }

        akamai.StopSpinnerOk()

        updateSum := UpdateSummary{}
        if c.IsSet("verbose") && verboseStatus && len(succVerboseArray) > 0 {
                updateSum.Updated_Properties = succVerboseArray
        } else if len(succShortArray) > 0 {
                updateSum.Updated_Properties = succShortArray
        }
        if len(failedArray) > 0 {        
                updateSum.Failed_Updates = failedArray
        }
       
        json, err := json.MarshalIndent(updateSum, "", "  ")
        if err != nil {
                return cli.NewExitError(color.RedString("Unable to display property update status"), 1)
        }
        if updateSum.Failed_Updates == nil && updateSum.Updated_Properties == nil {
                fmt.Fprintln(c.App.Writer, "No property updates were needed.")
        } else {  
                fmt.Fprintln(c.App.Writer, string(json))
        }

        return nil 

}
