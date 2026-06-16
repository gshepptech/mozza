#!/usr/bin/env bash
# Mozza installer — download and install the correct binary for the current platform.
# Usage: curl -sSfL https://raw.githubusercontent.com/gshepptech/mozza/main/scripts/install.sh | bash
set -euo pipefail

REPO="gshepptech/mozza"
INSTALL_DIR="/usr/local/bin"
BINARY="mozza"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33mWARN:\033[0m %s\n' "$*" >&2; }
error() { printf '\033[1;31mERROR:\033[0m %s\n' "$*" >&2; exit 1; }

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        error "Required command not found: $1"
    fi
}

# ---------------------------------------------------------------------------
# Detect platform
# ---------------------------------------------------------------------------

detect_os() {
    local os
    os="$(uname -s)"
    case "$os" in
        Linux)  echo "linux" ;;
        Darwin) echo "darwin" ;;
        *)      error "Unsupported OS: $os" ;;
    esac
}

detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   echo "amd64" ;;
        aarch64|arm64)  echo "arm64" ;;
        *)              error "Unsupported architecture: $arch" ;;
    esac
}

# ---------------------------------------------------------------------------
# Resolve latest version
# ---------------------------------------------------------------------------

latest_version() {
    local url="https://api.github.com/repos/${REPO}/releases/latest"
    local tag
    tag="$(curl -sSfL "$url" | grep '"tag_name"' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"
    if [ -z "$tag" ]; then
        error "Could not determine latest release from $url"
    fi
    echo "$tag"
}

# ---------------------------------------------------------------------------
# Download and verify
# ---------------------------------------------------------------------------

download_and_install() {
    local version="$1" os="$2" arch="$3"
    local ver_no_v="${version#v}"
    local ext="tar.gz"
    if [ "$os" = "darwin" ]; then
        ext="zip"
    fi

    local archive="mozza_${ver_no_v}_${os}_${arch}.${ext}"
    local base_url="https://github.com/${REPO}/releases/download/${version}"
    local tmpdir
    tmpdir="$(mktemp -d)"
    # shellcheck disable=SC2064
    trap "rm -rf '$tmpdir'" EXIT

    info "Downloading ${archive}..."
    curl -sSfL -o "${tmpdir}/${archive}" "${base_url}/${archive}"

    info "Downloading checksums..."
    curl -sSfL -o "${tmpdir}/checksums.txt" "${base_url}/checksums.txt"

    info "Verifying checksum..."
    (
        cd "$tmpdir"
        if command -v sha256sum >/dev/null 2>&1; then
            grep "$archive" checksums.txt | sha256sum -c --quiet -
        elif command -v shasum >/dev/null 2>&1; then
            grep "$archive" checksums.txt | shasum -a 256 -c --quiet -
        else
            warn "Neither sha256sum nor shasum found; skipping verification"
        fi
    )

    info "Extracting..."
    case "$ext" in
        tar.gz) tar -xzf "${tmpdir}/${archive}" -C "$tmpdir" ;;
        zip)    unzip -qo "${tmpdir}/${archive}" -d "$tmpdir" ;;
    esac

    if [ ! -f "${tmpdir}/${BINARY}" ]; then
        error "Binary '${BINARY}' not found in archive"
    fi
    chmod +x "${tmpdir}/${BINARY}"

    info "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        sudo mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi

    info "Installed ${BINARY} ${version} to ${INSTALL_DIR}/${BINARY}"
}

# ---------------------------------------------------------------------------
# Preflight checks
# ---------------------------------------------------------------------------

preflight() {
    need_cmd curl
    need_cmd uname

    if ! command -v docker >/dev/null 2>&1; then
        warn "Docker is not installed. Mozza requires Docker to deploy containers."
        warn "Install Docker from https://docs.docker.com/get-docker/"
    fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
    preflight

    local os arch version
    os="$(detect_os)"
    arch="$(detect_arch)"
    version="${MOZZA_VERSION:-$(latest_version)}"

    info "Platform: ${os}/${arch}"
    info "Version:  ${version}"

    if [ -f "${INSTALL_DIR}/${BINARY}" ]; then
        local current
        current="$("${INSTALL_DIR}/${BINARY}" version 2>/dev/null | head -1 || echo "unknown")"
        info "Existing installation detected: ${current}"
        info "Upgrading to ${version}..."
    fi

    download_and_install "$version" "$os" "$arch"

    info "Run 'mozza --help' to get started."
}

main "$@"
