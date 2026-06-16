// Package rules provides built-in diagnostic rules for the doctor engine.
package rules

import (
	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/plan"
)

// DockerRule checks whether the Docker daemon is reachable.
type DockerRule struct{}

// Name returns the rule identifier.
func (DockerRule) Name() string { return "docker" }

// Evaluate checks Docker reachability from the collected signal.
func (DockerRule) Evaluate(_ *plan.AppPlan, sig *doctor.Signal) []doctor.Finding {
	if sig.DockerReachable {
		return []doctor.Finding{{
			Rule:     "docker",
			Severity: doctor.SeverityOK,
			Message:  "Docker daemon is reachable",
		}}
	}

	msg := "Docker daemon is not reachable"
	if sig.DockerError != nil {
		msg += ": " + sig.DockerError.Error()
	}

	return []doctor.Finding{{
		Rule:        "docker",
		Severity:    doctor.SeverityError,
		Message:     msg,
		Explanation: "Mozza needs Docker to build and run containers. Without a running Docker daemon, no images can be pulled, built, or deployed.",
		Fix:         "ensure the Docker daemon is running and accessible",
	}}
}
