package plan_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

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
			// The first usable entry backfills Slice.Port, so a check that stopped
			// there would never look at a second out-of-range entry.
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

	// A single "health check /ready" directive lands in three places (HealthPath
	// plus a readiness and a liveness probe), and the author wrote one line, so
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
