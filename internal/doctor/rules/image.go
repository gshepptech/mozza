package rules

import (
	"fmt"
	"slices"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

// ImageRule checks whether all container images required by the plan are
// available in the local Docker image cache.
type ImageRule struct{}

// Name returns the rule identifier.
func (ImageRule) Name() string { return "image" }

// Evaluate checks image availability for every slice in the plan.
func (ImageRule) Evaluate(p *plan.AppPlan, sig *doctor.Signal) []doctor.Finding {
	var missing []doctor.Finding

	for _, s := range p.Slices {
		if s.Image == "" {
			continue
		}
		if !slices.Contains(sig.AvailableImages, s.Image) {
			missing = append(missing, doctor.Finding{
				Rule:        "image",
				Severity:    doctor.SeverityWarning,
				Message:     fmt.Sprintf("image %q required by slice %q is not available locally", s.Image, s.Name),
				Explanation: fmt.Sprintf("The container image %q is not in your local Docker cache. Deployment will need to pull it from a registry, which may fail if you're offline or the image doesn't exist.", s.Image),
				Fix:         fmt.Sprintf("docker pull %s", s.Image),
			})
		}
	}

	if len(missing) == 0 {
		return []doctor.Finding{{
			Rule:     "image",
			Severity: doctor.SeverityOK,
			Message:  "all required images are available",
		}}
	}

	return missing
}
