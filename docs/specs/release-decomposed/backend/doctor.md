---
domain: backend
file: doctor
depends_on: []
estimated_complexity: low
---

## Purpose

Enhance the existing doctor command with better error messages, plain-English suggestions, severity grouping, and a `--fix` flag for safe auto-remediation.

## Scope

**Included:**
- Enhance existing `internal/doctor/` package
- Plain-English explanations for every finding
- Actionable suggestions with exact recipe lines
- Severity grouping: "Must fix before deploy" / "Recommended" / "Nice to have"
- `mozza doctor --fix` auto-applies safe fixes
- Auto-run before first deploy with summary

**Excluded:**
- Full auto-remediation (v1.1)
- New checks beyond existing ones (just enhance messaging)

## Requirements

- REQ-1: Each finding includes plain-English explanation
- REQ-2: Suggestions are actionable — show exact recipe line to add/change
- REQ-3: Group findings by severity: "Must fix before deploy" / "Recommended" / "Nice to have"
- REQ-4: `mozza doctor --fix` auto-applies safe fixes (e.g., add missing health check defaults)
- REQ-5: Doctor runs automatically before first deploy with summary: "Found 2 issues. Fix them? [Y/n]"

## Explicit Behaviors

- Each finding type gets a message template with placeholders filled from context
- Example: "Your app 'web' doesn't have a health check. This means Mozza can't tell if your app is actually working. Add `health check /ready` to your recipe."
- Safe fixes (auto-apply with --fix):
  - Add default health check (`health check /` or `/healthz`)
  - Add default restart policy (`restart always`)
  - Set default resource limits if missing
- Unsafe fixes (suggest only, don't auto-apply):
  - Port conflicts
  - Missing environment variables
  - Image not found
- Severity levels mapped from existing checks:
  - Must fix: no image, port conflict, invalid syntax
  - Recommended: no health check, no restart policy, no resource limits
  - Nice to have: no labels, no description
- Auto-run on first deploy: check `.mozza-doctor-run` marker file, prompt if missing
- `--fix` applies fixes to recipe file in-place, shows diff before applying

## Dependencies

None — enhances existing doctor package.

## Interfaces

**CLI:**
```
mozza doctor              — run all checks, show findings grouped by severity
mozza doctor --fix        — auto-apply safe fixes with diff preview
mozza doctor --json       — output findings as JSON (for CI/automation)
```

**Internal:**
- `doctor.Finding{Check, Severity, Message, Suggestion, Fix func(*recipe.Recipe) error}`
- `doctor.Severity` enum: MustFix, Recommended, NiceToHave

## Constraints

- `--fix` must show diff and confirm before applying
- Must not break existing doctor functionality
- Must handle recipes with syntax errors gracefully

## Edge Cases

- Recipe with no issues → "All clear! Your recipe is ready to deploy."
- `--fix` on a recipe that's already valid → no-op with message
- Multiple fixes that conflict → apply one at a time with confirmation
- Recipe file read-only → error with clear message

## Acceptance Criteria

- [ ] Every finding has plain-English explanation
- [ ] Suggestions show exact recipe lines to add/change
- [ ] Findings grouped by severity
- [ ] `--fix` auto-applies safe fixes with diff preview
- [ ] Doctor runs before first deploy with prompt
- [ ] Existing doctor tests still pass
- [ ] `make test` passes

## Definition of Done

`mozza doctor` output is clear, actionable, and grouped by severity. `--fix` safely applies common fixes. Auto-run before first deploy works.

## Related Files

None — self-contained enhancement.

## Testing Strategy

- Test message templates with various finding types
- Test --fix applies correct changes to recipe
- Test severity grouping logic
- Test auto-run marker file behavior
- Run: `go test ./internal/doctor/...`
