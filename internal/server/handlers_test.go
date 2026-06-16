package server

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

func TestToSliceResponse_ExpandedFields(t *testing.T) {
	t.Parallel()

	sl := plan.Slice{
		Name:             "worker",
		Kind:             plan.SliceKindStateful,
		Image:            "worker:v2",
		Port:             9090,
		Public:           false,
		Replicas:         3,
		HealthPath:       "/ready",
		Env:              map[string]string{"LOG_LEVEL": "debug"},
		Schedule:         "*/5 * * * *",
		RunOnce:          true,
		Parallelism:      2,
		Retries:          3,
		DaemonMode:       true,
		OrderedStartup:   true,
		PeerDiscovery:    true,
		StatefulStorage:  "50Gi",
		DNSName:          "worker.ns.svc",
		ServiceAccount:   "worker-sa",
		GracefulShutdown: 30,
		RestartPolicy:    "always",
		Domain:           "worker.example.com",
		Ports: []plan.PortSpec{
			{Name: "http", Port: 8080, Protocol: "TCP"},
			{Name: "grpc", Port: 9090, Protocol: "TCP"},
		},
		Probes: []plan.ProbeSpec{
			{Type: "liveness", HTTPPath: "/healthz", Interval: 10, Timeout: 5, Delay: 15},
			{Type: "readiness", TCPPort: 9090, Interval: 5},
		},
		InitSteps: []plan.InitStep{
			{Image: "busybox:latest", Command: "echo init"},
		},
		Sidecars: []plan.Sidecar{
			{Name: "proxy", Image: "envoy:v1", Ports: []plan.PortSpec{{Name: "admin", Port: 9901}}},
		},
		Mounts: []plan.MountSpec{
			{Type: "pvc", Source: "data-vol", Target: "/data", ReadOnly: false},
			{Type: "secret", Source: "tls-cert", Target: "/certs", ReadOnly: true},
		},
		AutoScale: &plan.AutoScaleSpec{
			MinReplicas: 2, MaxReplicas: 10, CPUTarget: 80, MemoryTarget: 70,
		},
		DisruptionBudget: &plan.DisruptionBudgetSpec{
			MinAvailable: 1, MaxUnavailable: 2,
		},
		Security: &plan.SecuritySpec{
			RunAsUser: 1000, RunAsGroup: 1000, ReadOnlyRoot: true,
			DropCapabilities: []string{"ALL"}, AddCapabilities: []string{"NET_BIND_SERVICE"},
		},
		Permissions: []plan.Permission{
			{Verbs: []string{"get", "list"}, Resources: []string{"pods"}, Namespace: "default"},
		},
		Scheduling: &plan.SchedulingSpec{
			SpreadTopology: "topology.kubernetes.io/zone", AntiAffinity: true,
		},
		NetworkPolicy: &plan.NetworkPolicySpec{
			AllowFrom: []string{"frontend"}, AllowNamespace: []string{"prod"}, DenyAll: true,
		},
		Lifecycle: &plan.LifecycleSpec{
			PreStopCommand: "kill -SIGTERM 1", PreStopWait: 10, PostStartCommand: "echo started",
		},
		UpdateStrategy: &plan.UpdateStrategySpec{
			MaxSurge: "25%", MaxUnavailable: "0",
		},
	}

	resp := toSliceResponse(sl)

	// Verify basic fields preserved.
	assert.Equal(t, "worker", resp.Name)
	assert.Equal(t, "stateful", resp.Kind)
	assert.Equal(t, 3, resp.Replicas)

	// Verify expanded scalar fields.
	assert.Equal(t, "*/5 * * * *", resp.Schedule)
	assert.True(t, resp.RunOnce)
	assert.Equal(t, 2, resp.Parallelism)
	assert.Equal(t, 3, resp.Retries)
	assert.True(t, resp.DaemonMode)
	assert.True(t, resp.OrderedStartup)
	assert.True(t, resp.PeerDiscovery)
	assert.Equal(t, "50Gi", resp.StatefulStorage)
	assert.Equal(t, "worker.ns.svc", resp.DNSName)
	assert.Equal(t, "worker-sa", resp.ServiceAccount)
	assert.Equal(t, 30, resp.GracefulShutdown)
	assert.Equal(t, "always", resp.RestartPolicy)
	assert.Equal(t, "worker.example.com", resp.Domain)
	assert.Equal(t, map[string]string{"LOG_LEVEL": "debug"}, resp.Env)

	// Verify slice fields.
	require.Len(t, resp.Ports, 2)
	assert.Equal(t, "http", resp.Ports[0].Name)
	assert.Equal(t, 8080, resp.Ports[0].Port)

	require.Len(t, resp.Probes, 2)
	assert.Equal(t, "liveness", resp.Probes[0].Type)
	assert.Equal(t, "/healthz", resp.Probes[0].HTTPPath)

	require.Len(t, resp.InitSteps, 1)
	assert.Equal(t, "busybox:latest", resp.InitSteps[0].Image)

	require.Len(t, resp.Sidecars, 1)
	assert.Equal(t, "proxy", resp.Sidecars[0].Name)
	require.Len(t, resp.Sidecars[0].Ports, 1)

	require.Len(t, resp.Mounts, 2)
	assert.Equal(t, "pvc", resp.Mounts[0].Type)
	assert.True(t, resp.Mounts[1].ReadOnly)

	// Verify pointer fields.
	require.NotNil(t, resp.AutoScale)
	assert.Equal(t, 10, resp.AutoScale.MaxReplicas)
	assert.Equal(t, 80, resp.AutoScale.CPUTarget)

	require.NotNil(t, resp.DisruptionBudget)
	assert.Equal(t, 1, resp.DisruptionBudget.MinAvailable)

	require.NotNil(t, resp.Security)
	assert.Equal(t, 1000, resp.Security.RunAsUser)
	assert.True(t, resp.Security.ReadOnlyRoot)
	assert.Equal(t, []string{"ALL"}, resp.Security.DropCapabilities)

	require.Len(t, resp.Permissions, 1)
	assert.Equal(t, []string{"get", "list"}, resp.Permissions[0].Verbs)

	require.NotNil(t, resp.Scheduling)
	assert.True(t, resp.Scheduling.AntiAffinity)

	require.NotNil(t, resp.NetworkPolicy)
	assert.True(t, resp.NetworkPolicy.DenyAll)
	assert.Equal(t, []string{"frontend"}, resp.NetworkPolicy.AllowFrom)

	require.NotNil(t, resp.Lifecycle)
	assert.Equal(t, "kill -SIGTERM 1", resp.Lifecycle.PreStopCommand)
	assert.Equal(t, 10, resp.Lifecycle.PreStopWait)

	require.NotNil(t, resp.UpdateStrategy)
	assert.Equal(t, "25%", resp.UpdateStrategy.MaxSurge)
}

func TestToSliceResponse_OmitsEmptyExpandedFields(t *testing.T) {
	t.Parallel()

	// A simple slice with no expanded fields should produce clean JSON.
	sl := plan.Slice{
		Name:     "api",
		Kind:     plan.SliceKindWeb,
		Image:    "api:latest",
		Port:     8080,
		Public:   true,
		Replicas: 1,
	}

	resp := toSliceResponse(sl)
	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	// Expanded fields should be absent (omitempty).
	for _, key := range []string{
		"ports", "probes", "init_steps", "sidecars", "mounts", "env",
		"schedule", "stateful_storage", "dns_name", "service_account",
		"restart_policy", "domain", "auto_scale", "disruption_budget",
		"security", "permissions", "scheduling", "network_policy",
		"lifecycle", "update_strategy",
	} {
		_, exists := raw[key]
		assert.False(t, exists, "field %q should be omitted for simple slice", key)
	}

	// Core fields should still be present.
	assert.Equal(t, "api", raw["name"])
	assert.Equal(t, "web", raw["kind"])
}
