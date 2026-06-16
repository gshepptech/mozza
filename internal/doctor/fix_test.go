package doctor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

func TestAutoFix_HealthCheck(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb},
			{Name: "api", Kind: plan.SliceKindAPI, HealthPath: "/ready"},
			{Name: "worker", Kind: plan.SliceKindWorker},
		},
	}

	findings := []doctor.Finding{
		{Rule: "no-health-check", Fixable: true},
	}

	results := doctor.AutoFix(p, findings)

	// Should fix the web slice but not api (already has health) or worker (wrong kind).
	require.Len(t, results, 1)
	assert.Equal(t, "web", results[0].SliceName)
	assert.Equal(t, "no-health-check", results[0].Rule)
	assert.Equal(t, "/healthz", p.Slices[0].HealthPath)
	assert.Equal(t, "/ready", p.Slices[1].HealthPath) // unchanged
}

func TestAutoFix_ResourceLimits(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb},
			{Name: "api", Kind: plan.SliceKindAPI, Resources: &plan.ResourceSpec{CPULimit: "1"}},
			{Name: "db", Kind: plan.SliceKindDatabase},
		},
	}

	findings := []doctor.Finding{
		{Rule: "no-resource-limits", Fixable: true},
	}

	results := doctor.AutoFix(p, findings)

	// Should fix the web slice but not api (already has limits) or db (skipped kind).
	require.Len(t, results, 1)
	assert.Equal(t, "web", results[0].SliceName)
	assert.Equal(t, "500m", p.Slices[0].Resources.CPULimit)
	assert.Equal(t, "256Mi", p.Slices[0].Resources.MemoryLimit)
	assert.Equal(t, "1", p.Slices[1].Resources.CPULimit) // unchanged
}

func TestAutoFix_NonFixable(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb},
		},
	}

	findings := []doctor.Finding{
		{Rule: "no-health-check", Fixable: false},
		{Rule: "run-as-root", Fixable: false},
	}

	results := doctor.AutoFix(p, findings)

	assert.Empty(t, results)
	assert.Empty(t, p.Slices[0].HealthPath) // unchanged
}

func TestAutoFix_Empty(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{Name: "test"}
	results := doctor.AutoFix(p, nil)
	assert.Empty(t, results)
}

func TestAutoFix_MultipleFixable(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Kind: plan.SliceKindWeb},
			{Name: "api", Kind: plan.SliceKindAPI},
		},
	}

	findings := []doctor.Finding{
		{Rule: "no-health-check", Fixable: true},
		{Rule: "no-resource-limits", Fixable: true},
	}

	results := doctor.AutoFix(p, findings)

	// Should fix both health checks and resource limits on both slices.
	require.Len(t, results, 4)

	// Verify web got both fixes.
	assert.Equal(t, "/healthz", p.Slices[0].HealthPath)
	assert.Equal(t, "500m", p.Slices[0].Resources.CPULimit)

	// Verify api got both fixes.
	assert.Equal(t, "/healthz", p.Slices[1].HealthPath)
	assert.Equal(t, "500m", p.Slices[1].Resources.CPULimit)
}
