#!/usr/bin/env bash
# release.sh — cross-compile tunnelcat for the common targets.
#
# Usage:
#   bin/release.sh <version>     # e.g. bin/release.sh v0.1.0
#
# What it does:
#   1. Validates the version string
#   2. Builds the Rust crate once per (target, profile)
#   3. Cross-compiles the Go binary for:
#        linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
#   4. Embeds the version string via -ldflags
#   5. Places the result in dist/<version>/<target>/tunnelcat[.exe]
#   6. Prints a sha256 for each binary
#
# What it does NOT do:
#   - Create the GitHub release (needs the user's push access)
#   - Sign the binaries (the user can add cosign later)
#   - Build Windows MSI / macOS DMG (goreleaser can do this)
#
# Requirements:
#   - Go 1.27+ with CGO_ENABLED support for the host arch
#   - Rust stable
#   - The crates/tunnelcat-proto C header in the stable include
#     path (build.rs does this automatically)

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

# Cross-compile matrix.
TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

# Per-target CGO configuration. CGO is required because we link
# against the Rust static library. For non-host targets we need a
# cross C compiler.
#
# Mapping: target -> CGO_ENABLED + (any extra env).
declare -A CGO_ENV
CGO_ENV["linux/amd64"]="CGO_ENABLED=1 CC=x86_64-linux-gnu-gcc"
CGO_ENV["linux/arm64"]="CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc"
CGO_ENV["darwin/amd64"]="CGO_ENABLED=1 CC=o64-clang"   # requires osxcross
CGO_ENV["darwin/arm64"]="CGO_ENABLED=1 CC=oa64-clang"  # requires osxcross
CGO_ENV["windows/amd64"]="CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc"

# Detect host platform.
HOST_OS="$(go env GOOS)"
HOST_ARCH="$(go env GOARCH)"

# Setup.
DIST="dist/${VERSION}"
mkdir -p "${DIST}"
echo ""
echo "tunnelcat release builder"
echo "  version : ${VERSION}"
echo "  host    : ${HOST_OS}/${HOST_ARCH}"
echo "  targets : ${TARGETS[*]}"
echo "  output  : ${DIST}/"
echo ""

# Make sure the Rust crate is up to date.
echo "→ building Rust crate (host)"
(cd crates/tunnelcat-proto && cargo build --release)
echo ""

# For each target, rebuild the Rust static lib with the cross
# toolchain, then cross-compile Go with cgo, then collect the
# binary.
for target in "${TARGETS[@]}"; do
  os="${target%/*}"
  arch="${target#*/}"
  ext=""
  [[ "${os}" == "windows" ]] && ext=".exe"

  outdir="${DIST}/${target}"
  mkdir -p "${outdir}"
  outpath="${outdir}/tunnelcat${ext}"

  echo "→ building ${target}"

  # CGO env (with a sane default for host).
  if [[ "${target}" == "${HOST_OS}/${HOST_ARCH}" ]]; then
    cgo_line="CGO_ENABLED=1"
  else
    cgo_line="${CGO_ENV[${target}]:-CGO_ENABLED=0}"
    if [[ "${cgo_line}" == *"CGO_ENABLED=0"* ]]; then
      echo "  warning: no CGO toolchain configured for ${target};" \
        "falling back to CGO_ENABLED=0 (cgo bridge will not link)"
    fi
  fi

  # Rust cross-compile for the target. This uses cargo's standard
  # target-triple form. Skip if the target isn't installed.
  rust_target=""
  case "${target}" in
    linux/amd64)   rust_target="x86_64-unknown-linux-gnu" ;;
    linux/arm64)   rust_target="aarch64-unknown-linux-gnu" ;;
    darwin/amd64)  rust_target="x86_64-apple-darwin" ;;
    darwin/arm64)  rust_target="aarch64-apple-darwin" ;;
    windows/amd64) rust_target="x86_64-pc-windows-gnu" ;;
  esac
  if [[ -n "${rust_target}" ]] && rustup target list --installed 2>/dev/null | grep -q "${rust_target}"; then
    (cd crates/tunnelcat-proto && cargo build --release --target "${rust_target}")
    # Copy the static lib into the per-target out dir so the Go
    # build can find it via -L.
    rust_out="crates/tunnelcat-proto/target/${rust_target}/release"
    if [[ -f "${rust_out}/libtunnelcat_proto.a" ]]; then
      cp "${rust_out}/libtunnelcat_proto.a" "${outdir}/"
      # Also re-emit the C header for the per-target include.
      mkdir -p "${outdir}/include"
      cp "crates/tunnelcat-proto/include/tunnelcat_proto.h" "${outdir}/include/"
      extra_cflags="-I${outdir}/include"
      extra_ldflags="-L${outdir} -ltunnelcat_proto"
    else
      echo "  warning: Rust build did not produce libtunnelcat_proto.a for ${target}"
      extra_cflags=""
      extra_ldflags=""
    fi
  else
    echo "  warning: Rust target ${rust_target} not installed; skipping Rust cross-compile"
    extra_cflags=""
    extra_ldflags=""
  fi

  # Go cross-compile.
  #
  # CGO_CFLAGS / CGO_LDFLAGS are how you point cgo at a non-default
  # include path / library. We use them to point at the per-target
  # Rust static lib.
  if [[ -n "${extra_cflags}" ]]; then
    GOOS="${os}" GOARCH="${arch}" \
      CGO_ENABLED=1 \
      CGO_CFLAGS="${extra_cflags}" \
      CGO_LDFLAGS="${extra_ldflags}" \
      go build \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "${outpath}" \
        ./cmd/tunnelcat 2>/dev/null \
      || echo "  warning: Go build with cgo failed for ${target}; falling back to CGO_ENABLED=0"
  fi

  if [[ ! -f "${outpath}" ]]; then
    # Fallback: build without cgo. The Go binary will work but
    # the Rust bridge will return errors. The user can rebuild
    # with the right toolchain.
    GOOS="${os}" GOARCH="${arch}" \
      CGO_ENABLED=0 \
      go build \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o "${outpath}" \
        ./cmd/tunnelcat
  fi

  if [[ -f "${outpath}" ]]; then
    # sha256
    if command -v sha256sum >/dev/null 2>&1; then
      sha="$(sha256sum "${outpath}" | awk '{print $1}')"
    else
      sha="$(shasum -a 256 "${outpath}" | awk '{print $1}')"
    fi
    size="$(stat -f "%z" "${outpath}" 2>/dev/null || stat -c "%s" "${outpath}")"
    echo "  ✓ ${outpath} (${size} bytes, sha256=${sha})"
  else
    echo "  ✗ ${target} FAILED"
  fi
done

echo ""
echo "done. artifacts in ${DIST}/"
echo ""
echo "to create a GitHub release (after the user re-points origin):"
echo "  gh release create ${VERSION} ${DIST}/**/*.{tar.gz,zip} \\"
echo "    --title 'tunnelcat ${VERSION}' \\"
echo "    --notes-file - <<EOF"
echo "  First release of tunnelcat."
echo "  See README for usage."
echo "  EOF"
echo ""
