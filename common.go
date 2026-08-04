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
	"fmt"
	"strconv"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg/gtm"
	"github.com/urfave/cli"
)

// SuccUpdateShort is the success status structure for no verbose status updates
type SuccUpdateShort struct {
	PropName string
	ChangeId string
}

// SuccUpdateVerbose is the success status structure for verbose status updates
type SuccUpdateVerbose struct {
	PropName string
	RespStat *gtm.ResponseStatus
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
func ParseNicknames(c *cli.Context, nicknames []string, domain string) error {

	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return fmt.Errorf("session failed %v", err)
	}
	ctx = edgegrid.WithSession(ctx, sess)
	gtmClient := gtm.Client(edgegrid.GetSession(ctx))

	if len(nicknames) > 0 {
		// get list of data centers
		req := gtm.ListDatacentersRequest{DomainName: domain}

		dcList, err := gtmClient.ListDatacenters(ctx, req)
		if err != nil {
			return err
		}
		// walk thru datacenters and nicknames
		for _, dc := range dcList {
			for _, nn := range nicknames {
				if dc.Nickname == nn {
					dcFlags.Set(strconv.Itoa(dc.DatacenterID))
				}
			}
		}
	}
	return nil
}
