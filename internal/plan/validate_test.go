package plan_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

// validPlan returns a minimal valid AppPlan suitable for tests that need a
// known-good starting point.
func validPlan() *plan.AppPlan {
	return &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{
				Name:     "web",
				Kind:     plan.SliceKindWeb,
				Image:    "nginx:latest",
				Port:     8080,
				Replicas: 1,
			},
		},
	}
}

func TestValidate_ValidPlan(t *testing.T) {
	t.Parallel()

	err := plan.Validate(validPlan())

	assert.NoError(t, err)
}

func TestValidate_EmptyName(t *testing.T) {
	t.Parallel()

	p := validPlan()
	p.Name = ""

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "plan name must not be empty")
}

func TestValidate_NoSlices(t *testing.T) {
	t.Parallel()

	p := validPlan()
	p.Slices = nil

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one slice")
}

func TestValidate_SliceRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		{
			name: "empty slice name",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = ""
			},
			wantErr: "name must not be empty",
		},
		{
			name: "empty image",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Image = ""
			},
			wantErr: "image must not be empty",
		},
		{
			name: "negative port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = -1
			},
			wantErr: "port must be 0-65535",
		},
		{
			name: "negative replicas",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Replicas = -3
			},
			wantErr: "replicas must be >= 0",
		},
		{
			name: "zero port is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
			},
			wantErr: "",
		},
		{
			name: "zero replicas is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Replicas = 0
			},
			wantErr: "",
		},
		{
			name: "port exceeds 65535",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 70000
			},
			wantErr: "port must be 0-65535",
		},
		{
			name: "invalid slice name format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = "My_Slice!"
			},
			wantErr: "DNS-compatible",
		},
		{
			name: "slice name too long",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Name = strings.Repeat("a", 64)
			},
			wantErr: "DNS-compatible",
		},
		{
			name: "image contains whitespace",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Image = "my image:latest"
			},
			wantErr: "must not contain whitespace",
		},
		{
			name: "health path without leading slash",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "healthz"
			},
			wantErr: "must start with /",
		},
		{
			name: "valid health path",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/healthz"
			},
			wantErr: "",
		},
		{
			name: "valid nested health path",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/api/v1/health"
			},
			wantErr: "",
		},
		{
			name: "health path with whitespace",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health check"
			},
			wantErr: "must start with /",
		},
		{
			name: "health path with shell metacharacters",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health;echo pwned"
			},
			wantErr: "must start with /",
		},
		{
			name: "health path with newline",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].HealthPath = "/health\nhttp://evil.com"
			},
			wantErr: "must start with /",
		},
		{
			name: "invalid env key lowercase",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Env = map[string]string{"database_url": "postgres://localhost/db"}
			},
			wantErr: "env key",
		},
		{
			name: "valid env key",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Env = map[string]string{"DATABASE_URL": "postgres://localhost/db"}
			},
			wantErr: "",
		},
		{
			name: "invalid domain",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Domain = "not a valid domain!"
			},
			wantErr: "is not a valid domain name",
		},
		{
			name: "valid domain",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Domain = "api.example.com"
			},
			wantErr: "",
		},
		{
			name: "invalid restart policy",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].RestartPolicy = "never"
			},
			wantErr: "restart policy",
		},
		{
			name: "valid restart policy",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].RestartPolicy = "always"
			},
			wantErr: "",
		},
		{
			name: "invalid cpu limit format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{CPULimit: "five-hundred"}
			},
			wantErr: "cpu limit",
		},
		{
			name: "valid cpu limit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{CPULimit: "500m"}
			},
			wantErr: "",
		},
		{
			name: "invalid memory limit format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{MemoryLimit: "two-gigs"}
			},
			wantErr: "memory limit",
		},
		{
			name: "valid memory limit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Resources = &plan.ResourceSpec{MemoryLimit: "256Mi"}
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPlan()
			tt.modify(p)

			err := plan.Validate(p)

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidate_DuplicateSliceNames(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb, Image: "nginx:latest"},
			{Name: "web", Kind: plan.SliceKindWorker, Image: "worker:latest"},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate name")
	assert.Contains(t, err.Error(), `"web"`)
}

func TestValidate_NeedsUnknownSlice(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{
				Name:  "web",
				Kind:  plan.SliceKindWeb,
				Image: "nginx:latest",
				Needs: []string{"nonexistent"},
			},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "needs unknown slice")
	assert.Contains(t, err.Error(), `"nonexistent"`)
}

