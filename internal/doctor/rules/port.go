package rules

import (
	"fmt"
	"slices"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

// PortRule checks whether ports required by public slices are already in use
// on the host.
type PortRule struct{}

// Name returns the rule identifier.
func (PortRule) Name() string { return "port" }

// Evaluate checks for port conflicts on public slices that expose a port.
func (PortRule) Evaluate(p *plan.AppPlan, sig *doctor.Signal) []doctor.Finding {
	var conflicts []doctor.Finding

	for _, s := range p.Slices {
		if !s.Public || s.Port == 0 {
			continue
		}
		if slices.Contains(sig.UsedPorts, s.Port) {
			conflicts = append(conflicts, doctor.Finding{
				Rule:        "port",
				Severity:    doctor.SeverityError,
				Message:     fmt.Sprintf("port %d required by slice %q is already in use", s.Port, s.Name),
				Explanation: fmt.Sprintf("Another process is already listening on port %d. Your app will fail to start because it can't bind to a port that's already taken.", s.Port),
				Fix:         fmt.Sprintf("stop the process using port %d or choose a different port", s.Port),
			})
		}
	}

	if len(conflicts) == 0 {
		return []doctor.Finding{{
			Rule:     "port",
			Severity: doctor.SeverityOK,
			Message:  "no port conflicts detected",
		}}
	}

	return conflicts
}
