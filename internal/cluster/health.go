package cluster

import (
	"log/slog"
	"sync"
	"time"

	"k8s.io/client-go/kubernetes"
)

// staleThreshold is the maximum age of a successful probe before the
// status is considered stale.
const staleThreshold = 30 * time.Second

// StatusResponse is the JSON-serialisable health status of a cluster.
type StatusResponse struct {
	Reachable bool      `json:"reachable"`
	LastSeen  time.Time `json:"last_seen"`
	Error     string    `json:"error,omitempty"`
	Stale     bool      `json:"stale"`
}

// HealthMonitor probes cluster health on a background goroutine.
// Other components check IsReachable() before making K8s API calls.
type HealthMonitor struct {
	clientFn  func() (kubernetes.Interface, error)
	mu        sync.RWMutex
	reachable bool
	lastSeen  time.Time
	lastError string
	interval  time.Duration
	stopCh    chan struct{}
}

// NewHealthMonitor creates a HealthMonitor that uses clientFn to obtain
// a Kubernetes clientset and probes at the given interval.
func NewHealthMonitor(clientFn func() (kubernetes.Interface, error), interval time.Duration) *HealthMonitor {
	return &HealthMonitor{
		clientFn: clientFn,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background probe loop. It runs an initial probe
// immediately, then repeats at the configured interval until Stop is called.
func (h *HealthMonitor) Start() {
	go h.loop()
}

// Stop signals the background probe to exit.
func (h *HealthMonitor) Stop() {
	close(h.stopCh)
}

// IsReachable returns true if the most recent probe succeeded.
func (h *HealthMonitor) IsReachable() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.reachable
}

// Status returns the current health status snapshot.
func (h *HealthMonitor) Status() StatusResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return StatusResponse{
		Reachable: h.reachable,
		LastSeen:  h.lastSeen,
		Error:     h.lastError,
		Stale:     !h.lastSeen.IsZero() && time.Since(h.lastSeen) > staleThreshold,
	}
}

// loop runs the probe on a ticker until stopCh is closed.
func (h *HealthMonitor) loop() {
	h.probe()

	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.probe()
		}
	}
}

// probe performs a single health check by calling Discovery().ServerVersion().
func (h *HealthMonitor) probe() {
	cs, err := h.clientFn()
	if err != nil {
		h.setUnreachable(err.Error())
		return
	}

	_, err = cs.Discovery().ServerVersion()
	if err != nil {
		h.setUnreachable(err.Error())
		slog.Warn("cluster health probe failed", "error", err)
		return
	}

	h.mu.Lock()
	h.reachable = true
	h.lastSeen = time.Now()
	h.lastError = ""
	h.mu.Unlock()
}

// setUnreachable records a failed probe.
func (h *HealthMonitor) setUnreachable(msg string) {
	h.mu.Lock()
	h.reachable = false
	h.lastError = msg
	h.mu.Unlock()
}
