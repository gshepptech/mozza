package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// httpClient is the shared HTTP client used for GitHub API requests.
// It is a package-level variable to allow tests to replace it.
var httpClient = &http.Client{Timeout: 15 * time.Second}

const userAgent = "mozza-importer/1.0"

// URL builders — package-level vars so tests can override them.
var fetchRepoMetaURL = func(owner, repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
}
var listRootFilesURL = func(owner, repo string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/", owner, repo)
}
var fetchFileURL = func(owner, repo, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/HEAD/%s", owner, repo, path)
}
var checkGHCRURL = func(owner, repo string) string {
	return fmt.Sprintf("https://ghcr.io/v2/%s/%s/tags/list", owner, repo)
}

// ParseGitHubURL extracts the owner and repository name from a GitHub URL.
// It handles formats: https://github.com/owner/repo,
// https://github.com/owner/repo.git, github.com/owner/repo,
// and https://github.com/owner/repo/tree/main/...
func ParseGitHubURL(rawURL string) (owner, repo string, err error) {
	raw := strings.TrimSpace(rawURL)
	if raw == "" {
		return "", "", fmt.Errorf("empty URL")
	}

	// Normalise bare "github.com/..." to a full URL so url.Parse works.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid URL: %w", err)
	}

	if u.Host != "github.com" && u.Host != "www.github.com" {
		return "", "", fmt.Errorf("not a GitHub URL: %s", u.Host)
	}

	// Split the path into segments. We expect at least /owner/repo.
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("could not extract owner/repo from %s", rawURL)
	}

	owner = parts[0]
	repo = strings.TrimSuffix(parts[1], ".git")
	return owner, repo, nil
}

// repoMeta is the subset of the GitHub repo API response we use.
type repoMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// FetchRepoMeta returns the repository name and description from the GitHub API.
// When token is non-empty it is sent as a Bearer token for private repo access.
func FetchRepoMeta(owner, repo, token string) (name, description string, err error) {
	apiURL := fetchRepoMetaURL(owner, repo)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("fetching repo metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("authentication failed — check your token has 'repo' scope")
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("repository %s/%s not found", owner, repo)
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", "", fmt.Errorf("GitHub API rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var meta repoMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", "", fmt.Errorf("decoding repo metadata: %w", err)
	}
	return meta.Name, meta.Description, nil
}

// contentEntry represents a single item from the GitHub contents API.
type contentEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "dir"
}

// ListRootFiles returns the file and directory names at the repository root.
// When token is non-empty it is sent as a Bearer token for private repo access.
func ListRootFiles(owner, repo, token string) ([]string, error) {
	apiURL := listRootFilesURL(owner, repo)

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("listing repo contents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed — check your token has 'repo' scope")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("repository %s/%s not found", owner, repo)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("GitHub API rate limit exceeded")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var entries []contentEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("decoding contents: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		// Append "/" suffix for directories so callers can distinguish them.
		if e.Type == "dir" {
			names = append(names, e.Name+"/")
		} else {
			names = append(names, e.Name)
		}
	}
	return names, nil
}

// FetchFile returns the raw content of a file in the repository.
// When token is non-empty it is sent as a Bearer token for private repo access.
func FetchFile(owner, repo, path, token string) (string, error) {
	rawURL := fetchFileURL(owner, repo, path)

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("authentication failed — check your token has 'repo' scope")
	}
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("file %s not found in %s/%s", path, owner, repo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching file: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading file body: %w", err)
	}
	return string(body), nil
}

// CheckGHCR checks whether a container image exists on ghcr.io for the given
// owner/repo. Returns true if the image is available, false otherwise. This is
// a best-effort check — network errors or unexpected responses return false
// without an error so callers can fall back to build instructions.
func CheckGHCR(owner, repo string) bool {
	checkURL := checkGHCRURL(owner, repo)

	req, err := http.NewRequest(http.MethodHead, checkURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
