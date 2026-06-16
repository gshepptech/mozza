package cluster

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/client-go/kubernetes"
)

func TestNewClusterCache(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	require.NotNil(t, c)
	assert.NotNil(t, c.meta, "meta map should be initialized")
}

func TestClusterCache_SetAndGetPods(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	pods := []PodInfo{
		{Name: "web-abc123", Namespace: "default", Status: "Running"},
		{Name: "api-def456", Namespace: "production", Status: "Pending"},
	}

	c.SetPods(pods)

	got, env := c.Pods()
	assert.Len(t, got, 2)
	assert.Equal(t, "web-abc123", got[0].Name)
	assert.False(t, env.Stale, "freshly set data should not be stale")
}

func TestClusterCache_SetAndGetNodes(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	nodes := []NodeInfo{
		{Name: "node-1", Status: "Ready", Roles: "control-plane"},
	}

	c.SetNodes(nodes)

	got, env := c.Nodes()
	assert.Len(t, got, 1)
	assert.Equal(t, "node-1", got[0].Name)
	assert.False(t, env.Stale)
}

func TestClusterCache_SetAndGetDeployments(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	deps := []DeploymentInfo{
		{Name: "web", Namespace: "default", Ready: "3/3"},
	}

	c.SetDeployments(deps)

	got, env := c.Deployments()
	assert.Len(t, got, 1)
	assert.Equal(t, "web", got[0].Name)
	assert.False(t, env.Stale)
}

func TestClusterCache_EmptyReturnsStale(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()

	// Without any Set calls, the envelope should be stale.
	_, env := c.Pods()
	assert.True(t, env.Stale, "unset resource should report as stale")
}

func TestClusterCache_ReturnsCopy(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	pods := []PodInfo{{Name: "original"}}
	c.SetPods(pods)

	got, _ := c.Pods()
	got[0].Name = "modified"

	again, _ := c.Pods()
	assert.Equal(t, "original", again[0].Name, "modifying returned slice should not affect cache")
}

func TestClusterCache_SerializeAndLoad(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	c.SetPods([]PodInfo{{Name: "pod-1", Namespace: "ns"}})
	c.SetNodes([]NodeInfo{{Name: "node-1"}})
	c.SetMetrics(&MetricsInfo{Nodes: 3, CPUCores: 8.0})

	snapshot := c.SerializeSnapshot()
	assert.NotEqual(t, "{}", snapshot)

	// Load into a new cache.
	c2 := NewClusterCache()
	err := c2.LoadFromSnapshot(snapshot)
	require.NoError(t, err)

	pods, _ := c2.Pods()
	assert.Len(t, pods, 1)
	assert.Equal(t, "pod-1", pods[0].Name)

	nodes, _ := c2.Nodes()
	assert.Len(t, nodes, 1)

	metrics, _ := c2.Metrics()
	require.NotNil(t, metrics)
	assert.Equal(t, 3, metrics.Nodes)
	assert.InDelta(t, 8.0, metrics.CPUCores, 0.001)
}

func TestClusterCache_LoadFromSnapshot_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	err := c.LoadFromSnapshot("not json")
	require.Error(t, err)
}

func TestClusterCache_SetError(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	c.SetPods([]PodInfo{{Name: "pod-1"}})
	c.SetError("pods", errors.New("connection refused"))

	// Data should still be there.
	pods, _ := c.Pods()
	assert.Len(t, pods, 1)

	// But meta should show unhealthy.
	assert.False(t, c.meta["pods"].Healthy)
	assert.Equal(t, "connection refused", c.meta["pods"].Error)
}

func TestClusterCache_Metrics_NilWhenUnset(t *testing.T) {
	t.Parallel()

	c := NewClusterCache()
	m, env := c.Metrics()
	assert.Nil(t, m)
	assert.True(t, env.Stale)
}

func TestNewHealthMonitor(t *testing.T) {
	t.Parallel()

	clientFn := func() (kubernetes.Interface, error) {
		return nil, fmt.Errorf("no cluster")
	}

	hm := NewHealthMonitor(clientFn, 10*time.Second)
	require.NotNil(t, hm)
	assert.False(t, hm.IsReachable(), "should not be reachable before any probe")

	status := hm.Status()
	assert.False(t, status.Reachable)
}

func TestHealthMonitor_FailedProbe(t *testing.T) {
	t.Parallel()

	clientFn := func() (kubernetes.Interface, error) {
		return nil, fmt.Errorf("dial timeout")
	}

	hm := NewHealthMonitor(clientFn, time.Hour)
	// Manually call probe (don't start the loop).
	hm.probe()

	assert.False(t, hm.IsReachable())
	status := hm.Status()
	assert.False(t, status.Reachable)
	assert.Equal(t, "dial timeout", status.Error)
}

func TestClassifyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantNil    bool
		wantCode   string
		wantStatus int
	}{
		{name: "nil error", err: nil, wantNil: true},
		{
			name:       "connection refused",
			err:        errors.New("connection refused"),
			wantCode:   CodeUnreachable,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "timeout",
			err:        errors.New("context deadline exceeded"),
			wantCode:   CodeTimeout,
			wantStatus: http.StatusGatewayTimeout,
		},
		{
			name:       "unauthorized",
			err:        errors.New("unauthorized access"),
			wantCode:   CodeUnauthorized,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "kubeconfig missing",
			err:        errors.New("no valid kubeconfig found"),
			wantCode:   CodeUnreachable,
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name:       "unknown error",
			err:        errors.New("something weird happened"),
			wantCode:   CodeInternalError,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifyError(tt.err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.wantCode, got.Code)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.NotEmpty(t, got.Message)
		})
	}
}

func TestFormatAge(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ago  time.Duration
		want string
	}{
		{name: "days", ago: 72 * time.Hour, want: "3d"},
		{name: "hours", ago: 5 * time.Hour, want: "5h"},
		{name: "minutes", ago: 15 * time.Minute, want: "15m"},
		{name: "seconds", ago: 30 * time.Second, want: "30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatAge(time.Now().Add(-tt.ago))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatAge_ZeroTime(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "", formatAge(time.Time{}))
}
