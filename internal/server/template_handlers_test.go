package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/server"
	"github.com/gshepptech/mozza/internal/store"
	"github.com/gshepptech/mozza/internal/template"
)

// testServerWithTemplates creates a test server with a template catalog.
func testServerWithTemplates(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	svc := auth.New(st)

	// Try to load the real catalog; skip template tests if not available.
	catalog, err := template.Load()
	if err != nil {
		t.Skipf("template catalog not available: %v", err)
	}

	cfg := server.Config{
		Plan: &plan.AppPlan{
			Name: "testapp",
			Slices: []plan.Slice{
				{Name: "api", Kind: plan.SliceKindWeb, Image: "api:latest", Port: 8080},
			},
		},
		Store:     st,
		Auth:      svc,
		Templates: catalog,
	}
	srv := server.New(cfg)
	return httptest.NewServer(srv.Handler()), st
}

func TestListTemplates_ReturnsOK(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTemplates(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "tmpl-list@example.com", "Test", "password123")

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/templates", nil)
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Templates []json.RawMessage `json:"templates"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEmpty(t, body.Templates)
}

func TestListTemplates_WithCategoryFilter(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTemplates(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "tmpl-cat@example.com", "Test", "password123")

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/templates?category=nonexistent", nil)
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Templates []json.RawMessage `json:"templates"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	// Nonexistent category should return empty list (not null).
	assert.Empty(t, body.Templates)
}

func TestGetTemplate_NotFound(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTemplates(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "tmpl-404@example.com", "Test", "password123")

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/templates/does-not-exist", nil)
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeployTemplate_MissingFields(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTemplates(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "tmpl-deploy-bad@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/templates/some-id/deploy", bytes.NewReader(body))
	req.AddCookie(cookie)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Could be 404 (template not found) or 400 (missing fields) depending on catalog state.
	assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, resp.StatusCode)
}

func TestListTemplates_Unauthenticated(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTemplates(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/templates")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
