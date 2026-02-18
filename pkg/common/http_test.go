package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent header
		userAgent := r.Header.Get("User-Agent")
		assert.Equal(t, "RateRudder/"+strings.TrimSpace(version), userAgent, "User-Agent should match expected format")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Test client creation
	timeout := 5 * time.Second
	client := HTTPClient(timeout)

	// Verify client settings
	assert.Equal(t, timeout, client.Timeout, "Timeout should be set correctly")
	assert.NotNil(t, client.Transport, "Transport should not be nil")

	// Test actual request
	req, err := http.NewRequest("GET", server.URL, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
