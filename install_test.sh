#!/usr/bin/env bash
# install_test.sh — smoke tests for the install.sh script.
#
# This file is named *_test.sh so it shows up in the test
# discovery, but it's a bash script, not a Go test. Run it
# manually with `bash install_test.sh` (it's gated on
# TUNNELCAT_INSTALL_TESTS=1 to avoid running in CI by
# default because the install script writes to /tmp).

set -euo pipefail

if [[ "${TUNNELCAT_INSTALL_TESTS:-}" != "1" ]]; then
  echo "skip: set TUNNELCAT_INSTALL_TESTS=1 to run install script tests"
  exit 0
fi

REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
INSTALL_SH="${REPO_ROOT}/install.sh"

if [[ ! -f "${INSTALL_SH}" ]]; then
  echo "FAIL: install.sh not found at ${INSTALL_SH}"
  exit 1
fi

PASS=0
FAIL=0

assert_eq() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [[ "${expected}" == "${actual}" ]]; then
    echo "  ✓ ${name}"
    PASS=$((PASS + 1))
  else
    echo "  ✗ ${name}: expected '${expected}', got '${actual}'"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [[ "${actual}" == *"${expected}"* ]]; then
    echo "  ✓ ${name}"
    PASS=$((PASS + 1))
  else
    echo "  ✗ ${name}: '${actual}' does not contain '${expected}'"
    FAIL=$((FAIL + 1))
  fi
}

assert_file_exists() {
  local name="$1"
  local path="$2"
  if [[ -f "${path}" ]]; then
    echo "  ✓ ${name}"
    PASS=$((PASS + 1))
  else
    echo "  ✗ ${name}: file not found at ${path}"
    FAIL=$((FAIL + 1))
  fi
}

# ─────────────────────────────────────────────────────────────
# Test 1: --help works
# ─────────────────────────────────────────────────────────────
echo "test: --help"
out="$(bash "${INSTALL_SH}" --help 2>&1)"
assert_contains "help shows install --from-source" "--from-source" "${out}"
assert_contains "help mentions TUNNELCAT_INSTALL_DIR" "TUNNELCAT_INSTALL_DIR" "${out}"

# ─────────────────────────────────────────────────────────────
# Test 2: install from release works (with custom INSTALL_DIR
# so we don't need root)
# ─────────────────────────────────────────────────────────────
echo "test: install from release"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "${TMPDIR}" "${TMPDIR2}" "${TMPDIR3}" EXIT' EXIT
TUNNELCAT_INSTALL_DIR="${TMPDIR}/bin" bash "${INSTALL_SH}" > /tmp/install-test.log 2>&1
assert_file_exists "binary installed" "${TMPDIR}/bin/tunnelcat"
out="$( "${TMPDIR}/bin/tunnelcat" --version 2>&1)"
assert_contains "version is from release" "tunnelcat v" "${out}"

# ─────────────────────────────────────────────────────────────
# Test 3: install from a specific version works
# ─────────────────────────────────────────────────────────────
echo "test: install from v0.1.0"
TMPDIR2="$(mktemp -d)"
TUNNELCAT_INSTALL_DIR="${TMPDIR2}/bin" TUNNELCAT_VERSION=v0.1.0 bash "${INSTALL_SH}" > /tmp/install-test.log 2>&1
assert_file_exists "v0.1.0 binary installed" "${TMPDIR2}/bin/tunnelcat"
out="$( "${TMPDIR2}/bin/tunnelcat" --version 2>&1)"
assert_contains "v0.1.0 version string" "tunnelcat v0.1.0" "${out}"

# ─────────────────────────────────────────────────────────────
# Test 4: install from source works
# ─────────────────────────────────────────────────────────────
echo "test: install --from-source"
TMPDIR3="$(mktemp -d)"
(
  cd "${REPO_ROOT}"
  TUNNELCAT_INSTALL_DIR="${TMPDIR3}/bin" bash "${INSTALL_SH}" --from-source > /tmp/install-test.log 2>&1
)
assert_file_exists "from-source binary installed" "${TMPDIR3}/bin/tunnelcat"
out="$( "${TMPDIR3}/bin/tunnelcat" --version 2>&1)"
assert_contains "from-source version is dev or git describe" "tunnelcat " "${out}"

# ─────────────────────────────────────────────────────────────
# Test 5: --from-source outside a git checkout errors
# ─────────────────────────────────────────────────────────────
echo "test: --from-source outside git errors"
out="$(cd /tmp && bash "${INSTALL_SH}" --from-source 2>&1)" || true
assert_contains "errors when not in a git checkout" "must be run from a tunnelcat git checkout" "${out}"

# ─────────────────────────────────────────────────────────────
# Summary
# ─────────────────────────────────────────────────────────────
echo ""
echo "results: ${PASS} passed, ${FAIL} failed"
[[ "${FAIL}" -eq 0 ]] && exit 0 || exit 1
