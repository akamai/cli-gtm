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

//
// Support GTM reports thru Edgegrid (v11)
//

// Property Traffic Report Structs
type PropertyTMeta struct {
	Uri      string `json:"uri"`
	Domain   string `json:"domain"`
	Interval string `json:"interval,omitempty"`
	Property string `json:"property"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

type PropertyDRow struct {
	Nickname          string `json:"nickname"`
	DatacenterId      int    `json:"datacenterId"`
	TrafficTargetName string `json:"trafficTargetName"`
	Requests          int64  `json:"requests"`
	Status            string `json:"status"`
}

type PropertyTData struct {
	Timestamp   string          `json:"timestamp"`
	Datacenters []*PropertyDRow `json:"datacenters"`
}

// Property Traffic Response structure
type PropertyTrafficResponse struct {
	Metadata    *PropertyTMeta   `json:"metadata"`
	DataRows    []*PropertyTData `json:"dataRows"`
	DataSummary interface{}      `json:"dataSummary"`
	Links       []*Link          `json:"links"`
}

// IP Status structs
type IPStatusPerProperty struct {
	Metadata    *IpStatPerPropMeta   `json:"metadata"`
	DataRows    []*IpStatPerPropData `json:"dataRows"`
	DataSummary interface{}          `json:"dataSummary"`
	Links       []*Link              `json:"links"`
}

type IpStatPerPropMeta struct {
	Uri          string `json:"uri"`
	Domain       string `json:"domain"`
	Property     string `json:"property"`
	Start        string `json:"start"`
	End          string `json:"end"`
	MostRecent   bool   `json:"mostRecent"`
	Ip           string `json:"ip"`
	DatacenterId int    `json:"datacenterId"`
}

type IpStatPerPropData struct {
	Timestamp   string               `json:"timestamp"`
	CutOff      float64              `json:"cutOff"`
	Datacenters []*IpStatPerPropDRow `json:"datacenters"`
}

type IpStatPerPropDRow struct {
	Nickname          string      `json:"nickname"`
	DatacenterId      int         `json:"datacenterId"`
	TrafficTargetName string      `json:"trafficTargetName"`
	IPs               []*IpStatIp `json:"IPs"`
}

type IpStatIp struct {
	Ip        string  `json:"ip"`
	HandedOut bool    `json:"handedOut"`
	Score     float32 `json:"score"`
	Alive     bool    `json:"alive"`
}

// Generic helper function to make GET requests and parse the JSON response into a specific type T
func doReportRequest[T any](c *cli.Context, path string, optArgs map[string]string) (*T, error) {

	// Initialize Edgegrid session
	ctx := context.Background()
	sess, err := edgegrid.InitializeSession(c)
	if err != nil {
		return nil, fmt.Errorf("session initialization failed: %w", err)
	}

	// Build URL with query params
	fullURL := path
	if len(optArgs) > 0 {
		q := url.Values{}
		for k, v := range optArgs {
			q.Set(k, v)
		}
		fullURL += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request failed: %w", err)
	}

	var result T
	resp, err := sess.Exec(req, &result)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return &result, nil
}

// Constructs the endpoint and calls doReportRequest to get IP availability per property
func GetIpStatusPerProperty(c *cli.Context, domain, prop string, optArgs map[string]string) (*IPStatusPerProperty, error) {
	path := fmt.Sprintf("/gtm-api/v1/reports/ip-availability/domains/%s/properties/%s", domain, prop)
	return doReportRequest[IPStatusPerProperty](c, path, optArgs)
}

// Constructs the endpoint and calls doReportRequest to get traffic data for a specific property
func GetTrafficPerProperty(c *cli.Context, domain, prop string, optArgs map[string]string) (*PropertyTrafficResponse, error) {
	path := fmt.Sprintf("/gtm-api/v1/reports/traffic/domains/%s/properties/%s", domain, prop)
	return doReportRequest[PropertyTrafficResponse](c, path, optArgs)
}
