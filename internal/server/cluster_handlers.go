package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	"github.com/gshepptech/mozza/internal/cluster"
)

// --- Response types ---

type clusterNodeResponse struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Roles      string `json:"roles"`
	Age        string `json:"age"`
	Version    string `json:"version"`
	CPU        string `json:"cpu"`
	Memory     string `json:"memory"`
	InternalIP string `json:"internal_ip"`
}

type clusterPodResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Ready     string `json:"ready"`
	Restarts  int32  `json:"restarts"`
	Age       string `json:"age"`
	Node      string `json:"node"`
	IP        string `json:"ip,omitempty"`
	App       string `json:"app,omitempty"`
}

type clusterDeploymentResponse struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ready     string            `json:"ready"`
	UpToDate  int32             `json:"up_to_date"`
	Available int32             `json:"available"`
	Age       string            `json:"age"`
	Image     string            `json:"image,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type clusterNamespaceResponse struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

type clusterServiceResponse struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

type clusterEventResponse struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"`
	Namespace string `json:"namespace"`
	Age       string `json:"age"`
	Count     int32  `json:"count"`
}

type clusterMetricsResponse struct {
	Nodes       int     `json:"nodes"`
	CPUCores    float64 `json:"cpu_cores"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryGB    float64 `json:"memory_gb"`
	MemPercent  float64 `json:"memory_percent"`
	TotalPods   int     `json:"total_pods"`
	RunningPods int     `json:"running_pods"`
	PendingPods int     `json:"pending_pods"`
	FailedPods  int     `json:"failed_pods"`
	Uptime      string  `json:"uptime"`
}

type clusterInfoResponse struct {
	Connected bool                  `json:"connected"`
	Nodes     []clusterNodeResponse `json:"nodes"`
}

// clusterErr writes a classified cluster error response.
func clusterErr(w http.ResponseWriter, err error) {
	ce := cluster.ClassifyError(err)
	ClusterError(w, ce.Status, ce.Code, ce.Message)
}

// --- Handlers ---

func (s *Server) handleClusterStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, s.healthMon.Status())
	}
}

func (s *Server) handleClusterNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		nodes, env := s.clusterCache.Nodes()

		resp := make([]clusterNodeResponse, 0, len(nodes))
		for _, n := range nodes {
			resp = append(resp, clusterNodeResponse{
				Name:       n.Name,
				Status:     n.Status,
				Roles:      n.Roles,
				Age:        n.Age,
				Version:    n.Version,
				CPU:        n.CPU,
				Memory:     n.Memory,
				InternalIP: n.InternalIP,
			})
		}

		JSON(w, http.StatusOK, map[string]any{
			"connected": true,
			"nodes":     resp,
			"_cache":    env,
		})
	}
}

func (s *Server) handleClusterPods() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		pods, env := s.clusterCache.Pods()

		// Apply optional namespace filter client-side (cache stores all namespaces).
		ns := r.URL.Query().Get("namespace")
		var resp []clusterPodResponse
		for _, p := range pods {
			if ns != "" && p.Namespace != ns {
				continue
			}
			resp = append(resp, clusterPodResponse{
				Name:      p.Name,
				Namespace: p.Namespace,
				Status:    p.Status,
				Ready:     p.Ready,
				Restarts:  p.Restarts,
				Age:       p.Age,
				Node:      p.Node,
				IP:        p.IP,
				App:       p.App,
			})
		}
		if resp == nil {
			resp = []clusterPodResponse{}
		}

		JSON(w, http.StatusOK, map[string]any{
			"pods":   resp,
			"_cache": env,
		})
	}
}

func (s *Server) handleClusterDeployments() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		deps, env := s.clusterCache.Deployments()

		ns := r.URL.Query().Get("namespace")
		var resp []clusterDeploymentResponse
		for _, d := range deps {
			if ns != "" && d.Namespace != ns {
				continue
			}
			resp = append(resp, clusterDeploymentResponse{
				Name:      d.Name,
				Namespace: d.Namespace,
				Ready:     d.Ready,
				UpToDate:  d.UpToDate,
				Available: d.Available,
				Age:       d.Age,
				Image:     d.Image,
				Labels:    d.Labels,
			})
		}
		if resp == nil {
			resp = []clusterDeploymentResponse{}
		}

		JSON(w, http.StatusOK, map[string]any{
			"deployments": resp,
			"_cache":      env,
		})
	}
}

func (s *Server) handleClusterNamespaces() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		nsList, env := s.clusterCache.Namespaces()

		resp := make([]clusterNamespaceResponse, 0, len(nsList))
		for _, ns := range nsList {
			resp = append(resp, clusterNamespaceResponse{
				Name:   ns.Name,
				Status: ns.Status,
				Age:    ns.Age,
			})
		}

		JSON(w, http.StatusOK, map[string]any{
			"namespaces": resp,
			"_cache":     env,
		})
	}
}

func (s *Server) handleClusterServices() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		svcs, env := s.clusterCache.Services()

		ns := r.URL.Query().Get("namespace")
		var resp []clusterServiceResponse
		for _, svc := range svcs {
			if ns != "" && svc.Namespace != ns {
				continue
			}
			resp = append(resp, clusterServiceResponse{
				Name:      svc.Name,
				Namespace: svc.Namespace,
				Type:      svc.Type,
				ClusterIP: svc.ClusterIP,
				Ports:     svc.Ports,
				Age:       svc.Age,
			})
		}
		if resp == nil {
			resp = []clusterServiceResponse{}
		}

		JSON(w, http.StatusOK, map[string]any{
			"services": resp,
			"_cache":   env,
		})
	}
}

func (s *Server) handleClusterEvents() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		events, env := s.clusterCache.Events()

		// Limit to 50 most recent events to match previous behaviour.
		resp := make([]clusterEventResponse, 0, len(events))
		for _, e := range events {
			resp = append(resp, clusterEventResponse{
				Type:      e.Type,
				Reason:    e.Reason,
				Message:   e.Message,
				Object:    e.Object,
				Namespace: e.Namespace,
				Age:       e.Age,
				Count:     e.Count,
			})
		}

		JSON(w, http.StatusOK, map[string]any{
			"events": resp,
			"_cache": env,
		})
	}
}

func (s *Server) handleClusterMetrics() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}

		m, env := s.clusterCache.Metrics()
		if m == nil {
			JSON(w, http.StatusOK, map[string]any{
				"metrics": clusterMetricsResponse{},
				"_cache":  env,
			})
			return
		}

		JSON(w, http.StatusOK, map[string]any{
			"metrics": clusterMetricsResponse{
				Nodes:       m.Nodes,
				CPUCores:    m.CPUCores,
				CPUPercent:  m.CPUPercent,
				MemoryGB:    m.MemoryGB,
				MemPercent:  m.MemPercent,
				TotalPods:   m.TotalPods,
				RunningPods: m.RunningPods,
				PendingPods: m.PendingPods,
				FailedPods:  m.FailedPods,
				Uptime:      m.Uptime,
			},
			"_cache": env,
		})
	}
}

func (s *Server) handleClusterPodLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}
		cs, err := s.kubeClient()
		if err != nil {
			clusterErr(w, err)
			return
		}

		namespace := r.URL.Query().Get("namespace")
		pod := r.URL.Query().Get("pod")
		if namespace == "" || pod == "" {
			Error(w, http.StatusBadRequest, "namespace and pod query params required")
			return
		}

		tailLines := int64(100)
		logOpts := &corev1.PodLogOptions{TailLines: &tailLines}

		stream, err := cs.CoreV1().Pods(namespace).GetLogs(pod, logOpts).Stream(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("failed to get pod logs: %s", err.Error()))
			return
		}
		defer stream.Close()

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)

		buf := make([]byte, 4096)
		for {
			n, readErr := stream.Read(buf)
			if n > 0 {
				_, _ = w.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
	}
}

// handleRestartDeployment triggers a rolling restart by patching the
// kubectl.kubernetes.io/restartedAt annotation on the deployment.
func (s *Server) handleRestartDeployment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.healthMon.IsReachable() {
			ClusterError(w, http.StatusServiceUnavailable, cluster.CodeUnreachable, "Cluster is not reachable")
			return
		}
		cs, err := s.kubeClient()
		if err != nil {
			clusterErr(w, err)
			return
		}

		ns := chi.URLParam(r, "ns")
		name := chi.URLParam(r, "name")
		if ns == "" || name == "" {
			Error(w, http.StatusBadRequest, "namespace and deployment name are required")
			return
		}

		// Verify the deployment exists.
		_, err = cs.AppsV1().Deployments(ns).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			Error(w, http.StatusNotFound, fmt.Sprintf("deployment %s/%s not found", ns, name))
			return
		}

		// Patch the restartedAt annotation to trigger a rolling restart.
		patch := map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"metadata": map[string]any{
						"annotations": map[string]string{
							"kubectl.kubernetes.io/restartedAt": time.Now().Format(time.RFC3339),
						},
					},
				},
			},
		}
		patchBytes, err := json.Marshal(patch)
		if err != nil {
			Error(w, http.StatusInternalServerError, "failed to build patch")
			return
		}

		_, err = cs.AppsV1().Deployments(ns).Patch(
			r.Context(), name, k8stypes.StrategicMergePatchType, patchBytes, metav1.PatchOptions{},
		)
		if err != nil {
			Error(w, http.StatusInternalServerError, fmt.Sprintf("failed to restart deployment: %s", err.Error()))
			return
		}

		JSON(w, http.StatusOK, map[string]string{"status": "restarting"})
	}
}

// kubeClient and formatAge are in kube.go
