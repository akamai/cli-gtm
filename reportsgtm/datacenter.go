package reportsgtm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"cli-gtm/edgegrid"

	"github.com/urfave/cli"
)

type DCTMeta struct {
	Uri                string `json:"uri"`
	Domain             string `json:"domain"`
	Interval           string `json:"interval,omitempty"`
	DatacenterId       int    `json:"datacenterId"`
	DatacenterNickname string `json:"datacenterNickname"`
	Start              string `json:"start"`
	End                string `json:"end"`
}

type DCTDRow struct {
	Name     string `json:"name"`
	Requests int64  `json:"requests"`
	Status   string `json:"status"`
}

type DCTData struct {
	Timestamp  string     `json:"timestamp"`
	Properties []*DCTDRow `json:"properties"`
}

type DcTrafficResponse struct {
	Metadata    *DCTMeta    `json:"metadata"`
	DataRows    []*DCTData  `json:"dataRows"`
	DataSummary interface{} `json:"dataSummary"`
	Links       []*Link     `json:"links"`
}

type Link struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

func GetTrafficPerDatacenter(c *cli.Context, domain string, datacenterID int, optArgs map[string]string) (*DcTrafficResponse, error) {

	// Initialize Edgegrid session
	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("session initialization failed: %v", err)
	}

	// Build URL path with query params
	basePath := fmt.Sprintf("/gtm-api/v1/reports/traffic/domains/%s/datacenters/%d", domain, datacenterID)
	params := url.Values{}
	for k, v := range optArgs {
		params.Set(k, v)
	}
	if len(params) > 0 {
		basePath += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, basePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	/// Use sess.Exec() to sign and execute the request
	// Passing 'nil' as 'in' because this is a GET with no request body
	// The response body will be decoded into the result struct
	var result DcTrafficResponse
	resp, err := sess.Exec(req, &result)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 HTTP status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return &result, nil
}
