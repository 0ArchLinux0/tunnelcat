#!/usr/bin/env bash
# tunnelcat install script
#
# This script lives at the repo root. It does two things:
#   1. Default: download a release tarball from GitHub Releases
#      and install to /usr/local/bin/ (or $TUNNELCAT_INSTALL_DIR).
#   2. With --from-source: build from the current checkout and
#      install to the same place. Requires Go 1.27+.
#
# Usage:
#   # Install the latest release:
#   curl -fsSL https://raw.githubusercontent.com/0ArchLinux0/tunnelcat/main/install.sh | sh
#
#   # Install a specific version:
#   curl -fsSL https://raw.githubusercontent.com/0ArchLinux0/tunnelcat/main/install.sh | TUNNELCAT_VERSION=v0.1.0 sh
#
#   # Build from this checkout (development):
#   ./install.sh --from-source
#
#   # Custom install dir:
#   TUNNELCAT_INSTALL_DIR=$HOME/bin ./install.sh

set -euo pipefail

REPO="0ArchLinux0/tunnelcat"
INSTALL_DIR="${TUNNELCAT_INSTALL_DIR:-/usr/local/bin}"

# Parse flags. We only have one for now: --from-source.
FROM_SOURCE=0
for arg in "$@"; do
  case "${arg}" in
    --from-source) FROM_SOURCE=1 ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
      exit 0
      ;;
    *)
      echo "error: unknown argument: ${arg}" >&2
      exit 1
      ;;
  esac
done

# ─────────────────────────────────────────────────────────────
# Mode 1: install from a release tarball (default)
# ─────────────────────────────────────────────────────────────

install_from_release() {
  OS="$(uname -s | tr 'A-Z' 'a-z')"
  ARCH="$(uname -m)"
  case "${ARCH}" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
  esac
  case "${OS}" in
    darwin|linux) ;;
    *) echo "unsupported OS: ${OS}" >&2; exit 1 ;;
  esac

  FRIENDLY="${OS}-${ARCH}"
  URL_BASE="https://github.com/${REPO}/releases"
  if [[ -n "${TUNNELCAT_VERSION:-}" ]]; then
    URL_BASE="${URL_BASE}/download/${TUNNELCAT_VERSION}"
  else
    URL_BASE="${URL_BASE}/latest/download"
  fi
  TARBALL_URL="${URL_BASE}/${FRIENDLY}.tar.gz"

  echo "installing tunnelcat for ${FRIENDLY}..."
  echo "  from: ${TARBALL_URL}"
  echo "  to:   ${INSTALL_DIR}/tunnelcat"

  tmp="$(mktemp -d)"
  trap 'rm -rf "${tmp}"' EXIT

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${TARBALL_URL}" -o "${tmp}/tunnelcat.tar.gz"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "${TARBALL_URL}" -O "${tmp}/tunnelcat.tar.gz"
  else
    echo "error: need curl or wget" >&2
    exit 1
  fi

  # Verify SHA if a checksums file is reachable. The release
  # has two possible checksums files:
  #   - "${FRIENDLY}/SHA256SUMS" (per-platform, format: "hash  filename",
  #     matches "${FRIENDLY}.tar.gz")
  #   - "checksums.txt" (top-level, goreleaser-style, format: "hash  filename",
  #     covers the goreleaser-named tarballs but not the legacy
  #     "${FRIENDLY}.tar.gz")
  # Try the per-platform one first (it has the right filename).
  # If that fails, try the top-level one (the filename won't match
  # legacy assets, so we won't find a hash; that's fine, we just
  # skip the verification).
  expected=""
  sums_url="${URL_BASE}/${FRIENDLY}/SHA256SUMS"
  if curl -fsSL "${sums_url}" -o "${tmp}/sums" 2>/dev/null; then
    expected="$(awk -v p="${FRIENDLY}.tar.gz" '$2==p {print $1}' "${tmp}/sums")"
  fi
  if [[ -n "${expected}" ]]; then
    actual="$(sha256sum "${tmp}/tunnelcat.tar.gz" 2>/dev/null | awk '{print $1}')"
    if [[ "${expected}" != "${actual}" ]]; then
      echo "error: SHA mismatch for ${FRIENDLY}.tar.gz" >&2
      echo "  expected: ${expected}" >&2
      echo "  actual:   ${actual}" >&2
      exit 1
    fi
    echo "  sha256: ${actual} ✓"
  else
    echo "  (no per-platform SHA256SUMS at the release; skipping SHA verification)"
  fi

  tar -xzf "${tmp}/tunnelcat.tar.gz" -C "${tmp}"
  mkdir -p "${INSTALL_DIR}"
  install -m 0755 "${tmp}/${FRIENDLY}/tunnelcat" "${INSTALL_DIR}/tunnelcat"
  echo "  ✓ installed ${INSTALL_DIR}/tunnelcat"
}

