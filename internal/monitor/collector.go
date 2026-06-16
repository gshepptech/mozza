// Package monitor provides background metrics collection from Docker containers
// and health check polling for deployed applications.
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gshepptech/mozza/internal/store"
)

const (
	// collectInterval is how often Docker stats are polled.
	collectInterval = 15 * time.Second
	// pruneInterval is how often old metrics are pruned.
	pruneInterval = 1 * time.Hour
	// pruneMaxAge is the maximum age for retained metrics (7 days).
	pruneMaxAge = 7 * 24 * time.Hour
)

// dockerStats mirrors the JSON output of "docker stats --no-stream --format json".
type dockerStats struct {
	Container string `json:"Container"`
	Name      string `json:"Name"`
	ID        string `json:"ID"`
	CPUPerc   string `json:"CPUPerc"`
	MemUsage  string `json:"MemUsage"`
	NetIO     string `json:"NetIO"`
	PIDs      string `json:"PIDs"`
}

// dockerStatsAPI mirrors the JSON from the Docker engine API /containers/{id}/stats.
type dockerStatsAPI struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  int64   `json:"total_usage"`
			PercpuUsage []int64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
		OnlineCPUs     int   `json:"online_cpus"`
	} `json:"cpu_stats"`
	PrecpuStats struct {
		CPUUsage struct {
			TotalUsage int64 `json:"total_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage int64            `json:"usage"`
		Stats map[string]int64 `json:"stats"`
		Limit int64            `json:"limit"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes int64 `json:"rx_bytes"`
		TxBytes int64 `json:"tx_bytes"`
	} `json:"networks"`
}

// AppMapping associates a deployment/app string ID with a numeric ID for metric storage
// and the Docker container name for stats collection.
type AppMapping struct {
	AppID         int64
	DeploymentID  string
	ContainerName string
}

// Collector periodically collects Docker container metrics and persists them.
type Collector struct {
	store  *store.Store
	mu     sync.RWMutex
	apps   []AppMapping
	cancel context.CancelFunc
	done   chan struct{}
	// prevStats tracks previous stats for delta calculations.
	prevStats map[string]*dockerStatsAPI
	// TSStore is the optional in-memory time-series store for fast queries.
	TSStore *TimeSeriesStore
}

// NewCollector creates a new metrics collector backed by the given store.
func NewCollector(st *store.Store) *Collector {
	return &Collector{
		store:     st,
		done:      make(chan struct{}),
		prevStats: make(map[string]*dockerStatsAPI),
	}
}

// RegisterApp adds an app to be monitored. The containerName should match the
// Docker container name to poll stats from.
func (c *Collector) RegisterApp(appID int64, deploymentID, containerName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Avoid duplicates.
	for _, a := range c.apps {
		if a.AppID == appID {
			return
		}
	}
	c.apps = append(c.apps, AppMapping{
		AppID:         appID,
		DeploymentID:  deploymentID,
		ContainerName: containerName,
	})
}

// UnregisterApp removes an app from monitoring.
func (c *Collector) UnregisterApp(appID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, a := range c.apps {
		if a.AppID == appID {
			c.apps = append(c.apps[:i], c.apps[i+1:]...)
			return
		}
	}
}

// Start begins the background collection loop. Call Stop to shut it down.
func (c *Collector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	go c.collectLoop(ctx)
	go c.pruneLoop(ctx)

	slog.Info("monitor collector started",
		"collect_interval", collectInterval,
		"prune_interval", pruneInterval)
}

// Stop shuts down the collector and waits for the background goroutines to exit.
func (c *Collector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
	slog.Info("monitor collector stopped")
}

