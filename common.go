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
        "github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1"
        "strconv"
        "github.com/urfave/cli"
)

type SuccUpdateShort struct {
        PropName string
        ChangeId string
}

type SuccUpdateVerbose struct {
        PropName string
        RespStat *configgtm.ResponseStatus
}

type FailUpdate struct {
        PropName string
        FailMsg  string
}

type UpdateSummary struct {
        Updated_Properties    interface{}
        Failed_Updates        []*FailUpdate
}

var verboseStatus bool

// parse any nicknames provided and add to dcFlags
func ParseNicknames(nicknames cli.StringSlice, domainname string) {

        // get list of data centers
        dcList, _ := configgtm.ListDatacenters(domainname)
        // walk thru datacenters and nicknames
        for _, dc := range dcList {
                for _, nn := range nicknames {
                        if dc.Nickname == nn {
                                dcFlags.Set(strconv.Itoa(dc.DatacenterId))
                        }
                }     
        }

}

