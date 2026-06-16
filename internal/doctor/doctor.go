// Package doctor provides a health-checking engine with pluggable rules that
// diagnoses problems with a Mozza application's runtime environment.
package doctor

import (
	"context"
	"fmt"

	"github.com/gshepptech/mozza/internal/plan"
)

// Engine runs diagnostic rules against an application plan.
type Engine struct {
	rules     []Rule
	collector SignalCollector
}

// New creates an Engine with the given signal collector and rules.
func New(collector SignalCollector, rules ...Rule) *Engine {
	return &Engine{
		rules:     rules,
		collector: collector,
	}
}

// Report holds the aggregated results of a doctor run.
type Report struct {
	// Findings contains all diagnostic results from every rule.
	Findings []Finding
	// Summary counts findings by severity.
	Summary ReportSummary
}

// ReportSummary counts findings by severity level.
type ReportSummary struct {
	// Errors is the count of error-severity findings.
	Errors int
	// Warnings is the count of warning-severity findings.
	Warnings int
	// Info is the count of info-severity findings.
	Info int
	// OK is the count of ok-severity findings.
	OK int
}

// Run collects environment signals and evaluates all registered rules against
// the provided application plan. It returns a report aggregating every finding.
func (e *Engine) Run(ctx context.Context, p *plan.AppPlan) (*Report, error) {
	sig, err := e.collector.Collect(ctx)
	if err != nil {
		return nil, fmt.Errorf("Run: %w", err)
	}

	var findings []Finding
	for _, r := range e.rules {
		findings = append(findings, r.Evaluate(p, sig)...)
	}

	return &Report{
		Findings: findings,
		Summary:  summarize(findings),
	}, nil
}

// summarize counts findings by severity level.
func summarize(findings []Finding) ReportSummary {
	var s ReportSummary
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			s.Errors++
		case SeverityWarning:
			s.Warnings++
		case SeverityInfo:
			s.Info++
		case SeverityOK:
			s.OK++
		}
	}
	return s
}
