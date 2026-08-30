#!/usr/bin/env bash
# install-mac.sh — install tunnelcat on macOS, with Gatekeeper bypass.
#
# Usage:
#   bin/install-mac.sh                          # install latest
#   bin/install-mac.sh v0.1.0                   # install specific
#
# What it does:
#   1. Detects OS/arch (arm64 or amd64).
#   2. Downloads the tarball from GitHub Releases.
#   3. Verifies the SHA256 against SHA256SUMS.
#   4. Extracts to /usr/local/bin/ (or $TUNNELCAT_INSTALL_DIR).
#   5. Strips the macOS quarantine xattr so the binary is
#      runnable without the "unidentified developer" warning.
#   6. Prints the version.
#
# Why step 5 matters: macOS Gatekeeper quarantines any binary
# downloaded from the internet. The first run shows a scary
# dialog. Stripping the quarantine xattr removes the dialog.
# This is the standard fix for unsigned developer tools.

set -euo pipefail

REPO="0ArchLinux0/tunnelcat"
VERSION="${1:-}"
INSTALL_DIR="${TUNNELCAT_INSTALL_DIR:-/usr/local/bin}"

# Detect OS and arch.
OS="$(uname -s | tr 'A-Z' 'a-z')"
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
esac
if [[ "${OS}" != "darwin" ]]; then
  echo "error: this script is for macOS; use bin/release.sh for other platforms" >&2
  exit 1
fi

FRIENDLY="darwin-${ARCH}"
URL_BASE="https://github.com/${REPO}/releases"
if [[ -n "${VERSION}" ]]; then
  URL_BASE="${URL_BASE}/download/${VERSION}"
else
  URL_BASE="${URL_BASE}/latest/download"
fi
TARBALL_URL="${URL_BASE}/${FRIENDLY}.tar.gz"

echo "installing tunnelcat for ${FRIENDLY}..."
echo "  from: ${TARBALL_URL}"
echo "  to:   ${INSTALL_DIR}/tunnelcat"

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

# Download.
if command -v curl >/dev/null 2>&1; then
  curl -fsSL "${TARBALL_URL}" -o "${tmp}/tunnelcat.tar.gz"
elif command -v wget >/dev/null 2>&1; then
  wget -q "${TARBALL_URL}" -O "${tmp}/tunnelcat.tar.gz"
else
  echo "error: need curl or wget" >&2
  exit 1
fi

# Verify SHA if a SUMS file came with it.
SUMS_URL="${URL_BASE}/${FRIENDLY}/SHA256SUMS"
if command -v curl >/dev/null 2>&1; then
  if curl -fsSL "${SUMS_URL}" -o "${tmp}/SHA256SUMS" 2>/dev/null; then
    expected="$(awk '{print $1}' "${tmp}/SHA256SUMS")"
    actual="$(sha256sum "${tmp}/tunnelcat.tar.gz" 2>/dev/null | awk '{print $1}' || shasum -a 256 "${tmp}/tunnelcat.tar.gz" | awk '{print $1}')"
    if [[ "${expected}" != "${actual}" ]]; then
      echo "error: SHA mismatch" >&2
      echo "  expected: ${expected}" >&2
      echo "  actual:   ${actual}" >&2
      exit 1
    fi
    echo "  sha256: ${actual} ✓"
  fi
fi

# Extract.
tar -xzf "${tmp}/tunnelcat.tar.gz" -C "${tmp}"

# Install.
install -m 0755 "${tmp}/${FRIENDLY}/tunnelcat" "${INSTALL_DIR}/tunnelcat"

# Strip the macOS quarantine xattr. This is the Gatekeeper bypass.
# It removes the "tunnelcat is from an unidentified developer"
# warning on first run. Required because the binary is not
# signed with an Apple Developer ID.
if [[ -n "${INSTALL_DIR%%/*}" ]] || xattr -p com.apple.quarantine "${INSTALL_DIR}/tunnelcat" >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "${INSTALL_DIR}/tunnelcat" 2>/dev/null || true
  echo "  ✓ stripped quarantine xattr (Gatekeeper warning bypassed)"
fi

echo "  ✓ installed ${INSTALL_DIR}/tunnelcat"
echo ""
"${INSTALL_DIR}/tunnelcat" --version
