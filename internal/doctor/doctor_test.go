package doctor_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gshepptech/mozza/internal/doctor"
	"github.com/gshepptech/mozza/internal/doctor/rules"
	"github.com/gshepptech/mozza/internal/plan"
)

// mockCollector returns a predefined Signal for testing.
type mockCollector struct {
	signal *doctor.Signal
	err    error
}

func (m *mockCollector) Collect(_ context.Context) (*doctor.Signal, error) {
	return m.signal, m.err
}

// testPlan returns a representative AppPlan for engine tests.
func testPlan() *plan.AppPlan {
	return &plan.AppPlan{
		Name: "testapp",
		Slices: []plan.Slice{
			{
				Name:   "web",
				Kind:   plan.SliceKindWeb,
				Image:  "nginx:latest",
				Port:   8080,
				Public: true,
			},
			{
				Name:  "worker",
				Kind:  plan.SliceKindWorker,
				Image: "worker:latest",
			},
			{
				Name:  "db",
				Kind:  plan.SliceKindDatabase,
				Image: "postgres:16",
			},
		},
	}
}

func TestEngine_Run(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		plan         *plan.AppPlan
		signal       *doctor.Signal
		wantErrors   int
		wantWarnings int
		wantOK       int
	}{
		{
			name: "all healthy",
			plan: testPlan(),
			signal: &doctor.Signal{
				DockerReachable: true,
				AvailableImages: []string{"nginx:latest", "worker:latest", "postgres:16"},
				UsedPorts:       nil,
			},
			wantErrors:   0,
			wantWarnings: 5,
			wantOK:       6,
		},
		{
			name: "docker down",
			plan: testPlan(),
			signal: &doctor.Signal{
				DockerReachable: false,
				DockerError:     errors.New("connection refused"),
				AvailableImages: []string{"nginx:latest", "worker:latest", "postgres:16"},
				UsedPorts:       nil,
			},
			wantErrors:   1,
			wantWarnings: 5,
			wantOK:       5,
		},
		{
			name: "missing images and port conflict",
			plan: testPlan(),
			signal: &doctor.Signal{
				DockerReachable: true,
				AvailableImages: []string{"nginx:latest"},
				UsedPorts:       []int{8080},
			},
			wantErrors:   1,
			wantWarnings: 7,
			wantOK:       4,
		},
		{
			name: "empty plan only docker check matters",
			plan: &plan.AppPlan{Name: "empty"},
			signal: &doctor.Signal{
				DockerReachable: true,
				AvailableImages: nil,
				UsedPorts:       nil,
			},
			wantErrors:   0,
			wantWarnings: 0,
			wantOK:       9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			collector := &mockCollector{signal: tt.signal}
			eng := doctor.New(collector, rules.Default()...)

			report, err := eng.Run(context.Background(), tt.plan)

			require.NoError(t, err)
			assert.Equal(t, tt.wantErrors, report.Summary.Errors, "error count")
			assert.Equal(t, tt.wantWarnings, report.Summary.Warnings, "warning count")
			assert.Equal(t, tt.wantOK, report.Summary.OK, "ok count")
		})
	}
}

func TestEngine_Run_CollectorError(t *testing.T) {
	t.Parallel()

	collector := &mockCollector{err: errors.New("boom")}
	eng := doctor.New(collector)

	report, err := eng.Run(context.Background(), testPlan())

	require.Error(t, err)
	assert.Nil(t, report)
	assert.Contains(t, err.Error(), "Run:")
	assert.ErrorContains(t, err, "boom")
}

func TestEngine_Run_NoRules(t *testing.T) {
	t.Parallel()

	collector := &mockCollector{signal: &doctor.Signal{DockerReachable: true}}
	eng := doctor.New(collector)

	report, err := eng.Run(context.Background(), testPlan())

	require.NoError(t, err)
	assert.Empty(t, report.Findings)
	assert.Equal(t, 0, report.Summary.Errors)
}