func (c *Collector) collectLoop(ctx context.Context) {
	defer close(c.done)

	ticker := time.NewTicker(collectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.collectAll(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n, err := c.store.PruneMetrics(ctx, pruneMaxAge)
			if err != nil {
				slog.Error("metrics prune failed", "error", err)
			} else if n > 0 {
				slog.Info("pruned old metrics", "count", n)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (c *Collector) collectAll(ctx context.Context) {
	c.mu.RLock()
	apps := make([]AppMapping, len(c.apps))
	copy(apps, c.apps)
	c.mu.RUnlock()

	if len(apps) == 0 {
		return
	}

	for _, app := range apps {
		if err := c.collectOne(ctx, app); err != nil {
			slog.Debug("collect stats failed",
				"container", app.ContainerName,
				"error", err)
		}
	}
}

func (c *Collector) collectOne(ctx context.Context, app AppMapping) error {
	stats, err := dockerInspectStats(ctx, app.ContainerName)
	if err != nil {
		return fmt.Errorf("collectOne %s: %w", app.ContainerName, err)
	}

	cpu := calcCPUPercent(stats)
	mem := calcMemoryUsage(stats)
	rx, tx := calcNetwork(stats)

	now := time.Now()
	if err := c.store.RecordMetric(ctx, app.AppID, now.Unix(), cpu, mem, rx, tx, 0); err != nil {
		return fmt.Errorf("collectOne record: %w", err)
	}

	if c.TSStore != nil {
		c.TSStore.Record(strconv.FormatInt(app.AppID, 10), TimePoint{
			Timestamp:  now,
			CPU:        cpu,
			Memory:     mem,
			NetworkIn:  rx,
			NetworkOut: tx,
		})
	}

	c.mu.Lock()
	c.prevStats[app.ContainerName] = stats
	c.mu.Unlock()

	return nil
}

// dockerInspectStats runs "docker inspect" to get container stats via the CLI.
// This is the exec fallback approach (no Docker client library dependency).
func dockerInspectStats(ctx context.Context, containerName string) (*dockerStatsAPI, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "docker", "stats", containerName,
		"--no-stream", "--format", "{{json .}}")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker stats: %w: %s", err, stderr.String())
	}

	var raw dockerStats
	if err := json.Unmarshal(stdout.Bytes(), &raw); err != nil {
		return nil, fmt.Errorf("parse docker stats: %w", err)
	}

	return parseDockerCLIStats(raw)
}

// parseDockerCLIStats converts CLI docker stats JSON into our internal format.
func parseDockerCLIStats(raw dockerStats) (*dockerStatsAPI, error) {
	stats := &dockerStatsAPI{}

	// Parse CPU percentage (e.g., "0.50%" -> 0.5).
	cpu := parsePercent(raw.CPUPerc)
	// Set up fake deltas so calcCPUPercent returns the parsed percentage.
	// Formula: (deltaCPU / deltaSystem) * numCores * 100 = cpu
	// With numCores=1, deltaSystem=1e9: deltaCPU = cpu/100 * 1e9
	stats.CPUStats.CPUUsage.TotalUsage = int64(cpu / 100.0 * 1e9)
	stats.CPUStats.SystemCPUUsage = 1e9
	stats.CPUStats.OnlineCPUs = 1
	stats.PrecpuStats.CPUUsage.TotalUsage = 0
	stats.PrecpuStats.SystemCPUUsage = 0

	// Parse memory usage (e.g., "100MiB / 1GiB").
	mem := parseMemUsage(raw.MemUsage)
	stats.MemoryStats.Usage = mem

	// Parse network I/O (e.g., "1.5kB / 2.3kB").
	rx, tx := parseNetIO(raw.NetIO)
	stats.Networks = map[string]struct {
		RxBytes int64 `json:"rx_bytes"`
		TxBytes int64 `json:"tx_bytes"`
	}{
		"eth0": {RxBytes: rx, TxBytes: tx},
	}

	return stats, nil
}

// calcCPUPercent calculates CPU usage percentage from Docker stats.
// Formula: (delta_cpu / delta_system) * num_cores * 100
func calcCPUPercent(stats *dockerStatsAPI) float64 {
	deltaCPU := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PrecpuStats.CPUUsage.TotalUsage)
	deltaSystem := float64(stats.CPUStats.SystemCPUUsage - stats.PrecpuStats.SystemCPUUsage)

	if deltaSystem <= 0 {
		return 0
	}

	numCores := stats.CPUStats.OnlineCPUs
	if numCores == 0 {
		numCores = len(stats.CPUStats.CPUUsage.PercpuUsage)
	}
	if numCores == 0 {
		numCores = 1
	}

	return (deltaCPU / deltaSystem) * float64(numCores) * 100.0
}

// calcMemoryUsage returns memory usage in bytes (usage - cache).
func calcMemoryUsage(stats *dockerStatsAPI) int64 {
	usage := stats.MemoryStats.Usage
	cache := stats.MemoryStats.Stats["cache"]
	result := usage - cache
	if result < 0 {
		return usage
	}
	return result
}

// calcNetwork returns total rx and tx bytes across all interfaces.
func calcNetwork(stats *dockerStatsAPI) (rx, tx int64) {
	for _, iface := range stats.Networks {
		rx += iface.RxBytes
		tx += iface.TxBytes
	}
	return rx, tx
}
