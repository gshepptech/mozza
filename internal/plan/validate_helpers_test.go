package plan_test

import (
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
