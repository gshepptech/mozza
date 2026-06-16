package importer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeGitHub sets up a test HTTP server that mimics the GitHub API endpoints
// needed by the importer. It replaces the package-level httpClient so all
// requests go to the test server. The caller should defer the returned
// cleanup function.
func fakeGitHub(t *testing.T, handler http.Handler) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	old := httpClient
	httpClient = srv.Client()

	// Rewrite absolute URLs to point at the test server.
	origFetchRepoMeta := fetchRepoMetaURL
	origListRootFiles := listRootFilesURL
	origFetchFile := fetchFileURL
	origCheckGHCR := checkGHCRURL

	fetchRepoMetaURL = func(owner, repo string) string {
		return srv.URL + "/repos/" + owner + "/" + repo
	}
	listRootFilesURL = func(owner, repo string) string {
		return srv.URL + "/repos/" + owner + "/" + repo + "/contents/"
	}
	fetchFileURL = func(owner, repo, path string) string {
		return srv.URL + "/raw/" + owner + "/" + repo + "/" + path
	}
	checkGHCRURL = func(owner, repo string) string {
		return srv.URL + "/v2/" + owner + "/" + repo + "/tags/list"
	}

	return func() {
		httpClient = old
		fetchRepoMetaURL = origFetchRepoMeta
		listRootFilesURL = origListRootFiles
		fetchFileURL = origFetchFile
		checkGHCRURL = origCheckGHCR
		srv.Close()
	}
}

// dockerfileOnlyMux returns a handler that serves a repo with only a Dockerfile.
func dockerfileOnlyMux(authToken string, ghcrExists bool) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/repos/testowner/testrepo", func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(repoMeta{Name: "testrepo", Description: "A test repo"})
	})

	mux.HandleFunc("/repos/testowner/testrepo/contents/", func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, `{"message":"Bad credentials"}`, http.StatusUnauthorized)
			return
		}
		entries := []contentEntry{
			{Name: "Dockerfile", Type: "file"},
			{Name: "README.md", Type: "file"},
		}
		_ = json.NewEncoder(w).Encode(entries)
	})

	mux.HandleFunc("/raw/testowner/testrepo/Dockerfile", func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("FROM node:20-alpine\nWORKDIR /app\nCOPY . .\nEXPOSE 3000\nCMD [\"node\", \"index.js\"]\n"))
	})

	mux.HandleFunc("/v2/testowner/testrepo/tags/list", func(w http.ResponseWriter, _ *http.Request) {
		if ghcrExists {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"tags":["latest"]}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	})

	return mux
}

func TestScanWithToken(t *testing.T) {
	cleanup := fakeGitHub(t, dockerfileOnlyMux("secret-token-123", false))
	defer cleanup()

	// Without token — should fail with auth error.
	_, err := Scan("https://github.com/testowner/testrepo", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "authentication failed")

	// With correct token — should succeed.
	result, err := Scan("https://github.com/testowner/testrepo", &ScanOptions{Token: "secret-token-123"})
	require.NoError(t, err)
	assert.Equal(t, "testrepo", result.RepoName)
	assert.NotNil(t, result.Generated)
	assert.Equal(t, "from-dockerfile", result.Generated.Method)
}

func TestScanWithoutToken(t *testing.T) {
	// Public repo — no auth required.
	cleanup := fakeGitHub(t, dockerfileOnlyMux("", false))
	defer cleanup()

	result, err := Scan("https://github.com/testowner/testrepo", nil)
	require.NoError(t, err)
	assert.Equal(t, "testrepo", result.RepoName)
	assert.NotNil(t, result.Generated)
	assert.Equal(t, "from-dockerfile", result.Generated.Method)
	assert.True(t, result.Generated.NeedsBuild)
	assert.NotEmpty(t, result.Generated.BuildInstructions)
}

func TestBuildInstructions(t *testing.T) {
	// Dockerfile-only repo, no GHCR image.
	cleanup := fakeGitHub(t, dockerfileOnlyMux("", false))
	defer cleanup()

	result, err := Scan("https://github.com/testowner/testrepo", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Generated)

	assert.True(t, result.Generated.NeedsBuild)
	assert.Contains(t, result.Generated.BuildInstructions, "git clone")
	assert.Contains(t, result.Generated.BuildInstructions, "docker build")
	assert.Contains(t, result.Generated.BuildInstructions, "docker push")
	assert.Contains(t, result.Generated.BuildInstructions, "ghcr.io/testowner/testrepo:latest")
}

func TestBuildInstructions_GHCRExists(t *testing.T) {
	// Dockerfile repo with pre-built GHCR image.
	cleanup := fakeGitHub(t, dockerfileOnlyMux("", true))
	defer cleanup()

	result, err := Scan("https://github.com/testowner/testrepo", nil)
	require.NoError(t, err)
	require.NotNil(t, result.Generated)

	assert.False(t, result.Generated.NeedsBuild)
	assert.Empty(t, result.Generated.BuildInstructions)
	assert.Contains(t, result.Generated.Source, "ghcr.io/testowner/testrepo:latest")
}

func TestCheckGHCR(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       bool
	}{
		{"image exists", http.StatusOK, true},
		{"image not found", http.StatusNotFound, false},
		{"unauthorized", http.StatusUnauthorized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			old := httpClient
			httpClient = srv.Client()
			origURL := checkGHCRURL
			checkGHCRURL = func(_, _ string) string { return srv.URL + "/v2/o/r/tags/list" }
			defer func() {
				httpClient = old
				checkGHCRURL = origURL
			}()

			got := CheckGHCR("o", "r")
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchRepoMeta_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(repoMeta{Name: "r", Description: "d"})
	}))
	defer srv.Close()

	old := httpClient
	httpClient = srv.Client()
	origURL := fetchRepoMetaURL
	fetchRepoMetaURL = func(_, _ string) string { return srv.URL }
	defer func() {
		httpClient = old
		fetchRepoMetaURL = origURL
	}()

	// With token.
	_, _, err := FetchRepoMeta("o", "r", "my-token")
	require.NoError(t, err)
	assert.Equal(t, "Bearer my-token", gotAuth)

	// Without token.
	_, _, err = FetchRepoMeta("o", "r", "")
	require.NoError(t, err)
	assert.Empty(t, gotAuth)
}

func TestFetchRepoMeta_401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	old := httpClient
	httpClient = srv.Client()
	origURL := fetchRepoMetaURL
	fetchRepoMetaURL = func(_, _ string) string { return srv.URL }
	defer func() {
		httpClient = old
		fetchRepoMetaURL = origURL
	}()

	_, _, err := FetchRepoMeta("o", "r", "bad-token")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "authentication failed"))
}
