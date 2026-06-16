package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// registryImage represents a discovered image in a registry namespace.
type registryImage struct {
	Name        string `json:"name"`
	FullRef     string `json:"full_ref"`
	Description string `json:"description,omitempty"`
	LastUpdated string `json:"last_updated,omitempty"`
}

type scanNamespaceResponse struct {
	Namespace string          `json:"namespace"`
	Images    []registryImage `json:"images"`
	Total     int             `json:"total"`
}

// handleScanNamespace queries Docker Hub for all repositories under a namespace.
func (s *Server) handleScanNamespace() http.HandlerFunc {
	client := &http.Client{Timeout: 15 * time.Second}

	return func(w http.ResponseWriter, r *http.Request) {
		ns := r.URL.Query().Get("namespace")
		if ns == "" {
			Error(w, http.StatusBadRequest, "namespace query parameter required")
			return
		}

		// Sanitize: only allow alphanumeric, hyphens, underscores.
		for _, c := range ns {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
				Error(w, http.StatusBadRequest, "invalid namespace")
				return
			}
		}

		// Query Docker Hub V2 API. Cap at 100 to keep responses fast —
		// orgs with hundreds of images (e.g. bitnami/277) would be slow to load.
		url := fmt.Sprintf("https://hub.docker.com/v2/namespaces/%s/repositories/?page_size=100&ordering=last_updated", ns)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, url, nil)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to create request")
			return
		}

		resp, err := client.Do(req)
		if err != nil {
			Error(w, http.StatusBadGateway, "failed to reach Docker Hub")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			Error(w, http.StatusNotFound, fmt.Sprintf("namespace %q not found on Docker Hub", ns))
			return
		}
		if resp.StatusCode != http.StatusOK {
			Error(w, http.StatusBadGateway, fmt.Sprintf("Docker Hub returned %d", resp.StatusCode))
			return
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to read response")
			return
		}

		var hub struct {
			Count   int `json:"count"`
			Results []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				LastUpdated string `json:"last_updated"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &hub); err != nil {
			Error(w, http.StatusInternalServerError, "failed to parse Docker Hub response")
			return
		}

		images := make([]registryImage, 0, len(hub.Results))
		for _, repo := range hub.Results {
			updated := ""
			if repo.LastUpdated != "" {
				if t, err := time.Parse(time.RFC3339Nano, repo.LastUpdated); err == nil {
					updated = t.Format("2006-01-02")
				}
			}
			images = append(images, registryImage{
				Name:        repo.Name,
				FullRef:     ns + "/" + repo.Name + ":latest",
				Description: strings.TrimSpace(repo.Description),
				LastUpdated: updated,
			})
		}

		// Sort by name.
		sort.Slice(images, func(i, j int) bool { return images[i].Name < images[j].Name })

		JSON(w, http.StatusOK, scanNamespaceResponse{
			Namespace: ns,
			Images:    images,
			Total:     hub.Count,
		})
	}
}
