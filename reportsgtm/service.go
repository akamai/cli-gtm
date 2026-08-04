package reportsgtm

import (
	"context"
	"net/http"

	"cli-gtm/edgegrid"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/v13/pkg/log"
)

// InitLogging enables default logging if needed.
func InitLogging() {
	log.SetLogger(log.Default())
}

// Utility to log request if tracing is needed (manually log URI/method, etc.)
func logHttpRequest(ctx context.Context, req *http.Request) {
	logger := edgegrid.GetSession(ctx).Log(ctx)
	logger.Debugf("Request: %s %s", req.Method, req.URL.String())
}

// Utility to log response status
func logHttpResponse(ctx context.Context, res *http.Response) {
	logger := edgegrid.GetSession(ctx).Log(ctx)
	logger.Debugf("Response Status: %s", res.Status)
}
