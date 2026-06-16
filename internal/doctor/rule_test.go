package doctor_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/doctor/rules"
	"github.com/gshepptech/mozza/internal/plan"
)

func TestDockerRule_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "docker", rules.DockerRule{}.Name())
}

func TestDockerRule_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		signal       *doctor.Signal
		wantSeverity doctor.Severity
		wantContains string
	}{
		{
			name:         "reachable",
			signal:       &doctor.Signal{DockerReachable: true},
			wantSeverity: doctor.SeverityOK,
			wantContains: "reachable",
		},
		{
			name:         "not reachable without error",
			signal:       &doctor.Signal{DockerReachable: false},
			wantSeverity: doctor.SeverityError,
			wantContains: "not reachable",
		},
		{
			name: "not reachable with error",
			signal: &doctor.Signal{
				DockerReachable: false,
				DockerError:     errors.New("connection refused"),
			},
			wantSeverity: doctor.SeverityError,
			wantContains: "connection refused",
		},
	}

	p := &plan.AppPlan{Name: "test"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			findings := rules.DockerRule{}.Evaluate(p, tt.signal)

			require.Len(t, findings, 1)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
			assert.Contains(t, findings[0].Message, tt.wantContains)
			assert.Equal(t, "docker", findings[0].Rule)
		})
	}
}

func TestImageRule_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "image", rules.ImageRule{}.Name())
}

func TestImageRule_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		slices        []plan.Slice
		available     []string
		wantFindings  int
		wantSeverity  doctor.Severity
		wantOKMessage string
	}{
		{
			name: "all present",
			slices: []plan.Slice{
				{Name: "web", Image: "nginx:latest"},
				{Name: "db", Image: "postgres:16"},
			},
			available:     []string{"nginx:latest", "postgres:16"},
			wantFindings:  1,
			wantSeverity:  doctor.SeverityOK,
			wantOKMessage: "all required images are available",
		},
		{
			name: "some missing",
			slices: []plan.Slice{
				{Name: "web", Image: "nginx:latest"},
				{Name: "db", Image: "postgres:16"},
			},
			available:    []string{"nginx:latest"},
			wantFindings: 1,
			wantSeverity: doctor.SeverityWarning,
		},
		{
			name: "none present",
			slices: []plan.Slice{
				{Name: "web", Image: "nginx:latest"},
				{Name: "db", Image: "postgres:16"},
				{Name: "cache", Image: "redis:7"},
			},
			available:    nil,
			wantFindings: 3,
			wantSeverity: doctor.SeverityWarning,
		},
		{
			name:          "no slices",
			slices:        nil,
			available:     nil,
			wantFindings:  1,
			wantSeverity:  doctor.SeverityOK,
			wantOKMessage: "all required images are available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			sig := &doctor.Signal{AvailableImages: tt.available}

			findings := rules.ImageRule{}.Evaluate(p, sig)

			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
			if tt.wantOKMessage != "" {
				assert.Equal(t, tt.wantOKMessage, findings[0].Message)
			}
		})
	}
}

func TestImageRule_Evaluate_FixSuggestion(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Image: "nginx:latest"},
		},
	}
	sig := &doctor.Signal{AvailableImages: nil}

	findings := rules.ImageRule{}.Evaluate(p, sig)

	require.Len(t, findings, 1)
	assert.Equal(t, "docker pull nginx:latest", findings[0].Fix)
}

func TestPortRule_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "port", rules.PortRule{}.Name())
}

func TestPortRule_Evaluate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		slices       []plan.Slice
		usedPorts    []int
		wantFindings int
		wantSeverity doctor.Severity
	}{
		{
			name: "no conflicts",
			slices: []plan.Slice{
				{Name: "web", Port: 8080, Public: true},
			},
			usedPorts:    []int{3000, 5432},
			wantFindings: 1,
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "one conflict",
			slices: []plan.Slice{
				{Name: "web", Port: 8080, Public: true},
				{Name: "api", Port: 3000, Public: true},
			},
			usedPorts:    []int{8080},
			wantFindings: 1,
			wantSeverity: doctor.SeverityError,
		},
		{
			name: "multiple conflicts",
			slices: []plan.Slice{
				{Name: "web", Port: 8080, Public: true},
				{Name: "api", Port: 3000, Public: true},
			},
			usedPorts:    []int{8080, 3000},
			wantFindings: 2,
			wantSeverity: doctor.SeverityError,
		},
		{
			name: "non-public slice ignored",
			slices: []plan.Slice{
				{Name: "worker", Port: 8080, Public: false},
			},
			usedPorts:    []int{8080},
			wantFindings: 1,
			wantSeverity: doctor.SeverityOK,
		},
		{
			name: "zero port ignored",
			slices: []plan.Slice{
				{Name: "web", Port: 0, Public: true},
			},
			usedPorts:    nil,
			wantFindings: 1,
			wantSeverity: doctor.SeverityOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := &plan.AppPlan{Name: "test", Slices: tt.slices}
			sig := &doctor.Signal{UsedPorts: tt.usedPorts}

			findings := rules.PortRule{}.Evaluate(p, sig)

			require.Len(t, findings, tt.wantFindings)
			assert.Equal(t, tt.wantSeverity, findings[0].Severity)
		})
	}
}

func TestPortRule_Evaluate_FixSuggestion(t *testing.T) {
	t.Parallel()

	p := &plan.AppPlan{
		Name: "test",
		Slices: []plan.Slice{
			{Name: "web", Port: 8080, Public: true},
		},
	}
	sig := &doctor.Signal{UsedPorts: []int{8080}}

	findings := rules.PortRule{}.Evaluate(p, sig)

	require.Len(t, findings, 1)
	assert.Contains(t, findings[0].Fix, "8080")
}
