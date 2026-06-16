package cluster

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// cacheStaleThreshold is the maximum age of cached data before it is
// considered stale. Handlers expose this in the _cache response metadata.
const cacheStaleThreshold = 30 * time.Second

// CacheMeta holds per-resource-type freshness metadata.
type CacheMeta struct {
	LastUpdated time.Time `json:"last_updated"`
	Healthy     bool      `json:"healthy"`
	Error       string    `json:"error,omitempty"`
}

// CacheEnvelope is embedded in API responses to expose cache freshness.
type CacheEnvelope struct {
	AgeSeconds float64 `json:"age_seconds"`
	Stale      bool    `json:"stale"`
}

// --- Info types (JSON-serialisable, owned by the cluster package) ---

// NodeInfo is a cache-friendly representation of a Kubernetes node.
type NodeInfo struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Roles      string `json:"roles"`
	Age        string `json:"age"`
	Version    string `json:"version"`
	CPU        string `json:"cpu"`
	Memory     string `json:"memory"`
	InternalIP string `json:"internal_ip"`
}

// PodInfo is a cache-friendly representation of a Kubernetes pod.
type PodInfo struct {
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

// DeploymentInfo is a cache-friendly representation of a Kubernetes deployment.
type DeploymentInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Ready     string            `json:"ready"`
	UpToDate  int32             `json:"up_to_date"`
	Available int32             `json:"available"`
	Age       string            `json:"age"`
	Image     string            `json:"image,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// NamespaceInfo is a cache-friendly representation of a Kubernetes namespace.
type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Age    string `json:"age"`
}

// ServiceInfo is a cache-friendly representation of a Kubernetes service.
type ServiceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	ClusterIP string `json:"cluster_ip"`
	Ports     string `json:"ports"`
	Age       string `json:"age"`
}

// EventInfo is a cache-friendly representation of a Kubernetes event.
type EventInfo struct {
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"`
	Namespace string `json:"namespace"`
	Age       string `json:"age"`
	Count     int32  `json:"count"`
}

// MetricsInfo is a cache-friendly aggregate of cluster-level metrics.
type MetricsInfo struct {
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

// ClusterCache stores pre-processed Kubernetes resource data that is
// continually refreshed by the InformerManager. Handlers read from the
// cache instead of making live API calls.
type ClusterCache struct {
	mu          sync.RWMutex
	pods        []PodInfo
	deployments []DeploymentInfo
	nodes       []NodeInfo
	services    []ServiceInfo
	events      []EventInfo
	namespaces  []NamespaceInfo
	metrics     *MetricsInfo
	meta        map[string]CacheMeta
}

// NewClusterCache creates an empty ClusterCache.
func NewClusterCache() *ClusterCache {
	return &ClusterCache{
		meta: make(map[string]CacheMeta),
	}
}

// envelope builds a CacheEnvelope for the given resource type.
func (c *ClusterCache) envelope(resource string) CacheEnvelope {
	m, ok := c.meta[resource]
	if !ok {
		return CacheEnvelope{Stale: true}
	}
	age := time.Since(m.LastUpdated).Seconds()
	return CacheEnvelope{
		AgeSeconds: age,
		Stale:      age > cacheStaleThreshold.Seconds(),
	}
}

// Pods returns a copy of the cached pod list and its metadata.
func (c *ClusterCache) Pods() ([]PodInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PodInfo, len(c.pods))
	copy(out, c.pods)
	return out, c.envelope("pods")
}

// Nodes returns a copy of the cached node list and its metadata.
func (c *ClusterCache) Nodes() ([]NodeInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]NodeInfo, len(c.nodes))
	copy(out, c.nodes)
	return out, c.envelope("nodes")
}

// Deployments returns a copy of the cached deployment list and its metadata.
func (c *ClusterCache) Deployments() ([]DeploymentInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]DeploymentInfo, len(c.deployments))
	copy(out, c.deployments)
	return out, c.envelope("deployments")
}

