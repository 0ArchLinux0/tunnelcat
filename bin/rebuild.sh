#!/usr/bin/env bash
# rebuild.sh — force a clean rebuild of the Go↔Rust bridge.
#
# USE THIS whenever you change the Rust crate. The cgo cache is
# keyed on the C header (which we hand-write and only changes when
# the ABI changes), NOT on the .a file. So if you change a Rust
# function's body, the Go binary will keep linking the OLD .a until
# you run this script.
#
# Usage:
#   bin/rebuild.sh              # clean rebuild
#   bin/rebuild.sh --quick      # only re-link Go, no cargo clean
#
# --quick is safe when:
#   - you only changed Go code
#   - you only changed Rust code BUT the C header is unchanged
#     and the symbol set is the same (which is our default)
#
# Default (no flag) is for when:
#   - you added/removed a #[no_mangle] extern "C" function
#   - you changed the C header
#   - you're debugging "why is the old code still running?"
#   - you just don't know which case you're in

set -euo pipefail

QUICK=0
if [[ "${1:-}" == "--quick" ]]; then
  QUICK=1
fi

cd "$(dirname "$0")/.."

if [[ "${QUICK}" -eq 1 ]]; then
  echo "rebuild: --quick mode, only re-linking Go"
  # Don't cargo clean. Just rebuild the .a (cargo will figure
  # out what changed from source mtime) and force a Go relink.
  (cd crates/tunnelcat-proto && cargo build)
  go build -o /tmp/tunnelcat ./cmd/tunnelcat
  echo "done. /tmp/tunnelcat is fresh."
  exit 0
fi

echo "rebuild: full clean"
echo ""
echo "  → cargo clean (removes crates/tunnelcat-proto/target/)"
(cd crates/tunnelcat-proto && cargo clean)
echo "  → go clean -cache (removes Go's cgo build cache)"
go clean -cache
echo "  → cargo build (rebuilds the Rust static lib)"
(cd crates/tunnelcat-proto && cargo build)
echo "  → go build (rebuilds the Go binary, which re-links the lib)"
go build -o /tmp/tunnelcat ./cmd/tunnelcat
echo ""
echo "done. /tmp/tunnelcat is fresh and linked against the new .a"
echo ""
echo "verify the rebuild worked:"
echo "  /tmp/tunnelcat --version"
echo ""