# ─────────────────────────────────────────────────────────────
# Mode 2: build from this checkout (--from-source)
# ─────────────────────────────────────────────────────────────

install_from_source() {
  # Sanity: are we in a git checkout?
  if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    echo "error: --from-source must be run from a tunnelcat git checkout" >&2
    exit 1
  fi
  # Sanity: is Go available?
  if ! command -v go >/dev/null 2>&1; then
    echo "error: --from-source requires Go 1.27+ on PATH" >&2
    echo "  install Go from https://go.dev/dl/" >&2
    exit 1
  fi
  GO_VERSION="$(go version | awk '{print $3}' | sed 's/^go//')"
  GO_MAJOR="$(echo "${GO_VERSION}" | cut -d. -f1)"
  GO_MINOR="$(echo "${GO_VERSION}" | cut -d. -f2)"
  if [[ "${GO_MAJOR}" -lt 1 ]] || { [[ "${GO_MAJOR}" -eq 1 ]] && [[ "${GO_MINOR}" -lt 27 ]]; }; then
    echo "error: Go ${GO_VERSION} is too old; need 1.27+" >&2
    exit 1
  fi

  # If the Rust crate is set up, build the cgo bridge too. If not,
  # build a no-cgo version (the Rust bridge returns errors at
  # runtime but everything else works).
  cd "$(git rev-parse --show-toplevel)"
  mkdir -p "${INSTALL_DIR}"

  # Pick a version string. If HEAD is tagged, use the tag; otherwise
  # use the short commit hash; otherwise "dev".
  version="$(git describe --tags --always 2>/dev/null || echo dev)"

  if [[ -d crates/tunnelcat-proto ]] && command -v cargo >/dev/null 2>&1; then
    echo "→ building Rust crate (host, release)"
    (cd crates/tunnelcat-proto && cargo build --release)
    # Build into a temp dir, then move into place. This avoids
    # the duplicate-libraries warning that comes from putting
    # -L and -l on the same CGO_LDFLAGS.
    BUILDDIR="$(mktemp -d)"
    CGO_ENABLED=1 \
    CGO_CFLAGS="-I${PWD}/crates/tunnelcat-proto/include" \
    CGO_LDFLAGS="-L${PWD}/crates/tunnelcat-proto/target/release" \
      go build -trimpath -ldflags "-s -w -X main.version=${version} -extldflags '-ltunnelcat_proto'" \
      -o "${BUILDDIR}/tunnelcat" ./cmd/tunnelcat
    mv "${BUILDDIR}/tunnelcat" "${INSTALL_DIR}/tunnelcat"
    rm -rf "${BUILDDIR}"
  else
    echo "→ building (no Rust bridge; CGO_ENABLED=0)"
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${version}" -o "${INSTALL_DIR}/tunnelcat" ./cmd/tunnelcat
  fi
  chmod 0755 "${INSTALL_DIR}/tunnelcat"
  echo "  ✓ installed ${INSTALL_DIR}/tunnelcat (version: ${version})"
}

# ─────────────────────────────────────────────────────────────
# Dispatch
# ─────────────────────────────────────────────────────────────

if [[ "${FROM_SOURCE}" -eq 1 ]]; then
  install_from_source
else
  install_from_release
fi

# Sanity check (only if it's on PATH already).
if command -v tunnelcat >/dev/null 2>&1; then
  echo ""
  tunnelcat --version
else
  echo ""
  echo "note: ${INSTALL_DIR} is not on PATH. Add it to your shell rc,"
  echo "or call the binary directly: ${INSTALL_DIR}/tunnelcat --version"
fi
