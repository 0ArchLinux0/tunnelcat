#!/usr/bin/env bash
# release.sh — build tunnelcat binaries for distribution.
#
# v0.1.0: builds for the targets we can reasonably cross-compile
# from a Mac. The Rust bridge (the FFI into the tunnelcat-proto
# crate) only links when the host platform matches the target.
# For other targets, the binary is built with CGO_ENABLED=0 and
# the Rust bridge returns errors at runtime — but the rest of
# tunnelcat (the data plane, the CLI dispatch, the WireGuard
# tunnel) works fine.
#
# Usage:
#   bin/release.sh <version>     # e.g. bin/release.sh v0.1.0
#
# Output:
#   dist/<version>/<friendly>/tunnelcat[.exe]   the binary
#   dist/<version>/<friendly>/SHA256SUMS        checksums
#   dist/<version>/<friendly>.tar.gz            tarball
#   dist/<version>/install.sh                   one-liner installer

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <version>" >&2
  echo "example: $0 v0.1.0" >&2
  exit 1
fi

VERSION="$1"

if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo "error: version must look like v0.1.0 (got: ${VERSION})" >&2
  exit 1
fi

HOST_OS="$(uname -s | tr 'A-Z' 'a-z')"
HOST_ARCH="$(uname -m)"
case "${HOST_ARCH}" in
  x86_64)  HOST_ARCH="amd64" ;;
  aarch64) HOST_ARCH="arm64" ;;
esac

DIST="dist/${VERSION}"
mkdir -p "${DIST}"

echo ""
echo "tunnelcat release builder"
echo "  version : ${VERSION}"
echo "  host    : ${HOST_OS}/${HOST_ARCH}"
echo "  output  : ${DIST}/"
echo ""

# Build the Rust crate in release mode (host arch only).
echo "→ building Rust crate (host, release)"
(cd crates/tunnelcat-proto && cargo build --release)
echo ""

# Each entry: "goos goarch friendly cgo_enabled(0|1)"
TARGETS=(
  "${HOST_OS} ${HOST_ARCH} ${HOST_OS}-${HOST_ARCH} 1"
  "linux amd64 linux-amd64 0"
  "linux arm64 linux-arm64 0"
  "darwin amd64 darwin-amd64 1"
  "darwin arm64 darwin-arm64 1"
  "windows amd64 windows-amd64 0"
)

for entry in "${TARGETS[@]}"; do
  read -r goos goarch friendly cgo <<< "${entry}"
  ext=""
  [[ "${goos}" == "windows" ]] && ext=".exe"

  outdir="${DIST}/${friendly}"
  mkdir -p "${outdir}"
  outpath="${outdir}/tunnelcat${ext}"

  # Decide CGO.
  cgo_cflags=""
  cgo_ldflags=""
  if [[ "${cgo}" == "1" && "${goos}" == "${HOST_OS}" && "${goarch}" == "${HOST_ARCH}" ]]; then
    cgo_cflags="-I${PWD}/crates/tunnelcat-proto/include"
    cgo_ldflags="-L${PWD}/crates/tunnelcat-proto/target/release -ltunnelcat_proto"
    cgo_env="CGO_ENABLED=1"
  else
    cgo_env="CGO_ENABLED=0"
  fi

  echo "→ building ${friendly} (cgo=${cgo_env})"

  ldflags="-s -w -X main.version=${VERSION}"
  set +e
  if [[ -n "${cgo_cflags}" ]]; then
    CGO_ENABLED=1 \
      CGO_CFLAGS="${cgo_cflags}" \
      CGO_LDFLAGS="${cgo_ldflags}" \
      GOOS="${goos}" GOARCH="${goarch}" \
      go build -ldflags "${ldflags}" -o "${outpath}" ./cmd/tunnelcat 2>"${outdir}/build.err"
    rc=$?
  else
    GOOS="${goos}" GOARCH="${goarch}" \
      go build -ldflags "${ldflags}" -o "${outpath}" ./cmd/tunnelcat 2>"${outdir}/build.err"
    rc=$?
  fi
  set -e

  if [[ ! -f "${outpath}" ]]; then
    echo "  ✗ ${friendly} FAILED"
    sed 's/^/    /' "${outdir}/build.err" | head -5
    rm -rf "${outdir}"
    continue
  fi

  rm -f "${outdir}/build.err"

  if command -v sha256sum >/dev/null 2>&1; then
    sha="$(sha256sum "${outpath}" | awk '{print $1}')"
  else
    sha="$(shasum -a 256 "${outpath}" | awk '{print $1}')"
  fi
  size="$(stat -f "%z" "${outpath}" 2>/dev/null || stat -c "%s" "${outpath}")"
  printf "%s  %s\n" "${sha}" "tunnelcat${ext}" > "${outdir}/SHA256SUMS"
  echo "  ✓ ${outpath} (${size} bytes, sha256=${sha})"
done

# Tarball each successful build.
echo ""
echo "→ packaging tarballs"
for outdir in "${DIST}"/*/; do
  friendly="$(basename "${outdir}")"
  if [[ ! -f "${outdir}/tunnelcat" && ! -f "${outdir}/tunnelcat.exe" ]]; then
    continue
  fi
  tarball="${DIST}/${friendly}.tar.gz"
  tar -C "${DIST}" -czf "${tarball}" "${friendly}/" 2>/dev/null
  echo "  ✓ ${tarball}"
done

# Write install.sh.
cat > "${DIST}/install.sh" <<'INSTALL_EOF'
#!/usr/bin/env bash
# tunnelcat install script
# Usage:
#   curl -fsSL https://github.com/0ArchLinux0/tunnelcat/releases/latest/download/install.sh | sh
#   curl -fsSL https://github.com/0ArchLinux0/tunnelcat/releases/download/v0.1.0/install.sh | sh

set -euo pipefail
REPO="0ArchLinux0/tunnelcat"
INSTALL_DIR="${TUNNELCAT_INSTALL_DIR:-/usr/local/bin}"

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

tar -xzf "${tmp}/tunnelcat.tar.gz" -C "${tmp}"
install -m 0755 "${tmp}/${FRIENDLY}/tunnelcat" "${INSTALL_DIR}/tunnelcat"
echo "  ✓ installed ${INSTALL_DIR}/tunnelcat"
INSTALL_EOF
chmod +x "${DIST}/install.sh"
echo "  ✓ ${DIST}/install.sh"

echo ""
echo "done. artifacts in ${DIST}/"
ls -la "${DIST}/"
