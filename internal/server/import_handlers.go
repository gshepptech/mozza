package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gshepptech/mozza/internal/importer"
)

type scanRepoRequest struct {
	URL   string `json:"url"`
	Token string `json:"token,omitempty"` // optional GitHub token for private repos — never stored or logged
}

type deployImportRequest struct {
	TeamID    string `json:"team_id"`
	Target    string `json:"target"`
	ClusterID string `json:"cluster_id"`
	Namespace string `json:"namespace"`
	Source    string `json:"source"`
}

// handleScanRepo scans a GitHub repo and returns a generated recipe.
// POST /api/v1/import/scan
func (s *Server) handleScanRepo() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req scanRepoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.URL == "" {
			Error(w, http.StatusBadRequest, "url is required")
			return
		}

		var opts *importer.ScanOptions
		if req.Token != "" {
			opts = &importer.ScanOptions{Token: req.Token}
		}

		result, err := importer.Scan(req.URL, opts)
		if err != nil {
			Error(w, http.StatusUnprocessableEntity, fmt.Sprintf("scan failed: %v", err))
			return
		}

		JSON(w, http.StatusOK, result)
	}
}

// handleDeployImport deploys from a generated/edited recipe source.
// POST /api/v1/import/deploy
func (s *Server) handleDeployImport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())

		var req deployImportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.TeamID == "" || req.Target == "" || req.Source == "" {
			Error(w, http.StatusBadRequest, "team_id, target, and source are required")
			return
		}
		if req.Target != "kubernetes" && req.Target != "local" {
			Error(w, http.StatusBadRequest, "target must be 'kubernetes' or 'local'")
			return
		}

		// Verify team membership.
		isMember, err := s.isTeamMember(req.TeamID, user.ID)
		if err != nil || !isMember {
			Error(w, http.StatusForbidden, "not a team member")
			return
		}

		s.deployWithSource(w, req.Source, "imported-app", req.TeamID, req.Target, req.ClusterID, user.ID)
	}
}