// Namespaces returns a copy of the cached namespace list and its metadata.
func (c *ClusterCache) Namespaces() ([]NamespaceInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]NamespaceInfo, len(c.namespaces))
	copy(out, c.namespaces)
	return out, c.envelope("namespaces")
}

// Services returns a copy of the cached service list and its metadata.
func (c *ClusterCache) Services() ([]ServiceInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ServiceInfo, len(c.services))
	copy(out, c.services)
	return out, c.envelope("services")
}

// Events returns a copy of the cached event list and its metadata.
func (c *ClusterCache) Events() ([]EventInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]EventInfo, len(c.events))
	copy(out, c.events)
	return out, c.envelope("events")
}

// Metrics returns a copy of the cached cluster metrics and its metadata.
func (c *ClusterCache) Metrics() (*MetricsInfo, CacheEnvelope) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.metrics == nil {
		return nil, c.envelope("metrics")
	}
	cp := *c.metrics
	return &cp, c.envelope("metrics")
}

// --- Mutators (called by InformerManager) ---

// SetPods replaces the cached pod list.
func (c *ClusterCache) SetPods(pods []PodInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pods = pods
	c.meta["pods"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetNodes replaces the cached node list.
func (c *ClusterCache) SetNodes(nodes []NodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes = nodes
	c.meta["nodes"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetDeployments replaces the cached deployment list.
func (c *ClusterCache) SetDeployments(deps []DeploymentInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deployments = deps
	c.meta["deployments"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetNamespaces replaces the cached namespace list.
func (c *ClusterCache) SetNamespaces(ns []NamespaceInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.namespaces = ns
	c.meta["namespaces"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetServices replaces the cached service list.
func (c *ClusterCache) SetServices(svcs []ServiceInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services = svcs
	c.meta["services"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetEvents replaces the cached event list.
func (c *ClusterCache) SetEvents(events []EventInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = events
	c.meta["events"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// SetMetrics replaces the cached cluster metrics.
func (c *ClusterCache) SetMetrics(m *MetricsInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = m
	c.meta["metrics"] = CacheMeta{LastUpdated: time.Now(), Healthy: true}
}

// cacheSnapshot is the JSON-serialisable representation of all cached data.
type cacheSnapshot struct {
	Pods        []PodInfo            `json:"pods,omitempty"`
	Deployments []DeploymentInfo     `json:"deployments,omitempty"`
	Nodes       []NodeInfo           `json:"nodes,omitempty"`
	Services    []ServiceInfo        `json:"services,omitempty"`
	Events      []EventInfo          `json:"events,omitempty"`
	Namespaces  []NamespaceInfo      `json:"namespaces,omitempty"`
	Metrics     *MetricsInfo         `json:"metrics,omitempty"`
	Meta        map[string]CacheMeta `json:"meta,omitempty"`
}

// SerializeSnapshot returns a JSON string of all cached data.
func (c *ClusterCache) SerializeSnapshot() string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snap := cacheSnapshot{
		Pods:        c.pods,
		Deployments: c.deployments,
		Nodes:       c.nodes,
		Services:    c.services,
		Events:      c.events,
		Namespaces:  c.namespaces,
		Metrics:     c.metrics,
		Meta:        c.meta,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		slog.Error("failed to serialize cache snapshot", "error", err)
		return "{}"
	}
	return string(data)
}

// LoadFromSnapshot populates the cache from a JSON snapshot string.
func (c *ClusterCache) LoadFromSnapshot(data string) error {
	var snap cacheSnapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.pods = snap.Pods
	c.deployments = snap.Deployments
	c.nodes = snap.Nodes
	c.services = snap.Services
	c.events = snap.Events
	c.namespaces = snap.Namespaces
	c.metrics = snap.Metrics
	if snap.Meta != nil {
		c.meta = snap.Meta
	}
	return nil
}

// SetError records an error for a resource type without clearing its data.
func (c *ClusterCache) SetError(resource string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.meta[resource]
	m.Healthy = false
	m.Error = err.Error()
	c.meta[resource] = m
}
