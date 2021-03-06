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
	"errors"
	akamai "github.com/akamai/cli-common-golang"
	"github.com/urfave/cli"
	"strconv"
	"strings"
)

type arrayFlags struct {
	flagList       []int
	flagStringList []string
	nicknamesList  []string
}

var dcFlags arrayFlags

func (i *arrayFlags) String() string {

	if len(i.flagStringList) == 0 {
		return ""
	}
	fString := strings.Join(i.flagStringList, ", ")
	return fString
}

func (i *arrayFlags) Get(indx int) int {

	var val int
	if indx < len(i.flagList) {
		val = i.flagList[indx]
	}
	// TODO: Add some definition to error ...
	return val
}

func (i *arrayFlags) Set(value string) error {

	for _, v := range i.flagStringList {
		if v == value {
			return nil
		}
	}
	// See if its an id vs nickname
	intVal, err := strconv.Atoi(value)
	if err != nil {
		// nickname. save for later processing (need domain to map)
		i.nicknamesList = append(i.nicknamesList, value)
		return nil
	}

	i.flagList = append(i.flagList, intVal)
	i.flagStringList = append(i.flagStringList, value)
	return nil
}

func parseBoolString(val string) (bool, error) {
	boolVal := strings.ToLower(val)
	if boolVal == "true" {
		return true, nil
	} else if boolVal == "false" {
		return false, nil
	}
	return true, errors.New("Invalid value provided. Acceptable values: true, false")
}

var commandLocator akamai.CommandLocator = func() ([]cli.Command, error) {
	var commands []cli.Command

	commands = append(commands, cli.Command{
		Name:        "update-datacenter",
		Description: "Update datacenter configuration",
		ArgsUsage:   "<domain>",
		Action:      cmdUpdateDatacenter,
		Flags: []cli.Flag{
			cli.GenericFlag{
				Name:  "datacenter",
				Usage: "Apply change to specified datacenter by id or nickname.",
				Value: &dcFlags,
			},
			cli.BoolTFlag{
				Name:  "enable",
				Usage: "Enable specified datacenter(s).",
			},
			cli.BoolFlag{
				Name:  "disable",
				Usage: "Disable specified datacenter(s).",
			},
			cli.BoolFlag{
				Name:  "verbose",
				Usage: "Display verbose result status.",
			},
			cli.BoolFlag{
				Name:  "json",
				Usage: "Return status in JSON format.",
			},
			cli.BoolFlag{
				Name:  "complete",
				Usage: "Wait up to 5 minutes for change completion.",
			},
		},
		BashComplete: akamai.DefaultAutoComplete,
	})

	commands = append(commands, cli.Command{
		Name:        "update-property",
		Description: "Update property configuration",
		ArgsUsage:   "[domain, property]",
		Action:      cmdUpdateProperty,
		Flags: []cli.Flag{
			cli.GenericFlag{
				Name:  "datacenter",
				Usage: "Apply change to specified datacenter by id or nickname.",
				Value: &dcFlags,
			},
			cli.BoolTFlag{
				Name:  "enable",
				Usage: "Enable specified datacenter(s).",
			},
			cli.BoolFlag{
				Name:  "disable",
				Usage: "Disable specified datacenter(s).",
			},
			cli.Float64Flag{
				Name:  "weight",
				Usage: "Apply 'weight' to specified datacenter.",
			},
			cli.StringSliceFlag{
				Name:  "server",
				Usage: "Update target server for specified datacenter. Multiple server flags may be specified.",
			},
			cli.BoolFlag{
				Name:  "verbose",
				Usage: "Display verbose result status.",
			},
			cli.BoolFlag{
				Name:  "json",
				Usage: "Return status in JSON format.",
			},
			cli.BoolFlag{
				Name:  "complete",
				Usage: "Wait up to 5 minutes for change completion",
			},
		},
		BashComplete: akamai.DefaultAutoComplete,
	})

	commands = append(commands, cli.Command{
		Name:        "query-status",
		Description: "Query current status of domain, property or datacenter",
		ArgsUsage:   "<domain>",
		Action:      cmdQueryStatus,
		Flags: []cli.Flag{
			cli.GenericFlag{
				Name:  "datacenter",
				Usage: "Report status of specified datacenter by id or nickname.",
				Value: &dcFlags,
			},
			cli.StringFlag{
				Name:  "property",
				Usage: "Report status of specified property.",
			},
			cli.BoolFlag{
				Name:  "verbose",
				Usage: "Display verbose status.",
			},
			cli.BoolFlag{
				Name:  "json",
				Usage: "Return status in JSON format.",
			},
		},
		BashComplete: akamai.DefaultAutoComplete,
	})

	commands = append(commands,
		cli.Command{
			Name:        "list",
			Description: "List commands",
			Action:      akamai.CmdList,
		},
		cli.Command{
			Name:         "help",
			Description:  "Displays help information",
			ArgsUsage:    "[command] [sub-command]",
			Action:       akamai.CmdHelp,
			BashComplete: akamai.DefaultAutoComplete,
		},
	)

	return commands, nil
}
