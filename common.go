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
	"github.com/akamai/AkamaiOPEN-edgegrid-golang/configgtm-v1_3"
	"strconv"
)

// SuccUpdateShort is the success status structure for no verbose status updates
type SuccUpdateShort struct {
	PropName string
	ChangeId string
}

// SuccUpdateVerbose is the success status structure for verbose status updates
type SuccUpdateVerbose struct {
	PropName string
	RespStat *configgtm.ResponseStatus
}

// FailUpdate is the failure status structure for no verbose status updates
type FailUpdate struct {
	PropName string
	FailMsg  string
}

// UpdateSummary is the result summary status structure 
type UpdateSummary struct {
	Updated_Properties interface{}
	Failed_Updates     []*FailUpdate
}

var verboseStatus bool = false

// ParseNicknames parses any nicknames provided and adds to dcFlags
func ParseNicknames(nicknames []string, domain string) {

	// get list of data centers
	dcList, _ := configgtm.ListDatacenters(domain)
	// walk thru datacenters and nicknames
	for _, dc := range dcList {
		for _, nn := range nicknames {
			if dc.Nickname == nn {
				dcFlags.Set(strconv.Itoa(dc.DatacenterId))
			}
		}
	}

}
