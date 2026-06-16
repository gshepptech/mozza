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
)

// testServerWithAuth creates an httptest.Server with auth enabled.
func testServerWithAuth(t *testing.T) (*httptest.Server, *auth.Service, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })

	svc := auth.New(st)
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
		Store: st,
		Auth:  svc,
	}
	srv := server.New(cfg)
	return httptest.NewServer(srv.Handler()), svc, st
}

// registerUser is a helper that registers a user and returns the session cookie.
func registerUser(t *testing.T, ts *httptest.Server, email, name, password string) *http.Cookie {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"email":    email,
		"name":     name,
		"password": password,
	})
	resp, err := http.Post(ts.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)

	for _, c := range resp.Cookies() {
		if c.Name == "mozza_session" {
			return c
		}
	}
	t.Fatal("no session cookie returned from register")
	return nil
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	registerUser(t, ts, "login401@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{
		"email":    "login401@example.com",
		"password": "wrongpassword",
	})
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestLogin_ValidCredentials_Returns200(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	registerUser(t, ts, "login200@example.com", "Test", "password123")

	body, _ := json.Marshal(map[string]string{
		"email":    "login200@example.com",
		"password": "password123",
	})
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Should have a session cookie.
	var found bool
	for _, c := range resp.Cookies() {
		if c.Name == "mozza_session" {
			found = true
			assert.NotEmpty(t, c.Value)
		}
	}
	assert.True(t, found, "expected mozza_session cookie")
}

func TestRegister_ShortPassword_Returns400(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"email":    "short@example.com",
		"name":     "Short",
		"password": "abc",
	})
	resp, err := http.Post(ts.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegister_ValidData_Returns201(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"email":    "valid@example.com",
		"name":     "Valid User",
		"password": "password123",
	})
	resp, err := http.Post(ts.URL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var user map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&user))
	assert.Equal(t, "valid@example.com", user["email"])
	assert.Equal(t, "Valid User", user["name"])
	assert.NotEmpty(t, user["id"])
}

func TestProtectedEndpoint_NoSession_Returns401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	endpoints := []string{
		"/api/v1/auth/me",
		"/api/v1/plan",
		"/api/v1/plan/slices",
		"/api/v1/doctor",
		"/api/v1/status",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			resp, err := http.Get(ts.URL + ep)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

func TestMe_WithValidSession(t *testing.T) {
	t.Parallel()
	ts, _, _ := testServerWithAuth(t)
	defer ts.Close()

	cookie := registerUser(t, ts, "me@example.com", "Me User", "password123")

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/auth/me", nil)
	req.AddCookie(cookie)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var user map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&user))
	assert.Equal(t, "me@example.com", user["email"])
}