func TestValidate_CircularDependency(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		slices []plan.Slice
	}{
		{
			name: "direct cycle A->B->A",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"b"}},
				{Name: "b", Kind: plan.SliceKindWeb, Image: "img:2", Needs: []string{"a"}},
			},
		},
		{
			name: "self reference",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"a"}},
			},
		},
		{
			name: "three node cycle A->B->C->A",
			slices: []plan.Slice{
				{Name: "a", Kind: plan.SliceKindWeb, Image: "img:1", Needs: []string{"b"}},
				{Name: "b", Kind: plan.SliceKindWeb, Image: "img:2", Needs: []string{"c"}},
				{Name: "c", Kind: plan.SliceKindWeb, Image: "img:3", Needs: []string{"a"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name:   "myapp",
				Slices: tt.slices,
			}

			err := plan.Validate(p)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "circular dependency")
		})
	}
}

func TestValidate_ValidDependencyChain(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "myapp",
		Slices: []plan.Slice{
			{Name: "db", Kind: plan.SliceKindDatabase, Image: "postgres:16"},
			{Name: "cache", Kind: plan.SliceKindCache, Image: "redis:7"},
			{Name: "api", Kind: plan.SliceKindWeb, Image: "api:latest", Needs: []string{"db", "cache"}},
			{Name: "worker", Kind: plan.SliceKindWorker, Image: "worker:latest", Needs: []string{"db"}},
		},
	}

	err := plan.Validate(p)

	assert.NoError(t, err)
}

func TestValidate_MultipleErrorsCollected(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "",
		Slices: []plan.Slice{
			{Name: "", Kind: plan.SliceKindWeb, Image: ""},
		},
	}

	err := plan.Validate(p)

	require.Error(t, err)

	msg := err.Error()
	// Count distinct error messages; expect at least 3:
	// plan name empty, slice name empty, image empty.
	parts := strings.Split(msg, "\n")
	assert.GreaterOrEqual(t, len(parts), 3, "expected at least 3 collected errors, got: %s", msg)
}

