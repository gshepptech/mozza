package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gshepptech/mozza/internal/store"
)

type createClusterRequest struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}

type clusterResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type clustersListResponse struct {
	Clusters []clusterResponse `json:"clusters"`
}

// handleCreateCluster creates a new cluster record. Requires the user to belong
// to at least one team (cluster management is a team-level privilege).
func (s *Server) handleCreateCluster() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		teams, err := s.cfg.Store.TeamsForUser(user.ID)
		if err != nil || len(teams) == 0 {
			Error(w, http.StatusForbidden, "you must belong to a team to manage clusters")
			return
		}

		var req createClusterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			Error(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.Kubeconfig == "" {
			Error(w, http.StatusBadRequest, "name and kubeconfig are required")
			return
		}

		cluster, err := s.cfg.Store.CreateCluster(req.Name, req.Kubeconfig)
		if err != nil {
			if errors.Is(err, store.ErrConflict) {
				Error(w, http.StatusConflict, "cluster name already exists")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to create cluster")
			return
		}

		JSON(w, http.StatusCreated, clusterResponse{
			ID:        cluster.ID,
			Name:      cluster.Name,
			CreatedAt: cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: cluster.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// handleListClusters returns all clusters without kubeconfig.
func (s *Server) handleListClusters() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clusters, err := s.cfg.Store.ListClusters()
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to list clusters")
			return
		}

		resp := clustersListResponse{
			Clusters: make([]clusterResponse, len(clusters)),
		}
		for i, c := range clusters {
			resp.Clusters[i] = clusterResponse{
				ID:        c.ID,
				Name:      c.Name,
				CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}
		JSON(w, http.StatusOK, resp)
	}
}

// handleGetCluster returns a single cluster by ID without kubeconfig.
func (s *Server) handleGetCluster() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		cluster, err := s.cfg.Store.ClusterByID(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "cluster not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to get cluster")
			return
		}

		JSON(w, http.StatusOK, clusterResponse{
			ID:        cluster.ID,
			Name:      cluster.Name,
			CreatedAt: cluster.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: cluster.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// handleDeleteCluster removes a cluster by ID. Requires team membership.
func (s *Server) handleDeleteCluster() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		teams, err := s.cfg.Store.TeamsForUser(user.ID)
		if err != nil || len(teams) == 0 {
			Error(w, http.StatusForbidden, "you must belong to a team to manage clusters")
			return
		}

		id := chi.URLParam(r, "id")
		if err := s.cfg.Store.DeleteCluster(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				Error(w, http.StatusNotFound, "cluster not found")
				return
			}
			Error(w, http.StatusInternalServerError, "failed to delete cluster")
			return
		}
		JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
