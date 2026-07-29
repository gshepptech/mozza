package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/auth"
	"github.com/gshepptech/mozza/internal/compile"
	"github.com/gshepptech/mozza/internal/local"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/server"
	"github.com/gshepptech/mozza/internal/store"
)

// validRecipe is a minimal recipe that parses, builds, and validates.
const validRecipe = `App: compiletest

Api:
  from image nginx:alpine
  open to the public on port 8080
  health check /healthz
  run 1 copy
`

// targetsBody mirrors the JSON returned by GET /api/v1/targets.
type targetsBody struct {
	Targets []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Default     bool   `json:"default"`
	} `json:"targets"`
	Default string `json:"default"`
}

// compileBody mirrors the JSON returned by POST /api/v1/compile.
type compileBody struct {
	Target  string `json:"target"`
	Summary string `json:"summary"`
	Error   string `json:"error"`
	Files   []struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	} `json:"files"`
	Warnings []string `json:"warnings"`
}

// testServerWithTargets builds an authenticated server backed by the given
// compile registry. A nil registry is passed through untouched so callers can
// exercise the Config.Targets nil fallback.
func testServerWithTargets(t *testing.T, reg *compile.Registry) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	srv := server.New(server.Config{
		Plan:    &plan.AppPlan{Name: "testapp"},
		Store:   st,
		Auth:    auth.New(st),
		Targets: reg,
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return ts, st
}

// postCompile sends a compile request and decodes the response.
func postCompile(t *testing.T, ts *httptest.Server, cookie *http.Cookie, payload map[string]string) (int, compileBody) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/compile", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var decoded compileBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))

	return resp.StatusCode, decoded
}

func TestTargetsEndpoint(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTargets(t, nil)

	cookie := registerUser(t, ts, "targets-list@example.com", "Test", "password123")

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/targets", nil)
	require.NoError(t, err)
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body targetsBody
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, "kubernetes", body.Default)
	require.Len(t, body.Targets, 2, "built-in registry should expose kubernetes and local")

	// targets.List sorts by name, so the order here is part of the contract.
	assert.Equal(t, "kubernetes", body.Targets[0].Name)
	assert.True(t, body.Targets[0].Default)
	assert.NotEmpty(t, body.Targets[0].Description)

	assert.Equal(t, "local", body.Targets[1].Name)
	assert.False(t, body.Targets[1].Default)
	assert.NotEmpty(t, body.Targets[1].Description)
}

func TestTargetsEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTargets(t, nil)

	resp, err := http.Get(ts.URL + "/api/v1/targets")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestCompile_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       string
		wantTarget   string
		wantFile     string
		wantContains string
	}{
		{
			name:         "target omitted defaults to kubernetes",
			target:       "",
			wantTarget:   "kubernetes",
			wantFile:     "k8s/api-deployment.yaml",
			wantContains: "kind: Deployment",
		},
		{
			name:         "explicit kubernetes target",
			target:       "kubernetes",
			wantTarget:   "kubernetes",
			wantFile:     "k8s/api-deployment.yaml",
			wantContains: "kind: Deployment",
		},
		{
			name:         "explicit local target",
			target:       "local",
			wantTarget:   "local",
			wantFile:     "docker-compose.yml",
			wantContains: "nginx:alpine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts, _ := testServerWithTargets(t, nil)
			cookie := registerUser(t, ts, "compile-ok-"+tt.wantTarget+tt.target+"@example.com", "Test", "password123")

			status, body := postCompile(t, ts, cookie, map[string]string{
				"source": validRecipe,
				"target": tt.target,
			})

			require.Equal(t, http.StatusOK, status, "unexpected error: %s", body.Error)
			assert.Equal(t, tt.wantTarget, body.Target)
			assert.NotEmpty(t, body.Summary)
			assert.NotNil(t, body.Warnings, "warnings must be a list, never null")

			paths := make([]string, len(body.Files))
			var content string
			for i, f := range body.Files {
				paths[i] = f.Path
				if f.Path == tt.wantFile {
					content = f.Content
				}
			}
			require.Contains(t, paths, tt.wantFile, "compiled files: %v", paths)
			assert.Contains(t, content, tt.wantContains)
		})
	}
}

