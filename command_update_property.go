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
        "os"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/fatih/color"
	//"github.com/olekukonko/tablewriter"
	"github.com/urfave/cli"
)

func cmdUpdateProperty(c *cli.Context) error {

        var dcWeight float64
        var dcServers []string
        var dcEnabled bool
        var dcDatacenters *arrayFlags
        var verboseStatus bool

	config, err := akamai.GetEdgegridConfig(c)
	if err != nil {
		return err
	}

       configgtm.Config = config

        fmt.Println("OS Args: %#v", os.Args) 

	if c.NArg() < 2 {
		cli.ShowCommandHelp(c, c.Command.Name)
		return cli.NewExitError(color.RedString("domain and property are required"), 1)
	}

	domainname := c.Args().Get(0) 
        propertyname := c.Args().Get(1)

        // Changes may be to enabled, weight or servers
        dcWeight = c.Float64("weight")
        dcServers = c.StringSlice("servers")
        dcEnabled = c.BoolT("enabled")
        dcDatacenters = (c.Generic("datacenter")).(*arrayFlags)
        verboseStatus = c.Bool("verbose")

        if !c.IsSet("datacenter") {
                return cli.NewExitError(color.RedString("datacenter(s) must be specified"), 1)
        }
        if c.IsSet("servers") && len(dcDatacenters.flagList) > 1 {
                return cli.NewExitError(color.RedString("servers update may only apply to one datacenter"), 1)
        }
        if c.IsSet("weight") && len(dcDatacenters.flagList) > 1 {
                return cli.NewExitError(color.RedString("weight update may only apply to one datacenter"), 1)
        }

        // Debug
        fmt.Println("Args: ", strings.Join(c.Args(), ", "))
        fmt.Println("domainname: ", domainname)
        fmt.Println("propertyname: ", propertyname)
        fmt.Println("datacenters: ", dcDatacenters.String())
        fmt.Println("enabled: ", strconv.FormatBool(dcEnabled))
        fmt.Println("weight: ", dcWeight)
        fmt.Println("servers: ", dcServers)

        akamai.StartSpinner(
                "Updating property ...",
                fmt.Sprintf("Fetching " + propertyname + " ...... [%s]", color.GreenString("OK")),
        )

        property, err := configgtm.GetProperty(domainname, propertyname)
        if err != nil {
                akamai.StopSpinnerFail()
                return cli.NewExitError(color.RedString("Property not found"), 1)
        }

        changes_made := false

        fmt.Println("Property: "+property.Name)
        trafficTargets := property.TrafficTargets
        targetsmsg := property.Name + " contains " + strconv.Itoa(len(trafficTargets)) + " targets"
        fmt.Println(targetsmsg)
        fmt.Sprintf(targetsmsg)
        for _, traffTarg := range trafficTargets {
               for _, dcid := range dcDatacenters.flagList {
                        if traffTarg.DatacenterId == dcid {
                                fmt.Sprintf(traffTarg.Name + " contains dc " + strconv.Itoa(dcid))
                                if c.IsSet("enabled") && traffTarg.Enabled != dcEnabled {
                                        traffTarg.Enabled = dcEnabled
                                        changes_made = true
                                }
                                if c.IsSet("weight") && traffTarg.Weight != dcWeight {
                                        // Note: weight will be ignored for a number of property types
                                        traffTarg.Weight = dcWeight
                                        changes_made = true 
                                }       
                                if c.IsSet("servers") {
                                        for i, v := range traffTarg.Servers {
                                                if v != dcServers[i] {
                                                        traffTarg.Servers = dcServers
                                                        changes_made = true
                                                }
                                        }
                                }
                        }       
               }
        }

        if changes_made {
                propstat, err := property.Update(domainname)
                if err != nil {
                        akamai.StopSpinnerFail()
                        return cli.NewExitError(color.RedString("Error updating property "+propertyname+". "+err.Error()), 1)
                }
                fmt.Fprintln(c.App.Writer,  "Property " + propertyname + " updated")

                var status interface{}

                if c.IsSet("verbose") && verboseStatus {
                        status = propstat
                } else {
                        status = &propstat.ChangeId
                }

                json, err := json.MarshalIndent(status, "", "  ")
                if err != nil {
                        akamai.StopSpinnerFail()
                        return cli.NewExitError(color.RedString("Unable to display property update status"), 1)
                }
                fmt.Fprintln(c.App.Writer, string(json))
        }

        akamai.StopSpinnerOk()

        return nil

}
