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
			wantErr: "port is -1 — a port must be between 1 and 65535",
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
			wantErr: "port is 70000 — a port must be between 1 and 65535",
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
			name: "invalid env key leading digit",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Env = map[string]string{"1BAD": "postgres://localhost/db"}
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

func TestValidate_PortRanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		{
			name: "slice port zero means no port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
			},
			wantErr: "",
		},
		{
			name: "slice port at the bottom of the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 1
			},
			wantErr: "",
		},
		{
			name: "slice port at the top of the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 65535
			},
			wantErr: "",
		},
		{
			name: "slice port one above the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 65536
			},
			wantErr: `slice[0] "web": port is 65536 — a port must be between 1 and 65535`,
		},
		{
			name: "slice port negative",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = -1
			},
			wantErr: `slice[0] "web": port is -1 — a port must be between 1 and 65535`,
		},
		{
			name: "named port zero is not a sentinel",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: 0}}
			},
			wantErr: `slice[0] "web": port "http" is 0 — a port must be between 1 and 65535`,
		},
		{
			name: "unnamed port entry is reported by position",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{{Port: 0}}
			},
			wantErr: `slice[0] "web": port[0] is 0 — a port must be between 1 and 65535`,
		},
		{
			name: "named port negative",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: -1}}
			},
			wantErr: `slice[0] "web": port "http" is -1 — a port must be between 1 and 65535`,
		},
		{
			name: "named ports at both ends of the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: 1}, {Name: "grpc", Port: 65535}}
			},
			wantErr: "",
		},
		{
			// The first usable entry backfills Slice.Port, so before this rule
			// existed a second out-of-range entry was never looked at.
			name: "second named port one above the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: 8080}, {Name: "grpc", Port: 65536}}
			},
			wantErr: `slice[0] "web": port "grpc" is 65536 — a port must be between 1 and 65535`,
		},
		{
			name: "sidecar port zero",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Sidecars = []plan.Sidecar{{Name: "envoy", Image: "envoy:1.26", Ports: []plan.PortSpec{{Port: 0}}}}
			},
			wantErr: `slice[0] "web": sidecar "envoy" port[0] is 0 — a port must be between 1 and 65535`,
		},
		{
			name: "unnamed sidecar with a named port above the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Sidecars = []plan.Sidecar{{Image: "envoy:1.26", Ports: []plan.PortSpec{{Name: "metrics", Port: 65536}}}}
			},
			wantErr: `slice[0] "web": sidecar[0] port "metrics" is 65536 — a port must be between 1 and 65535`,
		},
		{
			name: "sidecar port negative",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Sidecars = []plan.Sidecar{{Name: "envoy", Image: "envoy:1.26", Ports: []plan.PortSpec{{Port: -1}}}}
			},
			wantErr: `slice[0] "web": sidecar "envoy" port[0] is -1 — a port must be between 1 and 65535`,
		},
		{
			name: "sidecar ports at both ends of the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Sidecars = []plan.Sidecar{{Name: "envoy", Image: "envoy:1.26", Ports: []plan.PortSpec{{Port: 1}, {Port: 65535}}}}
			},
			wantErr: "",
		},
		{
			name: "probe port zero means inherit the slice port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready", TCPPort: 0}}
			},
			wantErr: "",
		},
		{
			name: "probe ports at both ends of the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", TCPPort: 1}, {Type: "liveness", TCPPort: 65535}}
			},
			wantErr: "",
		},
		{
			name: "probe port one above the range",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", TCPPort: 65536}}
			},
			wantErr: `slice[0] "web": readiness check's port is 65536 — a port must be between 1 and 65535`,
		},
		{
			name: "probe port negative",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready", TCPPort: -1}}
			},
			wantErr: `slice[0] "web": readiness check's port is -1 — a port must be between 1 and 65535`,
		},
		{
			name: "typeless probe port still names a subject",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{TCPPort: 65536}}
			},
			wantErr: `slice[0] "web": this probe's port is 65536 — a port must be between 1 and 65535`,
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

func TestValidate_RejectedSlicePortSilencesUnaimedHealthCheck(t *testing.T) {
	t.Parallel()

	// The slice declared a port and it was thrown out, so PrimaryPort resolves to
	// nothing for a second reason. Telling the author their health check has no
	// port to check restates the port they already know is wrong.
	p := validPlan()
	p.Slices[0].Port = 0
	p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: 0}}
	p.Slices[0].HealthPath = "/healthz"

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `port "http" is 0 — a port must be between 1 and 65535`)
	assert.NotContains(t, err.Error(), "has no port to check")
}

