package recipe

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_ScheduleDirectives(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantCron string
	}{
		{
			name:     "daily at hour",
			input:    "App: t\n\nS:\n  from image x\n  run every day at 2am",
			wantCron: "0 2 * * *",
		},
		{
			name:     "every hour",
			input:    "App: t\n\nS:\n  from image x\n  run every hour",
			wantCron: "0 * * * *",
		},
		{
			name:     "every N minutes",
			input:    "App: t\n\nS:\n  from image x\n  run every 5 minutes",
			wantCron: "*/5 * * * *",
		},
		{
			name:     "weekday",
			input:    "App: t\n\nS:\n  from image x\n  run every monday at 9am",
			wantCron: "0 9 * * 1",
		},
		{
			name:     "raw cron",
			input:    "App: t\n\nS:\n  from image x\n  schedule \"15 3 * * 1-5\"",
			wantCron: "15 3 * * 1-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			parser := NewParser(tt.input)
			recipe, err := parser.Parse()
			require.NoError(t, err)
			require.Len(t, recipe.Slices, 1)
			assert.Equal(t, tt.wantCron, recipe.Slices[0].Schedule)
		})
	}
}

func TestParser_RunOnce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		input           string
		wantRunOnce     bool
		wantParallelism int
		wantRetries     int
	}{
		{
			name:        "run once",
			input:       "App: t\n\nJ:\n  from image x\n  run once",
			wantRunOnce: true,
		},
		{
			name:        "run to completion",
			input:       "App: t\n\nJ:\n  from image x\n  run to completion",
			wantRunOnce: true,
		},
		{
			name:            "run once with parallel",
			input:           "App: t\n\nJ:\n  from image x\n  run once with 4 parallel",
			wantRunOnce:     true,
			wantParallelism: 4,
		},
		{
			name:        "run once with retries",
			input:       "App: t\n\nJ:\n  from image x\n  run once, retry up to 3 times",
			wantRunOnce: true,
			wantRetries: 3,
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
			assert.Equal(t, tt.wantRunOnce, s.RunOnce)
			assert.Equal(t, tt.wantParallelism, s.Parallelism)
			assert.Equal(t, tt.wantRetries, s.Retries)
		})
	}
}

func TestParser_DaemonMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantDaemon     bool
		wantNodeLabels int
	}{
		{
			name:       "run on every node",
			input:      "App: t\n\nD:\n  from image x\n  run on every node",
			wantDaemon: true,
		},
		{
			name:           "run on every node labeled",
			input:          "App: t\n\nD:\n  from image x\n  run on every node labeled \"tier=edge\"",
			wantDaemon:     true,
			wantNodeLabels: 1,
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
			assert.Equal(t, tt.wantDaemon, s.DaemonMode)
			if tt.wantNodeLabels > 0 {
				require.NotNil(t, s.Scheduling)
				assert.Len(t, s.Scheduling.NodeRequirements, tt.wantNodeLabels)
				assert.Equal(t, "tier", s.Scheduling.NodeRequirements[0].Key)
				assert.Equal(t, "edge", s.Scheduling.NodeRequirements[0].Value)
			}
		})
	}
}

func TestParser_Stateful(t *testing.T) {
	t.Parallel()

	input := `App: t

Store:
  from image myorg/store:1.0
  each copy needs its own storage of 10Gi
  start copies in order
  allow copies to find each other`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Equal(t, "10Gi", s.StatefulStorage)
	assert.True(t, s.OrderedStartup)
	assert.True(t, s.PeerDiscovery)
}

// --- Container and config directive tests ---

func TestParser_InitBlock(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/app:1.0
  before starting:
    run image myorg/migrate:1.0 with "migrate up"
    run image myorg/init:1.0
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.InitSteps, 2)
	assert.Equal(t, "myorg/migrate:1.0", s.InitSteps[0].Image)
	assert.Equal(t, "migrate up", s.InitSteps[0].Command)
	assert.Equal(t, "myorg/init:1.0", s.InitSteps[1].Image)
	assert.Empty(t, s.InitSteps[1].Command)
	assert.Equal(t, 8080, s.Port)
}

