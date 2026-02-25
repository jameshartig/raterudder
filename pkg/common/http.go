package common

import (
	_ "embed"
	"net/http"
	"strings"
	"time"
)

//go:embed VERSION
var version string

type userAgentTransport struct {
	transport http.RoundTripper
	userAgent string
}

// RoundTrip implements the
func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original request's headers
	// which might be shared or reused
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.userAgent)
	return t.transport.RoundTrip(req)
}

// HTTPClient returns a default http client with a default user-agent set
func HTTPClient(timeout time.Duration) *http.Client {
	v := strings.TrimSpace(version)
	userAgent := "RateRudder/" + v

	return &http.Client{
		Transport: &userAgentTransport{
			transport: http.DefaultTransport,
			userAgent: userAgent,
		},
		Timeout: timeout,
	}
}