func TestValidate_UnrelatedBadPortKeepsUnaimedHealthCheck(t *testing.T) {
	t.Parallel()

	// A sidecar port is not what a health check aims at, so rejecting one must not
	// swallow the genuine complaint that this slice declares no port of its own.
	p := validPlan()
	p.Slices[0].Port = 0
	p.Slices[0].HealthPath = "/healthz"
	p.Slices[0].Sidecars = []plan.Sidecar{
		{Name: "logger", Image: "fluentbit:2.0", Ports: []plan.PortSpec{{Port: 70000}}},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `sidecar "logger" port[0] is 70000 — a port must be between 1 and 65535`)
	assert.Contains(t, err.Error(), `health check "/healthz" has no port to check`)
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
		{name: "all lowercase", key: "port", wantErr: false},
		{name: "mixed case", key: "Database_Url", wantErr: false},
		{name: "contains hyphen", key: "API-KEY", wantErr: false},
		{name: "starts with underscore", key: "_SECRET", wantErr: false},
		{name: "contains dot", key: "MY_ENV.NAME", wantErr: false},
		{name: "lowercase with dot and hyphen", key: "my.env-name", wantErr: false},
		{name: "ghost nested key", key: "database__connection__user", wantErr: false},
		{name: "gitea nested key", key: "GITEA__database__HOST", wantErr: false},
		{name: "starts with digit", key: "3PO", wantErr: true},
		{name: "contains space", key: "MY VAR", wantErr: true},
		{name: "empty key", key: "", wantErr: true},
		{name: "leading digit with uppercase", key: "1PASSWORD", wantErr: true},
		{name: "leading digit with letters", key: "1BAD", wantErr: true},
		{name: "current directory", key: ".", wantErr: true},
		{name: "parent directory", key: "..", wantErr: true},
		{name: "parent directory prefix", key: "..foo", wantErr: true},
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
				assert.NotContains(t, err.Error(), "^", "the rule must be stated in words, not as a regex")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidate_ProbeTargets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modify  func(p *plan.AppPlan)
		wantErr string
	}{
		{
			name: "http check with no port to resolve",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}}
			},
			wantErr: `health check "/ready" has no port to check`,
		},
		{
			name: "legacy health path with no port to resolve",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].HealthPath = "/healthz"
			},
			wantErr: `health check "/healthz" has no port to check`,
		},
		{
			name: "http check inherits the slice port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}}
			},
			wantErr: "",
		},
		{
			name: "http check inherits a named port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].Ports = []plan.PortSpec{{Name: "http", Port: 3000}}
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}}
			},
			wantErr: "",
		},
		{
			name: "http check naming its own port needs no slice port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready", TCPPort: 9000}}
			},
			wantErr: "",
		},
		{
			name: "tcp check needs no slice port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "liveness", TCPPort: 5432}}
			},
			wantErr: "",
		},
		{
			name: "command check needs no slice port",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Port = 0
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "liveness", Command: "pg_isready"}}
			},
			wantErr: "",
		},
		{
			name: "check with nothing to check",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{Type: "readiness"}}
			},
			wantErr: "readiness check must say what to check — give it an HTTP path, a TCP port, or a command",
		},
		{
			name: "check with nothing to check and no type still names a subject",
			modify: func(p *plan.AppPlan) {
				p.Slices[0].Probes = []plan.ProbeSpec{{}}
			},
			wantErr: "this probe must say what to check — give it an HTTP path, a TCP port, or a command",
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

func TestValidate_UnaimedHealthCheckReportedOnce(t *testing.T) {
	t.Parallel()

	// A single "health check /ready" directive lands in three places — HealthPath
	// plus a readiness and a liveness probe — and the author wrote one line, so
	// they should be told about it once.
	p := validPlan()
	p.Slices[0].Port = 0
	p.Slices[0].HealthPath = "/ready"
	p.Slices[0].Probes = []plan.ProbeSpec{
		{Type: "readiness", HTTPPath: "/ready"},
		{Type: "liveness", HTTPPath: "/ready"},
	}

	err := plan.Validate(p)

	require.Error(t, err)
	assert.Equal(t, 1, strings.Count(err.Error(), "has no port to check"))
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
