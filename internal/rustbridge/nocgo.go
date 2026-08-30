//go:build !cgo
// +build !cgo

// Package rustbridge stubs for non-cgo builds.
//
// This file is compiled when CGO_ENABLED=0 (cross-compiled to
// platforms without a C toolchain available). The real
// implementation (which uses cgo to call into the Rust static
// library) is in bridge.go, which is gated on cgo.
//
// On non-cgo builds, Version() returns an error explaining the
// situation. Callers should not depend on the Rust crate in
// cross-compiled binaries.
package rustbridge

import "errors"

// Version returns an error on non-cgo builds. The real version
// string would come from the Rust crate, but the Rust bridge
// requires cgo which isn't available here.
func Version() (string, error) {
	return "", errors.New("rustbridge: cgo disabled; the Rust crate is not available in this build")
}

// Echo returns an error on non-cgo builds.
func Echo(input string) (string, error) {
	return "", errors.New("rustbridge: cgo disabled; the Rust crate is not available in this build")
}
