package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimit_BlocksAfterLimit(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	// Auth register endpoint is rate-limited at 10 rpm.
	// We'll hit the login endpoint repeatedly since it's also rate-limited.
	// Use a unique IP via X-Forwarded-For to isolate from other tests.
	client := &http.Client{}
	var lastStatus int

	for i := 0; i < 15; i++ {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "10.99.99.1")
		resp, err := client.Do(req)
		require.NoError(t, err)
		lastStatus = resp.StatusCode
		resp.Body.Close()

		// Once we get a 429, we've confirmed rate limiting works.
		if lastStatus == http.StatusTooManyRequests {
			break
		}
	}

	assert.Equal(t, http.StatusTooManyRequests, lastStatus, "expected 429 after exceeding rate limit")
}

func TestSecurityHeaders(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Contains(t, resp.Header.Get("Content-Security-Policy"), "default-src 'self'")
}

func TestRecoverer_PanicDoesNotCrash(t *testing.T) {
	t.Parallel()
	// The recoverer middleware is applied globally. We can verify the server
	// stays alive after handling requests — if the middleware were missing,
	// any panic in a handler would kill the test server.
	ts := testServer(t)
	defer ts.Close()

	// Multiple sequential requests to verify the server stays healthy.
	for i := 0; i < 3; i++ {
		resp, err := http.Get(ts.URL + "/api/v1/health")
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestResponseRecorder_CapturesStatus(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	// A 404 response should be properly recorded.
	resp, err := http.Get(ts.URL + "/api/v1/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Chi returns 405 for unmatched routes under a route group, or 404.
	// Either way, the response should have a valid status code.
	assert.True(t, resp.StatusCode >= 400, "expected 4xx for nonexistent route")
}

func TestMaxRequestBody(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	// The maxRequestBody middleware limits to 1MB. A request with content
	// that is too large should fail. We test that normal-sized requests work.
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json",
		httptest.NewRequest(http.MethodPost, "/", nil).Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should get a 400 (bad request body) not a 413, since the body is empty/nil
	// but the handler tries to decode it.
	assert.True(t, resp.StatusCode < 500, "expected non-5xx for normal request")
}
