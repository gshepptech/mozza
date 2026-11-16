package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/store"
)

type previewResponse struct {
	ID           int64  `json:"id"`
	AppID        int64  `json:"app_id"`
	Branch       string `json:"branch"`
	Domain       string `json:"domain,omitempty"`
	DeployID     int64  `json:"deploy_id,omitempty"`
	CreatedAt    string `json:"created_at"`
	LastActivity string `json:"last_activity,omitempty"`
}

type previewListResponse struct {
	Previews []previewResponse `json:"previews"`
}

// handleListPreviews returns active preview deploys for an app.
// GET /api/v1/apps/{id}/previews
func (s *Server) handleListPreviews() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		appID, err := parseIDParam(r)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid app ID")
			return
		}

		deploys, err := s.cfg.Store.ListPreviewDeploysByApp(r.Context(), appID)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list previews")
			return
		}

		resp := previewListResponse{
			Previews: make([]previewResponse, len(deploys)),
		}
		for i, d := range deploys {
			resp.Previews[i] = toPreviewResponse(d)
		}

		JSON(w, http.StatusOK, resp)
	}
}

// handleDeletePreview removes a specific preview deploy by branch.
// DELETE /api/v1/apps/{id}/previews/{branch}
func (s *Server) handleDeletePreview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Store == nil {
			Error(w, http.StatusServiceUnavailable, "store not configured")
			return
		}

		appID, err := parseIDParam(r)
		if err != nil {
			Error(w, http.StatusBadRequest, "invalid app ID")
			return
		}

		branch := chi.URLParam(r, "branch")
		if branch == "" {
			Error(w, http.StatusBadRequest, "branch is required")
			return
		}

		err = s.cfg.Store.DeletePreviewDeployByAppAndBranch(r.Context(), appID, branch)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "preview not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to delete preview")
			return
		}

		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// toPreviewResponse converts a store.PreviewDeploy to a previewResponse.
func toPreviewResponse(d store.PreviewDeploy) previewResponse {
	resp := previewResponse{
		ID:        d.ID,
		AppID:     d.AppID,
		Branch:    d.Branch,
		Domain:    d.Domain,
		DeployID:  d.DeployID,
		CreatedAt: d.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if d.LastActivity != nil {
		resp.LastActivity = d.LastActivity.Format("2006-01-02T15:04:05Z")
	}
	return resp
}