func TestValidate_EnvKeyFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{name: "uppercase only", key: "PORT", wantErr: false},
		{name: "uppercase with digits", key: "API_V2", wantErr: false},
		{name: "uppercase with underscores", key: "DATABASE_URL", wantErr: false},
		{name: "single letter", key: "X", wantErr: false},
		{name: "starts with digit", key: "3PO", wantErr: true},
		{name: "all lowercase", key: "port", wantErr: true},
		{name: "mixed case", key: "Database_Url", wantErr: true},
		{name: "contains hyphen", key: "API-KEY", wantErr: true},
		{name: "contains space", key: "MY VAR", wantErr: true},
		{name: "starts with underscore", key: "_SECRET", wantErr: true},
		{name: "empty key", key: "", wantErr: true},
		{name: "leading digit with uppercase", key: "1PASSWORD", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{
				Name: "myapp",
				Slices: []plan.Slice{
					{
						Name:  "web",
						Kind:  plan.SliceKindWeb,
						Image: "nginx:latest",
						Port:  8080,
						Env:   map[string]string{tt.key: "value"},
					},
				},
			}

			err := plan.Validate(p)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "env key")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_KindConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		{
			name: "scheduled without schedule",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindScheduled
			},
			wantErr: "scheduled slice requires a schedule",
		},
		{
			name: "scheduled with schedule is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindScheduled
				p.Slices[0].Schedule = "0 3 * * *"
			},
			wantErr: "",
		},
		{
			name: "task without run-once",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindTask
			},
			wantErr: "task slice requires run-once",
		},
		{
			name: "task with run-once is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindTask
				p.Slices[0].RunOnce = true
			},
			wantErr: "",
		},
		{
			name: "daemon with replicas",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindDaemon
				p.Slices[0].Replicas = 3
			},
			wantErr: "daemon slice runs on every node — replicas is not applicable",
		},
		{
			name: "daemon without replicas is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindDaemon
				p.Slices[0].Replicas = 0
			},
			wantErr: "",
		},
		{
			name: "daemon with autoscale",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindDaemon
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{MinReplicas: 1, MaxReplicas: 5, CPUTarget: 80}
			},
			wantErr: "daemon slice runs on every node — auto-scaling is not applicable",
		},
		{
			name: "stateful without features",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindStateful
			},
			wantErr: "stateful slice needs at least one stateful feature",
		},
		{
			name: "stateful with storage is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindStateful
				p.Slices[0].StatefulStorage = "10Gi"
			},
			wantErr: "",
		},
		{
			name: "stateful with ordered startup is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindStateful
				p.Slices[0].OrderedStartup = true
			},
			wantErr: "",
		},
		{
			name: "duplicate port names",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{
					{Name: "http", Port: 8080},
					{Name: "http", Port: 9090},
				}
			},
			wantErr: "duplicate port name",
		},
		{
			name: "unique port names is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{
					{Name: "http", Port: 8080},
					{Name: "grpc", Port: 9090},
				}
			},
			wantErr: "",
		},
		{
			name: "init step empty image",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].InitSteps = []plan.InitStep{
					{Image: "", Command: "echo hi"},
				}
			},
			wantErr: "init step[0] image must not be empty",
		},
		{
			name: "init step with image is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].InitSteps = []plan.InitStep{
					{Image: "busybox:latest", Command: "echo hi"},
				}
			},
			wantErr: "",
		},
		{
			name: "duplicate sidecar names",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Sidecars = []plan.Sidecar{
					{Name: "proxy", Image: "envoy:latest"},
					{Name: "proxy", Image: "nginx:latest"},
				}
			},
			wantErr: "duplicate sidecar name",
		},
		{
			name: "invalid schedule format",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindScheduled
				p.Slices[0].Schedule = "every day"
			},
			wantErr: "invalid schedule",
		},
		{
			name: "valid cron schedule",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindScheduled
				p.Slices[0].Schedule = "0 3 * * *"
			},
			wantErr: "",
		},
		{
			name: "mount target not absolute",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Mounts = []plan.MountSpec{
					{Type: "configmap", Source: "cfg", Target: "etc/config"},
				}
			},
			wantErr: "must be an absolute path",
		},
		{
			name: "mount target absolute is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Mounts = []plan.MountSpec{
					{Type: "configmap", Source: "cfg", Target: "/etc/config"},
				}
			},
			wantErr: "",
		},
		{
			name: "network policy unknown source",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].NetworkPolicy = &plan.NetworkPolicySpec{
					AllowFrom: []string{"nonexistent"},
				}
			},
			wantErr: "unknown source",
		},
		{
			name: "network policy valid source",
			modify: func(p *plan.AppPlan) {
				p.Slices = append(p.Slices, plan.Slice{
					Name:  "backend",
					Kind:  plan.SliceKindAPI,
					Image: "api:latest",
				})
				p.Slices[0].NetworkPolicy = &plan.NetworkPolicySpec{
					AllowFrom: []string{"backend"},
				}
			},
			wantErr: "",
		},
		{
			name: "autoscale min less than 1",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas: 0,
					MaxReplicas: 5,
					CPUTarget:   80,
				}
			},
			wantErr: "auto-scale min replicas must be > 0",
		},
		{
			name: "autoscale max less than min",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas: 5,
					MaxReplicas: 3,
					CPUTarget:   80,
				}
			},
			wantErr: "auto-scale max must be >= min",
		},
		{
			name: "autoscale cpu target out of range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas: 1,
					MaxReplicas: 5,
					CPUTarget:   150,
				}
			},
			wantErr: "auto-scale CPU target must be 1-100",
		},
		{
			name: "autoscale memory target out of range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas:  1,
					MaxReplicas:  5,
					MemoryTarget: 0,
				}
			},
			wantErr: "", // 0 means not set, which is valid
		},
		{
			name: "autoscale memory target 101",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas:  1,
					MaxReplicas:  5,
					MemoryTarget: 101,
				}
			},
			wantErr: "auto-scale memory target must be 1-100",
		},
		{
			name: "autoscale valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].AutoScale = &plan.AutoScaleSpec{
					MinReplicas: 2,
					MaxReplicas: 10,
					CPUTarget:   80,
				}
			},
			wantErr: "",
		},
		{
			name: "disruption budget on daemon",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Kind = plan.SliceKindDaemon
				p.Slices[0].DisruptionBudget = &plan.DisruptionBudgetSpec{MinAvailable: 1}
			},
			wantErr: "disruption budget requires a workload with replicas",
		},
		{
			name: "disruption budget on web is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].DisruptionBudget = &plan.DisruptionBudgetSpec{MinAvailable: 1}
			},
			wantErr: "",
		},
		{
			name: "negative graceful shutdown",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].GracefulShutdown = -5
			},
			wantErr: "graceful shutdown must be > 0",
		},
		{
			name: "zero graceful shutdown is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].GracefulShutdown = 0
			},
			wantErr: "",
		},
		{
			name: "positive graceful shutdown is valid",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].GracefulShutdown = 30
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validPlan()
			tt.modify(p)

			err := plan.Validate(p)

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}
