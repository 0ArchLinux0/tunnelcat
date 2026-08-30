//go:build cgo
// +build cgo

// Package rustbridge is the Go side of the Go↔Rust boundary.
//
// The Rust crate lives at `crates/tunnelcat-proto/`. It is built
// by `cargo build --release` and produces a static library
// `libtunnelcat_proto.a`. The C header it exports is
// `tunnelcat_proto.h` (hand-written at
// `crates/tunnelcat-proto/include/tunnelcat_proto.h` because
// cbindgen emitted C++ headers by default which break cgo).
//
// This package is only compiled when CGO_ENABLED=1, because it
// uses cgo to call into the Rust static library. Binaries built
// with CGO_ENABLED=0 (cross-compiled to other platforms) skip
// this package entirely. The Version() and Echo() functions are
// small enough that callers should check for their presence via
// build tags or just use the rest of the binary normally.
//
// # IMPORTANT: cgo cache gotcha
//
// Go's cgo build cache is keyed on the C source/header contents,
// NOT on the static library. If you change a Rust function's body
// without changing the C header, `go build` will keep linking the
// OLD `.a` file. Symptoms: behavior doesn't change after a
// `cargo build`, mysterious "stale" results, Rust UB checks
// firing in production binaries.
//
// Fix: run `bin/rebuild.sh` after any Rust change. The script
// does `cargo clean` + `go clean -cache` + `cargo build` +
// `go build` to guarantee a fresh link.
package rustbridge

// #cgo CFLAGS: -I${SRCDIR}/../../crates/tunnelcat-proto/include
// #cgo LDFLAGS: ${SRCDIR}/../../crates/tunnelcat-proto/target/release/libtunnelcat_proto.a
// #include <stdlib.h>
// #include "tunnelcat_proto.h"
import "C"

import (
	"errors"
	"unsafe"
)

// Version returns the version string baked into the Rust crate
// at compile time. It is a cheap smoke test: if Version() returns
// a non-empty string, the Go↔Rust boundary is wired up correctly.
func Version() (string, error) {
	c := C.tunnelcat_proto_version()
	if c == nil {
		return "", errors.New("rustbridge: tunnelcat_proto_version returned NULL")
	}
	// C.GoString copies the bytes into a Go-owned string. The
	// Rust function returns a pointer to a static, so we are
	// free to copy without freeing.
	return C.GoString(c), nil
}

// Echo passes a string to Rust's echo function and returns the
// result. This is a more interesting smoke test: it round-trips
// a Go-owned string through the FFI boundary and back.
func Echo(input string) (string, error) {
	cIn := C.CString(input)
	defer C.free(unsafe.Pointer(cIn))

	cOut := C.tunnelcat_proto_echo(cIn)
	if cOut == nil {
		return "", errors.New("rustbridge: tunnelcat_proto_echo returned NULL")
	}
	// The Rust function returns a pointer to a CString that it
	// intentionally leaked (see the Rust source). We copy the
	// bytes, then free the CString.
	defer C.free(unsafe.Pointer(cOut))

	return C.GoString(cOut), nil
}
