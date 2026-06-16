package doctor

import (
	"fmt"
	"strings"
)

// FormatText renders a Report as human-readable text suitable for terminal output.
// Findings are grouped by category: "Must fix before deploy", "Recommended",
// "Nice to have", and then passing checks.
func FormatText(r *Report) string {
	var b strings.Builder

	grouped := groupBySeverity(r.Findings)

	// Category-based grouping for non-OK findings.
	mustFix := grouped[SeverityError]
	recommended := grouped[SeverityWarning]
	niceToHave := grouped[SeverityInfo]
	passing := grouped[SeverityOK]

	if len(mustFix) > 0 {
		writeCategoryGroup(&b, string(CategoryMustFix), mustFix)
	}
	if len(recommended) > 0 {
		writeCategoryGroup(&b, string(CategoryRecommended), recommended)
	}
	if len(niceToHave) > 0 {
		writeCategoryGroup(&b, string(CategoryNiceToHave), niceToHave)
	}
	if len(passing) > 0 {
		writePassingGroup(&b, passing)
	}

	fmt.Fprintf(&b, "\n%d errors, %d warnings, %d info, %d ok\n",
		r.Summary.Errors, r.Summary.Warnings, r.Summary.Info, r.Summary.OK)

	return b.String()
}

// groupBySeverity buckets findings by their severity level.
func groupBySeverity(findings []Finding) map[Severity][]Finding {
	groups := make(map[Severity][]Finding)
	for _, f := range findings {
		groups[f.Severity] = append(groups[f.Severity], f)
	}
	return groups
}

// writeCategoryGroup writes a category header and all findings in that category.
func writeCategoryGroup(b *strings.Builder, category string, findings []Finding) {
	fmt.Fprintf(b, "\n── %s ─────────────────────────────\n", category)
	for _, f := range findings {
		writeFinding(b, f)
	}
}

// writePassingGroup writes passing checks in a compact format.
func writePassingGroup(b *strings.Builder, findings []Finding) {
	fmt.Fprintf(b, "\n── Passing ─────────────────────────────\n")
	for _, f := range findings {
		fmt.Fprintf(b, "  ✓ %s: %s\n", f.Rule, f.Message)
	}
}

// writeFinding writes a single finding with explanation and suggestion.
func writeFinding(b *strings.Builder, f Finding) {
	severityLabel := strings.ToUpper(string(f.Severity))
	fmt.Fprintf(b, "  [%s] %s: %s\n", severityLabel, f.Rule, f.Message)
	if f.Explanation != "" {
		fmt.Fprintf(b, "    Why: %s\n", f.Explanation)
	}
	if f.Fix != "" {
		fmt.Fprintf(b, "    Fix: %s\n", f.Fix)
	}
	if f.RecipeLine != "" {
		fmt.Fprintf(b, "    Recipe: %s\n", f.RecipeLine)
	}
	if f.Fixable {
		fmt.Fprintf(b, "    (auto-fixable with --fix)\n")
	}
}
