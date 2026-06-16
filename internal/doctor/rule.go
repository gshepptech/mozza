package doctor

import "github.com/gshepptech/mozza/internal/plan"

// Severity indicates the importance of a diagnostic finding.
type Severity string

// Severity constants ordered from most to least severe.
const (
	// SeverityError indicates a problem that will prevent the application from running.
	SeverityError Severity = "error"
	// SeverityWarning indicates a potential problem that may cause issues at runtime.
	SeverityWarning Severity = "warning"
	// SeverityInfo provides informational context about the environment.
	SeverityInfo Severity = "info"
	// SeverityOK indicates a successful health check.
	SeverityOK Severity = "ok"
)

// Category groups findings into user-facing severity buckets for display.
type Category string

// Category constants for display grouping.
const (
	// CategoryMustFix indicates problems that must be resolved before deploying.
	CategoryMustFix Category = "Must fix before deploy"
	// CategoryRecommended indicates improvements that are strongly recommended.
	CategoryRecommended Category = "Recommended"
	// CategoryNiceToHave indicates optional improvements.
	CategoryNiceToHave Category = "Nice to have"
)

// CategoryForSeverity maps a Severity to its display Category.
func CategoryForSeverity(s Severity) Category {
	switch s {
	case SeverityError:
		return CategoryMustFix
	case SeverityWarning:
		return CategoryRecommended
	case SeverityInfo:
		return CategoryNiceToHave
	case SeverityOK:
		return ""
	}
	return ""
}

// Finding is a single diagnostic result produced by a rule.
type Finding struct {
	// Rule is the short identifier of the rule that produced this finding.
	Rule string
	// Severity indicates the importance of the finding.
	Severity Severity
	// Message describes the finding.
	Message string
	// Explanation is a plain-English description of why this matters.
	Explanation string
	// Fix is a suggested remediation. It may be empty when no fix is applicable.
	Fix string
	// RecipeLine is the exact recipe line to add or change to fix the issue.
	RecipeLine string
	// Fixable indicates whether --fix can auto-remediate this finding.
	Fixable bool
}

// Rule evaluates one aspect of environment health against an application plan.
type Rule interface {
	// Name returns a short identifier for the rule.
	Name() string
	// Evaluate checks the environment and returns zero or more findings.
	Evaluate(p *plan.AppPlan, sig *Signal) []Finding
}
