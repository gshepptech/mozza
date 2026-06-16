---
domain: docs
file: readme-community
depends_on: []
estimated_complexity: medium
---

## Purpose

Rewrite the README with authentic, pain-first voice. Set up GitHub repo infrastructure (Discussions, issue templates, CONTRIBUTING.md). Plan Discord community structure.

## Scope

**Included:**
- README.md complete rewrite
- CONTRIBUTING.md
- CODE_OF_CONDUCT.md
- SECURITY.md
- GitHub issue templates (Bug Report, Feature Request, Recipe Submission)
- GitHub Discussions categories configuration
- GitHub Actions workflow (build, test, lint)
- Discord server structure documentation

**Excluded:**
- Actual Discord server creation (manual, requires Discord account)
- Actual GitHub repo migration from GitLab (manual)
- 60-second demo recording (requires running app)
- Domain acquisition

## Requirements

- REQ-1: README opens with real developer frustration — not marketing speak
- REQ-2: 60-second demo GIF/terminal recording embedded (placeholder for now)
- REQ-3: Side-by-side: K8s YAML (200 lines) vs Mozza recipe (10 lines)
- REQ-4: Quick start: one-line install → `mozza init` → `mozza up` → running app
- REQ-5: Feature list with real screenshots (placeholders for now)
- REQ-6: "How it works" section with visual flow
- REQ-7: Comparison table: Mozza vs Coolify vs Dokku vs Render vs Railway
- REQ-8: Links to docs, Discord, GitHub Discussions, marketplace, landing page
- REQ-9: Contributing section
- REQ-10: License section (placeholder)
- REQ-11: No emojis. Custom badges/shields only.
- REQ-12: CONTRIBUTING.md with development setup, PR process, coding standards
- REQ-13: CODE_OF_CONDUCT.md (Contributor Covenant)
- REQ-14: SECURITY.md with vulnerability reporting process
- REQ-15: Issue templates: Bug Report (repro steps, expected/actual), Feature Request (use case, proposal), Recipe Submission
- REQ-16: GitHub Discussions categories: Q&A, Show and Tell, Ideas, General
- REQ-17: GitHub Actions: build, test, lint on PR + push to main
- REQ-18: Branch protection on main (require PR, require passing checks)
- REQ-19: Discord structure: #general, #help, #show-and-tell, #recipes, #feature-requests, #announcements

## Explicit Behaviors

- README voice: first-person developer frustration, like a blog post. "I spent 3 hours debugging a Kubernetes deployment for a todo app. Three. Hours. So I built Mozza."
- No buzzwords: no "cloud-native", no "DevOps", no "platform engineering"
- Comparison table with honest assessments (don't trash competitors, acknowledge trade-offs)
- Badges: build status, go version, license — custom shields.io badges only
- Quick start section is copy-pasteable (each command on its own line with `$` prefix)
- GitHub Actions workflow: Go build matrix (linux/amd64, darwin/arm64), golangci-lint, go test -race
- Branch protection: reviewers required, status checks required, no force push to main

## Dependencies

None — documentation can be written independently.

## Interfaces

**Files created:**
```
README.md
CONTRIBUTING.md
CODE_OF_CONDUCT.md
SECURITY.md
.github/ISSUE_TEMPLATE/bug-report.yml
.github/ISSUE_TEMPLATE/feature-request.yml
.github/ISSUE_TEMPLATE/recipe-submission.yml
.github/DISCUSSION_TEMPLATE/ (if supported)
.github/workflows/ci.yml
```

## Constraints

- No emojis in README
- Authentic voice — not corporate, not try-hard
- README should be under 500 lines (focused, not exhaustive)
- Comparison table must be fair and accurate

## Edge Cases

- Competitors change features → comparison table may need updates (note date)
- Screenshots not available yet → use code blocks as placeholders
- Demo GIF not recorded yet → placeholder with instruction to record

## Acceptance Criteria

- [ ] README has authentic pain-first opening
- [ ] Quick start section is copy-pasteable
- [ ] Side-by-side comparison (K8s vs Mozza) included
- [ ] Comparison table with 5 tools
- [ ] No emojis anywhere
- [ ] CONTRIBUTING.md with dev setup
- [ ] CODE_OF_CONDUCT.md (Contributor Covenant)
- [ ] SECURITY.md with reporting process
- [ ] 3 issue templates created
- [ ] GitHub Actions CI workflow
- [ ] Discord channel structure documented

## Definition of Done

README compelling and authentic. All community files in place. CI workflow runs on PR. Issue templates available.

## Related Files

- frontend/landing-page.md (landing page links from README)
- infra/install-releases.md (install command in quick start)

## Testing Strategy

- README renders correctly on GitHub (check markdown rendering)
- CI workflow passes on current codebase
- Issue templates render correctly
- All links valid
