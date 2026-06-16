package rules_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/doctor/rules"
	"github.com/gshepptech/mozza/internal/plan"
)

var emptySig = &doctor.Signal{}

func TestPublicDatabaseWarning_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "public-database", rules.PublicDatabaseWarning{}.Name())
}

func TestPublicDatabaseWarning_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "public database warns",
			slices: []plan.Slice{
				{Name: "db", Kind: plan.SliceKindDatabase, Public: true},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "public cache warns",
			slices: []plan.Slice{
				{Name: "cache", Kind: plan.SliceKindCache, Public: true},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "private database ok",
			slices: []plan.Slice{
				{Name: "db", Kind: plan.SliceKindDatabase, Public: false},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "public web not flagged",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Public: true},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.PublicDatabaseWarning{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestNoHealthCheck_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "no-health-check", rules.NoHealthCheck{}.Name())
}

func TestNoHealthCheck_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "web without health check warns",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "api with health path ok",
			slices: []plan.Slice{
				{Name: "api", Kind: plan.SliceKindAPI, HealthPath: "/healthz"},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "web with probes ok",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Probes: []plan.ProbeSpec{{Type: "readiness", HTTPPath: "/ready"}}},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "worker without health check not flagged",
			slices: []plan.Slice{
				{Name: "worker", Kind: plan.SliceKindWorker},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.NoHealthCheck{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestSingleReplicaProduction_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "single-replica-production", rules.SingleReplicaProduction{}.Name())
}

func TestSingleReplicaProduction_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		namespace    string
		slices       []plan.Slice
		wantFindings int
		wantSeverity doctor.Severity
	}{
		{
			name:      "single replica in prod warns",
			namespace: "production",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 1},
			},
			wantFindings: 1,
			wantSeverity: doctor.SeverityWarning,
		},
		{
			name:      "multiple replicas in prod ok",
			namespace: "production",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 3},
			},
			wantFindings: 1,
			wantSeverity: doctor.SeverityOK,
		},
		{
			name:      "single replica not in prod ignored",
			namespace: "staging",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 1},
			},
			wantFindings: 0,
		},
		{
			name:      "zero replicas in prod warns",
			namespace: "prod",
			slices: []plan.Slice{
				{Name: "api", Kind: plan.SliceKindAPI, Replicas: 0},
			},
			wantFindings: 1,
			wantSeverity: doctor.SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Namespace: tt.namespace, Slices: tt.slices}
			findings := rules.SingleReplicaProduction{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			if tt.wantFindings > 0 {
				assert.Equal(t, tt.wantSeverity, findings[0].Severity)
			}
		})
	}
}

func TestNoResourceLimits_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "no-resource-limits", rules.NoResourceLimits{}.Name())
}

func TestNoResourceLimits_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "no limits warns",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "with limits ok",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Resources: &plan.ResourceSpec{CPULimit: "500m"}},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "database skipped",
			slices: []plan.Slice{
				{Name: "db", Kind: plan.SliceKindDatabase},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.NoResourceLimits{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestNoAutoScaleWithHighReplicas_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "no-autoscale-high-replicas", rules.NoAutoScaleWithHighReplicas{}.Name())
}

func TestNoAutoScaleWithHighReplicas_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "high replicas no autoscale suggests",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 5},
			},
			wantSeverity: doctor.SeverityInfo,
			wantFindings: 1,
		},
		{
			name: "high replicas with autoscale ok",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 5, AutoScale: &plan.AutoScaleSpec{MinReplicas: 5, MaxReplicas: 10}},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "low replicas not flagged",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 2},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.NoAutoScaleWithHighReplicas{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestRunAsRoot_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "run-as-root", rules.RunAsRoot{}.Name())
}

func TestRunAsRoot_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "no security context warns",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "user 0 warns",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Security: &plan.SecuritySpec{RunAsUser: 0}},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "non-root user ok",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Security: &plan.SecuritySpec{RunAsUser: 1000}},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "database skipped",
			slices: []plan.Slice{
				{Name: "db", Kind: plan.SliceKindDatabase},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.RunAsRoot{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestNoGracefulShutdown_Name(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "no-graceful-shutdown", rules.NoGracefulShutdown{}.Name())
}

func TestNoGracefulShutdown_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		wantSeverity doctor.Severity
		wantFindings int
	}{
		{
			name: "multi-replica web no shutdown warns",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 3},
			},
			wantSeverity: doctor.SeverityWarning,
			wantFindings: 1,
		},
		{
			name: "multi-replica web with shutdown ok",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 3, GracefulShutdown: 30},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "single replica not flagged",
			slices: []plan.Slice{
				{Name: "web", Kind: plan.SliceKindWeb, Replicas: 1},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
		{
			name: "worker not flagged",
			slices: []plan.Slice{
				{Name: "worker", Kind: plan.SliceKindWorker, Replicas: 3},
			},
			wantSeverity: doctor.SeverityOK,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			findings := rules.NoGracefulShutdown{}.Evaluate(p, emptySig)
			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}
