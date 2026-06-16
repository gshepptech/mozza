package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanRepo_MissingURL(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "scan-empty@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import/scan", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestScanRepo_InvalidURL(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "scan-invalid@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{"url": "not-a-github-url"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import/scan", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

func TestDeployImport_MissingFields(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "import-bad@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{"team_id": "t1"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import/deploy", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDeployImport_InvalidTarget(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "import-target@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{
		"team_id": "t1",
		"target":  "invalid",
		"source":  "App: test\n\nApi:\n  from image test:latest\n  on port 8080\n",
	})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/import/deploy", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestScanRepo_Unauthenticated(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"url": "https://github.com/owner/repo"})
	resp, err := http.Post(ts.URL+"/api/v1/import/scan", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
