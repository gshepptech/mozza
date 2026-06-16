---
domain: infra
file: install-releases
depends_on: []
estimated_complexity: medium
---

## Purpose

Set up GoReleaser for cross-platform binary releases and create a one-line install script. Enable users to install Mozza with `go install github.com/gshepptech/mozza/cmd/mozza@latest`.

## Scope

**Included:**
- GoReleaser configuration (`.goreleaser.yml`)
- Install script (`scripts/install.sh`)
- Docker image configuration
- `mozza version` command enhancement
- Cross-platform binary targets

**Excluded:**
- Homebrew tap (nice-to-have, defer)
- Actual domain acquisition (manual, out of scope)
- CI/CD pipeline for automated releases (manual GoReleaser runs)

## Requirements

- REQ-1: GoReleaser produces binaries for: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
- REQ-2: Install script detects OS/arch and downloads correct binary
- REQ-3: `curl -fsSL <url> | sh` installs Mozza to `/usr/local/bin/mozza`
- REQ-4: Script verifies checksum after download
- REQ-5: Script handles: existing installation (upgrade), permission issues (suggests sudo), missing deps (warns about Docker)
- REQ-6: Docker image: `docker run -v /var/run/docker.sock:/var/run/docker.sock mozza/mozza serve`
- REQ-7: `go install` path still works
- REQ-8: GitHub Releases page with changelog, checksums, install instructions
- REQ-9: `mozza version` shows version, commit SHA, build date
- REQ-10: Install completes in < 30 seconds on 50Mbps connection (NFR-1)
- REQ-11: Binary < 50MB (NFR-11)

## Explicit Behaviors

- GoReleaser config: ldflags inject version, commit, date into `cmd/version.go` vars
- Install script flow: detect OS → detect arch → construct download URL → download binary → verify checksum → move to /usr/local/bin → verify installation
- Checksum file: `checksums.txt` published alongside binaries in GitHub Release
- Upgrade detection: check if `mozza` binary exists, compare versions, replace if newer
- Permission handling: try `/usr/local/bin/` first, if permission denied suggest `sudo`
- Docker image: multi-stage build, final stage from `alpine:3.19`, expose port 8080
- `mozza version` output format:
  ```
  mozza v1.0.0
  commit: abc1234
  built: 2026-03-18T00:00:00Z
  go: go1.24.0
  ```

## Dependencies

None — self-contained infrastructure setup.

## Interfaces

**CLI:**
```
mozza version  — show version info (enhanced with commit SHA, build date)
```

**Files:**
- `.goreleaser.yml` — GoReleaser configuration
- `scripts/install.sh` — install script
- `Dockerfile` — multi-stage Docker build

## Constraints

- Binary must be < 50MB (NFR-11)
- Install script must work on: Ubuntu 20+, Debian 11+, macOS 12+, Amazon Linux 2
- No dependencies beyond curl/wget and basic shell utilities
- Docker image should be < 100MB

## Edge Cases

- User on unsupported architecture (e.g., armv7) → clear error message
- Download interrupted → cleanup partial file
- No write permission to /usr/local/bin → suggest alternative paths or sudo
- Docker not installed when using Docker image path → host install instead
- Existing installation is newer than offered version → don't downgrade, warn
- User behind corporate proxy → install script should respect HTTPS_PROXY

## Acceptance Criteria

- [ ] GoReleaser produces binaries for 4 platform/arch combos
- [ ] Install script works on Linux and macOS
- [ ] Checksum verification passes
- [ ] Existing installation detected and upgraded
- [ ] Permission issues handled with helpful message
- [ ] Docker image builds and runs
- [ ] `mozza version` shows version, commit, date
- [ ] Binary < 50MB
- [ ] Install < 30 seconds on fast connection

## Definition of Done

GoReleaser config produces correct binaries. Install script works on Linux and macOS. Docker image starts `mozza serve`. `mozza version` shows build info.

## Related Files

None — standalone infrastructure.

## Testing Strategy

- Test install script with shellcheck
- Test GoReleaser config with `goreleaser check`
- Test Docker image builds and starts
- Test version command output format
- Verify binary size < 50MB
