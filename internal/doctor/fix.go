package doctor

import (
	"github.com/gshepptech/mozza/internal/plan"
)

// FixResult describes the outcome of an auto-fix attempt.
type FixResult struct {
	// Rule is the rule that produced the original finding.
	Rule string
	// SliceName is the name of the slice that was modified.
	SliceName string
	// Description explains what was changed.
	Description string
}

// AutoFix applies safe, non-destructive fixes to an AppPlan based on fixable
// findings. It returns a list of what was changed. Only findings marked as
// Fixable are acted upon.
func AutoFix(p *plan.AppPlan, findings []Finding) []FixResult {
	var results []FixResult

	for _, f := range findings {
		if !f.Fixable {
			continue
		}

		switch f.Rule {
		case "no-health-check":
			results = append(results, fixHealthCheck(p, f)...)
		case "no-resource-limits":
			results = append(results, fixResourceLimits(p, f)...)
		}
	}

	return results
}

// fixHealthCheck adds a default /healthz health check to web/API/gateway slices
// that are missing one.
func fixHealthCheck(p *plan.AppPlan, _ Finding) []FixResult {
	var results []FixResult

	for i := range p.Slices {
		s := &p.Slices[i]
		if s.Kind != plan.SliceKindWeb && s.Kind != plan.SliceKindAPI && s.Kind != plan.SliceKindGateway {
			continue
		}
		if s.HealthPath != "" || len(s.Probes) > 0 {
			continue
		}
		s.HealthPath = "/healthz"
		results = append(results, FixResult{
			Rule:        "no-health-check",
			SliceName:   s.Name,
			Description: "added default health check path /healthz",
		})
	}

	return results
}

// fixResourceLimits adds default CPU and memory limits to slices that have none.
func fixResourceLimits(p *plan.AppPlan, _ Finding) []FixResult {
	var results []FixResult

	for i := range p.Slices {
		s := &p.Slices[i]
		if s.Kind == plan.SliceKindDatabase || s.Kind == plan.SliceKindCache {
			continue
		}
		if s.Resources != nil && (s.Resources.CPULimit != "" || s.Resources.MemoryLimit != "") {
			continue
		}
		if s.Resources == nil {
			s.Resources = &plan.ResourceSpec{}
		}
		s.Resources.CPULimit = "500m"
		s.Resources.MemoryLimit = "256Mi"
		results = append(results, FixResult{
			Rule:        "no-resource-limits",
			SliceName:   s.Name,
			Description: "added default resource limits: cpu 500m, memory 256Mi",
		})
	}

	return results
}
