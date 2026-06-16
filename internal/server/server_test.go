package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
	"github.com/gshepptech/mozza/internal/server"
)

// testServer creates an httptest.Server backed by a Server with sample plan data.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := server.Config{
		Plan: &plan.AppPlan{
			Name: "testapp",
			Slices: []plan.Slice{
				{
					Name:       "api",
					Kind:       plan.SliceKindWeb,
					Image:      "api:latest",
					Port:       8080,
					Public:     true,
					Replicas:   2,
					HealthPath: "/healthz",
					Needs:      []string{"db"},
				},
				{
					Name:     "db",
					Kind:     plan.SliceKindDatabase,
					Image:    "postgres:16",
					Port:     5432,
					Public:   false,
					Replicas: 1,
					Database: &plan.DatabaseSpec{Storage: "10Gi"},
				},
			},
			Ingredients: []plan.Ingredient{
				{From: "api", To: "db"},
			},
		},
	}
	srv := server.New(cfg)
	return httptest.NewServer(srv.Handler())
}

func TestHealthEndpoint(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}

func TestVersionEndpoint(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/version")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "version")
	assert.Contains(t, body, "commit")
	assert.Contains(t, body, "date")
}

func TestPlanEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/plan")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSlicesEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/plan/slices")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestSliceEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/plan/slices/api")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIngredientsEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/plan/ingredients")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestJSONContentType(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

// stubCollector is a test implementation of doctor.SignalCollector.
type stubCollector struct{}

// Collect returns a minimal signal with Docker reachable and no images or ports.
func (s stubCollector) Collect(_ context.Context) (*doctor.Signal, error) {
	return &doctor.Signal{DockerReachable: true}, nil
}

// testServerWithDoctor creates an httptest.Server with the doctor engine configured.
func testServerWithDoctor(t *testing.T) *httptest.Server {
	t.Helper()
	engine := doctor.New(stubCollector{})

	cfg := server.Config{
		Plan: &plan.AppPlan{
			Name: "testapp",
			Slices: []plan.Slice{
				{
					Name:  "api",
					Kind:  plan.SliceKindWeb,
					Image: "api:latest",
					Port:  8080,
				},
			},
		},
		Doctor: engine,
	}
	srv := server.New(cfg)
	return httptest.NewServer(srv.Handler())
}

func TestDoctorEndpoint_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts := testServerWithDoctor(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/doctor")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestStatusEndpoint(t *testing.T) {
	t.Parallel()
	ts := testServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/status")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Status endpoint now requires auth.
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}
