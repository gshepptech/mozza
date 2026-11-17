package plan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/plan"
)

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