func TestFormatText_ContainsExpectedStrings(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{
		Findings: []doctor.Finding{
			{Rule: "docker", Severity: doctor.SeverityError, Message: "not reachable", Fix: "start docker", Explanation: "Docker is needed"},
			{Rule: "image", Severity: doctor.SeverityWarning, Message: "missing nginx:latest", Fix: "docker pull nginx:latest"},
			{Rule: "port", Severity: doctor.SeverityOK, Message: "no conflicts"},
		},
		Summary: doctor.ReportSummary{Errors: 1, Warnings: 1, OK: 1},
	}

	output := doctor.FormatText(report)

	assert.Contains(t, output, "[ERROR] docker: not reachable")
	assert.Contains(t, output, "Fix: start docker")
	assert.Contains(t, output, "Why: Docker is needed")
	assert.Contains(t, output, "[WARNING] image: missing nginx:latest")
	assert.Contains(t, output, "no conflicts")
	assert.Contains(t, output, "1 errors, 1 warnings, 0 info, 1 ok")
}

func TestFormatText_EmptyReport(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{}

	output := doctor.FormatText(report)

	assert.Contains(t, output, "0 errors, 0 warnings, 0 info, 0 ok")
}

func TestFormatText_SeverityOrdering(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{
		Findings: []doctor.Finding{
			{Rule: "ok-rule", Severity: doctor.SeverityOK, Message: "all good"},
			{Rule: "err-rule", Severity: doctor.SeverityError, Message: "bad"},
			{Rule: "warn-rule", Severity: doctor.SeverityWarning, Message: "maybe bad"},
			{Rule: "info-rule", Severity: doctor.SeverityInfo, Message: "fyi"},
		},
		Summary: doctor.ReportSummary{Errors: 1, Warnings: 1, Info: 1, OK: 1},
	}

	output := doctor.FormatText(report)

	errIdx := strings.Index(output, "[ERROR]")
	warnIdx := strings.Index(output, "[WARNING]")
	infoIdx := strings.Index(output, "[INFO]")
	okIdx := strings.Index(output, "Passing")

	assert.Less(t, errIdx, warnIdx, "errors should appear before warnings")
	assert.Less(t, warnIdx, infoIdx, "warnings should appear before info")
	assert.Less(t, infoIdx, okIdx, "info should appear before passing")
}

func TestFormatText_CategoryHeaders(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{
		Findings: []doctor.Finding{
			{Rule: "port", Severity: doctor.SeverityError, Message: "port conflict"},
			{Rule: "health", Severity: doctor.SeverityWarning, Message: "no health check"},
			{Rule: "autoscale", Severity: doctor.SeverityInfo, Message: "consider autoscaling"},
		},
		Summary: doctor.ReportSummary{Errors: 1, Warnings: 1, Info: 1},
	}

	output := doctor.FormatText(report)

	assert.Contains(t, output, "Must fix before deploy")
	assert.Contains(t, output, "Recommended")
	assert.Contains(t, output, "Nice to have")
}

func TestFormatText_ExplanationAndRecipeLine(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{
		Findings: []doctor.Finding{
			{
				Rule:        "no-health-check",
				Severity:    doctor.SeverityWarning,
				Message:     "no health check",
				Explanation: "Without a health check, bad pods get traffic",
				Fix:         "add health check /healthz",
				RecipeLine:  "  health check /healthz",
				Fixable:     true,
			},
		},
		Summary: doctor.ReportSummary{Warnings: 1},
	}

	output := doctor.FormatText(report)

	assert.Contains(t, output, "Why: Without a health check")
	assert.Contains(t, output, "Recipe:   health check /healthz")
	assert.Contains(t, output, "(auto-fixable with --fix)")
}

func TestFormatText_PassingChecks(t *testing.T) {
	t.Parallel()

	report := &doctor.Report{
		Findings: []doctor.Finding{
			{Rule: "docker", Severity: doctor.SeverityOK, Message: "Docker daemon is reachable"},
		},
		Summary: doctor.ReportSummary{OK: 1},
	}

	output := doctor.FormatText(report)

	assert.Contains(t, output, "Passing")
	assert.Contains(t, output, "docker: Docker daemon is reachable")
}

func TestCategoryForSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		severity doctor.Severity
		want     doctor.Category
	}{
		{doctor.SeverityError, doctor.CategoryMustFix},
		{doctor.SeverityWarning, doctor.CategoryRecommended},
		{doctor.SeverityInfo, doctor.CategoryNiceToHave},
		{doctor.SeverityOK, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, doctor.CategoryForSeverity(tt.severity))
		})
	}
}
