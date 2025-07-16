package reportsgtm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cli-gtm/edgegrid"
)

type WindowResponse struct {
	StartTime time.Time
	EndTime   time.Time
}

type APIWindowResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Converts RFC3339-formatted string to Go time.Time
func convertRFC3339toDate(rfc3339Stamp string) (time.Time, error) {
	return time.Parse(time.RFC3339, rfc3339Stamp)
}

// Converts API response to a structured WindowResponse object
func createTimeWindow(apiResponse *APIWindowResponse) (*WindowResponse, error) {
	startTime, err := convertRFC3339toDate(apiResponse.Start)
	if err != nil {
		return nil, err
	}
	endTime, err := convertRFC3339toDate(apiResponse.End)
	if err != nil {
		return nil, err
	}
	return &WindowResponse{
		StartTime: startTime,
		EndTime:   endTime,
	}, nil
}

// Reads and decodes a JSON response body into the provided struct
func readJSONBody(res *http.Response, out interface{}) error {
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	return decoder.Decode(out)
}

type APIError struct {
	StatusCode int
	Status     string
	Body       string
}

// Creates a new APIError from an HTTP response.
func NewAPIError(res *http.Response) error {
	bodyBytes, _ := io.ReadAll(res.Body)
	return &APIError{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		Body:       string(bodyBytes),
	}
}

// Formats the APIError into a readable string
func (e *APIError) Error() string {
	return fmt.Sprintf("API Error %d: %s - %s", e.StatusCode, e.Status, e.Body)
}

type CommonError struct {
	items map[string]string
}

// Adds a key-value pair to a CommonError
func (e *CommonError) SetItem(key, val string) {
	if e.items == nil {
		e.items = make(map[string]string)
	}
	e.items[key] = val
}

// Converts CommonError to a printable error string
func (e *CommonError) Error() string {
	return fmt.Sprintf("CommonError: %v", e.items)
}

// Central logic for calling a GTM report window API
func getWindowCore(ctx context.Context, sess edgegrid.Session, hostURL string) (*WindowResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, hostURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := sess.Exec(req, nil)
	if err != nil {
		return nil, fmt.Errorf("request execution failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		switch res.StatusCode {
		case 400:
			var errBody map[string]interface{}
			if err := readJSONBody(res, &errBody); err != nil {
				return nil, err
			}

			stat := &APIWindowResponse{}

			if availEnd, ok := errBody["availableEndDate"]; ok {
				if endStr, ok := availEnd.(string); ok {
					stat.End = endStr
				}
			}
			if availStart, ok := errBody["availableStartDate"]; ok {
				if startStr, ok := availStart.(string); ok {
					stat.Start = startStr
				}
			}

			if stat.End == "" || stat.Start == "" {
				cErr := &CommonError{}
				cErr.SetItem("entityName", "Window")
				cErr.SetItem("name", "Data Window")
				cErr.SetItem("apiErrorMessage", "No available data window")
				return nil, cErr
			}

			return createTimeWindow(stat)

		case 404:
			cErr := &CommonError{}
			cErr.SetItem("entityName", "Window")
			cErr.SetItem("name", "Data Window")
			return nil, cErr

		default:
			return nil, NewAPIError(res)
		}
	}

	stat := &APIWindowResponse{}
	if err := readJSONBody(res, stat); err != nil {
		return nil, err
	}

	return createTimeWindow(stat)
}

// Public API functions:

// Gets the demand report window for a specific domain and property
func GetDemandWindow(ctx context.Context, sess edgegrid.Session, domainName, propertyName string) (*WindowResponse, error) {
	url := fmt.Sprintf("/gtm-api/v1/reports/demand/domains/%s/properties/%s/window", domainName, propertyName)
	return getWindowCore(ctx, sess, url)
}

// Gets latency report window for a domain
func GetLatencyDomainsWindow(ctx context.Context, sess edgegrid.Session, domainName string) (*WindowResponse, error) {
	url := fmt.Sprintf("/gtm-api/v1/reports/latency/domains/%s/window", domainName)
	return getWindowCore(ctx, sess, url)
}

// Retrieves the available report window for liveness tests
func GetLivenessTestsWindow(ctx context.Context, sess edgegrid.Session) (*WindowResponse, error) {
	url := "/gtm-api/v1/reports/liveness-tests/window"
	return getWindowCore(ctx, sess, url)
}

// Gets traffic report window for all datacenters
func GetDatacentersTrafficWindow(ctx context.Context, sess edgegrid.Session) (*WindowResponse, error) {
	url := "/gtm-api/v1/reports/traffic/datacenters-window"
	return getWindowCore(ctx, sess, url)
}

// Gets traffic report window for all properties
func GetPropertiesTrafficWindow(ctx context.Context, sess edgegrid.Session) (*WindowResponse, error) {
	url := "/gtm-api/v1/reports/traffic/properties-window"
	return getWindowCore(ctx, sess, url)
}