func TestParser_Sidecar(t *testing.T) {
	t.Parallel()

	input := `App: t

Web:
  from image myorg/web:1.0
  with sidecar envoy from myorg/envoy:1.26 on port 9901`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Sidecars, 1)
	assert.Equal(t, "envoy", s.Sidecars[0].Name)
	assert.Equal(t, "myorg/envoy:1.26", s.Sidecars[0].Image)
	require.Len(t, s.Sidecars[0].Ports, 1)
	assert.Equal(t, 9901, s.Sidecars[0].Ports[0].Port)
}

func TestParser_Mounts(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  mount file "config/app.yaml" at /etc/app/config.yaml
  mount secret "db-creds" at /etc/secrets/db
  mount config "configs/" at /etc/app/configs`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Mounts, 3)

	assert.Equal(t, MountSpec{Type: "file", Source: "config/app.yaml", Target: "/etc/app/config.yaml"}, s.Mounts[0])
	assert.Equal(t, MountSpec{Type: "secret", Source: "db-creds", Target: "/etc/secrets/db"}, s.Mounts[1])
	assert.Equal(t, MountSpec{Type: "config-dir", Source: "configs/", Target: "/etc/app/configs"}, s.Mounts[2])
}

func TestParser_ProbeTypes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  readiness check /ready
  liveness check /alive
  startup check /started
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 3)
	assert.Equal(t, ProbeSpec{Type: "readiness", HTTPPath: "/ready"}, s.Probes[0])
	assert.Equal(t, ProbeSpec{Type: "liveness", HTTPPath: "/alive"}, s.Probes[1])
	assert.Equal(t, ProbeSpec{Type: "startup", HTTPPath: "/started"}, s.Probes[2])
}

func TestParser_ExecAndTCPProbes(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  readiness check by running "cat /tmp/healthy"
  liveness check on tcp port 3306`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 2)
	assert.Equal(t, "readiness", s.Probes[0].Type)
	assert.Equal(t, "cat /tmp/healthy", s.Probes[0].Command)
	assert.Equal(t, "liveness", s.Probes[1].Type)
	assert.Equal(t, 3306, s.Probes[1].TCPPort)
}

func TestParser_HealthCheckBackwardCompat(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  health check /healthz
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	assert.Equal(t, "/healthz", s.Health)
	require.Len(t, s.Probes, 2)
	assert.Equal(t, "readiness", s.Probes[0].Type)
	assert.Equal(t, "/healthz", s.Probes[0].HTTPPath)
	assert.Equal(t, "liveness", s.Probes[1].Type)
	assert.Equal(t, "/healthz", s.Probes[1].HTTPPath)
}

func TestParser_HealthCheckWithTiming(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  health check /healthz every 10s, timeout 5s, wait 30s before starting
  on port 8080`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.Len(t, s.Probes, 2)
	for _, probe := range s.Probes {
		assert.Equal(t, "/healthz", probe.HTTPPath)
		assert.Equal(t, 10, probe.Interval)
		assert.Equal(t, 5, probe.Timeout)
		assert.Equal(t, 30, probe.Delay)
	}
}

// --- Operational directive tests ---

func TestParser_Lifecycle(t *testing.T) {
	t.Parallel()

	input := `App: t

Svc:
  from image myorg/svc:1.0
  before stopping, wait 30s
  before stopping, run "kill -SIGTERM 1"
  after starting, run "/hooks/warmup.sh"`

	parser := NewParser(input)
	recipe, err := parser.Parse()

	require.NoError(t, err)
	require.Len(t, recipe.Slices, 1)

	s := recipe.Slices[0]
	require.NotNil(t, s.Lifecycle)
	assert.Equal(t, 30, s.Lifecycle.PreStopWait)
	assert.Equal(t, "kill -SIGTERM 1", s.Lifecycle.PreStopCommand)
	assert.Equal(t, "/hooks/warmup.sh", s.Lifecycle.PostStartCommand)
}

func TestParser_PlainEnglishToCron(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantCron string
		wantErr  bool
	}{
		{"day at 2am", "0 2 * * *", false},
		{"day at 2pm", "0 14 * * *", false},
		{"hour", "0 * * * *", false},
		{"5 minutes", "*/5 * * * *", false},
		{"monday at 9am", "0 9 * * 1", false},
		{"friday at 5pm", "0 17 * * 5", false},
		{"garbage text", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			cron, err := plainEnglishToCron(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCron, cron)
			}
		})
	}
}
