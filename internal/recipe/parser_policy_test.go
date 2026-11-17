package recipe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Permissions(t *testing.T) {
	t.Parallel()

	input := `App: t

Ctrl:
  from image myorg/ctrl:1.0
  needs permission to read pods
  needs permission to read and write deployments
  needs cluster-wide permission to manage secrets
  use account "controller-sa"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Permissions, 3)

	// "read pods"
	assert.Equal(t, []string{"get", "list", "watch"}, s.Permissions[0].Verbs)
	assert.Equal(t, []string{"pods"}, s.Permissions[0].Resources)
	assert.False(t, s.Permissions[0].ClusterWide)

	// "read and write deployments"
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch"}, s.Permissions[1].Verbs)
	assert.Equal(t, []string{"deployments"}, s.Permissions[1].Resources)
	assert.False(t, s.Permissions[1].ClusterWide)

	// "manage secrets" cluster-wide
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch", "delete"}, s.Permissions[2].Verbs)
	assert.Equal(t, []string{"secrets"}, s.Permissions[2].Resources)
	assert.True(t, s.Permissions[2].ClusterWide)

	assert.Equal(t, "controller-sa", s.ServiceAccount)
}

func TestParser_SchedulingPreferences(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  prefer nodes labeled "zone=us-west-2a"
  require nodes labeled "disk=ssd"
  spread copies across zones
  never run two copies on the same node`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Scheduling)
	require.Len(t, s.Scheduling.NodePreferences, 1)
	assert.Equal(t, LabelConstraint{Key: "zone", Value: "us-west-2a"}, s.Scheduling.NodePreferences[0])
	require.Len(t, s.Scheduling.NodeRequirements, 1)
	assert.Equal(t, LabelConstraint{Key: "disk", Value: "ssd"}, s.Scheduling.NodeRequirements[0])
	assert.Equal(t, "zones", s.Scheduling.SpreadTopology)
	assert.True(t, s.Scheduling.AntiAffinity)
}

func TestParser_NetworkPolicy(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  only accept traffic from frontend and backend`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.NetworkPolicy)
	assert.Equal(t, []string{"frontend", "backend"}, s.NetworkPolicy.AllowFrom)
}

func TestParser_NetworkPolicyBlock(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  block all traffic except from namespace "production"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.NetworkPolicy)
	assert.Equal(t, []string{"production"}, s.NetworkPolicy.AllowNamespace)
}

func TestParser_AutoScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantMin    int
		wantMax    int
		wantCPU    int
		wantMemory int
	}{
		{
			name:    "cpu based",
			input:   "App: t\n\nS:\n  from image x\n  scale between 2 and 10 copies based on cpu 80%",
			wantMin: 2, wantMax: 10, wantCPU: 80,
		},
		{
			name:    "memory based",
			input:   "App: t\n\nS:\n  from image x\n  scale between 1 and 5 copies based on memory 70%",
			wantMin: 1, wantMax: 5, wantMemory: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			require.NotNil(t, s.AutoScale)
			assert.Equal(t, tt.wantMin, s.AutoScale.MinReplicas)
			assert.Equal(t, tt.wantMax, s.AutoScale.MaxReplicas)
			assert.Equal(t, tt.wantCPU, s.AutoScale.CPUTarget)
			assert.Equal(t, tt.wantMemory, s.AutoScale.MemoryTarget)
		})
	}
}

func TestParser_DisruptionBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantMin        int
		wantMaxUnavail int
	}{
		{
			name:    "min available",
			input:   "App: t\n\nS:\n  from image x\n  keep at least 2 copies running during updates",
			wantMin: 2,
		},
		{
			name:           "max unavailable",
			input:          "App: t\n\nS:\n  from image x\n  allow at most 1 copies down during updates",
			wantMaxUnavail: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			s := recipe.Slices[0]
			require.NotNil(t, s.DisruptionBudget)
			assert.Equal(t, tt.wantMin, s.DisruptionBudget.MinAvailable)
			assert.Equal(t, tt.wantMaxUnavail, s.DisruptionBudget.MaxUnavailable)
		})
	}
}

func TestParser_SecurityContext(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  run as user 1000
  run as group 1000
  drop all capabilities
  add capability NET_BIND_SERVICE
  read-only filesystem`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Security)
	assert.Equal(t, 1000, s.Security.RunAsUser)
	assert.Equal(t, 1000, s.Security.RunAsGroup)
	assert.Equal(t, []string{"ALL"}, s.Security.DropCapabilities)
	assert.Equal(t, []string{"NET_BIND_SERVICE"}, s.Security.AddCapabilities)
	assert.True(t, s.Security.ReadOnlyRoot)
}

func TestParser_UpdateStrategy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantSpec *UpdateStrategySpec
	}{
		{
			name:     "one at a time",
			input:    "App: t\n\nS:\n  from image x\n  update one at a time",
			wantSpec: &UpdateStrategySpec{MaxSurge: "1", MaxUnavailable: "0"},
		},
		{
			name:     "percentage",
			input:    "App: t\n\nS:\n  from image x\n  update 25% at a time",
			wantSpec: &UpdateStrategySpec{MaxSurge: "25%", MaxUnavailable: "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantSpec, recipe.Slices[0].UpdateStrategy)
		})
	}
}

func TestParser_GracefulShutdown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantSeconds int
	}{
		{
			name:        "seconds",
			input:       "App: t\n\nS:\n  from image x\n  graceful shutdown 60s",
			wantSeconds: 60,
		},
		{
			name:        "minutes",
			input:       "App: t\n\nS:\n  from image x\n  graceful shutdown 2m",
			wantSeconds: 120,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantSeconds, recipe.Slices[0].GracefulShutdown)
		})
	}
}

func TestParser_KindOverride(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  kind StatefulSet`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "StatefulSet", recipe.Slices[0].Kind)
}

func TestParser_DNSName(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  reachable as "my-service.local"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	assert.Equal(t, "my-service.local", recipe.Slices[0].DNSName)
}

func TestParser_MapVerb(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"get", "list", "watch"}, mapVerb("read"))
	assert.Equal(t, []string{"create", "update", "patch"}, mapVerb("write"))
	assert.Equal(t, []string{"delete"}, mapVerb("delete"))
	assert.Equal(t, []string{"get", "list", "watch", "create", "update", "patch", "delete"}, mapVerb("manage"))
	assert.Nil(t, mapVerb("unknown"))
}

func TestParser_SpreadNodes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  spread copies across nodes`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)
	require.NotNil(t, recipe.Slices[0].Scheduling)
	assert.Equal(t, "nodes", recipe.Slices[0].Scheduling.SpreadTopology)
}