func TestCompile_BadRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		payload      map[string]string
		wantContains []string
	}{
		{
			name:         "unknown target lists the valid ones",
			payload:      map[string]string{"source": validRecipe, "target": "helm"},
			wantContains: []string{`unknown target "helm"`, "kubernetes", "local"},
		},
		{
			name:         "recipe that does not parse",
			payload:      map[string]string{"source": "this is not a recipe at all"},
			wantContains: []string{"recipe parse failed"},
		},
		{
			name:         "empty source",
			payload:      map[string]string{"source": "   \n"},
			wantContains: []string{"source is required"},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ts, _ := testServerWithTargets(t, nil)
			cookie := registerUser(t, ts, "compile-bad-"+strconv.Itoa(i)+"@example.com", "Test", "password123")

			status, body := postCompile(t, ts, cookie, tt.payload)

			require.Equal(t, http.StatusBadRequest, status)
			for _, want := range tt.wantContains {
				assert.Contains(t, body.Error, want)
			}
		})
	}
}

func TestCompile_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTargets(t, nil)

	status, _ := postCompile(t, ts, nil, map[string]string{"source": validRecipe})

	assert.Equal(t, http.StatusUnauthorized, status)
}

// TestCompile_AllowsViewer pins the route-placement decision: /compile mutates
// nothing, so it sits with the read-only endpoints rather than behind
// requireRole("deployer","admin"). The first registered user is promoted to
// admin, so the second one is the viewer under test.
func TestCompile_AllowsViewer(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTargets(t, nil)

	registerUser(t, ts, "compile-admin@example.com", "Admin", "password123")
	viewer := registerUser(t, ts, "compile-viewer@example.com", "Viewer", "password123")

	// Confirm the second user really is a viewer before relying on the result.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	require.NoError(t, err)
	req.AddCookie(viewer)
	meResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer meResp.Body.Close()
	var me struct {
		Role string `json:"role"`
	}
	require.NoError(t, json.NewDecoder(meResp.Body).Decode(&me))
	require.Equal(t, "viewer", me.Role)

	status, body := postCompile(t, ts, viewer, map[string]string{"source": validRecipe})
	require.Equal(t, http.StatusOK, status, "unexpected error: %s", body.Error)
	assert.NotEmpty(t, body.Files)
}

// TestCompile_NilTargetsFallsBack asserts the load-bearing default: a Config
// with no Targets still compiles, so existing callers need no wiring.
func TestCompile_NilTargetsFallsBack(t *testing.T) {
	t.Parallel()
	ts, _ := testServerWithTargets(t, nil)
	cookie := registerUser(t, ts, "compile-nilreg@example.com", "Test", "password123")

	status, body := postCompile(t, ts, cookie, map[string]string{"source": validRecipe, "target": "local"})

	require.Equal(t, http.StatusOK, status, "unexpected error: %s", body.Error)
	require.Len(t, body.Files, 1)
	assert.Equal(t, "docker-compose.yml", body.Files[0].Path)
}

// TestCompile_HonoursConfiguredRegistry proves the fallback does not shadow an
// explicitly configured registry: a registry holding only "local" must reject
// the kubernetes target.
func TestCompile_HonoursConfiguredRegistry(t *testing.T) {
	t.Parallel()
	reg := compile.NewRegistry()
	reg.Register("local", local.New())

	ts, _ := testServerWithTargets(t, reg)
	cookie := registerUser(t, ts, "compile-customreg@example.com", "Test", "password123")

	status, body := postCompile(t, ts, cookie, map[string]string{"source": validRecipe, "target": "kubernetes"})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, body.Error, `unknown target "kubernetes"`)
	assert.Contains(t, body.Error, "valid targets: local")
}

// TestCompile_WritesNothingToStore is the side-effect-free guarantee: compiling
// must not create deployment, deploy, or recipe rows.
func TestCompile_WritesNothingToStore(t *testing.T) {
	t.Parallel()
	ts, st := testServerWithTargets(t, nil)
	cookie := registerUser(t, ts, "compile-noside@example.com", "Test", "password123")

	before := storeRowCounts(t, st)

	for _, target := range []string{"", "kubernetes", "local"} {
		status, body := postCompile(t, ts, cookie, map[string]string{"source": validRecipe, "target": target})
		require.Equal(t, http.StatusOK, status, "unexpected error: %s", body.Error)
	}

	assert.Equal(t, before, storeRowCounts(t, st), "compile must not write to the store")
}

// storeRowCounts returns the row count of every table in the store, keyed by
// table name.
func storeRowCounts(t *testing.T, st *store.Store) map[string]int {
	t.Helper()

	rows, err := st.DB().Query(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`)
	require.NoError(t, err)

	var tables []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		tables = append(tables, name)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	require.NotEmpty(t, tables, "migrations should have created tables")

	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		var n int
		// Table names come from sqlite_master, not from request input.
		require.NoError(t, st.DB().QueryRow("SELECT COUNT(*) FROM "+table).Scan(&n))
		counts[table] = n
	}

	return counts
}
