//! `tunnelcat-proto` — the protocol layer of the tunnelcat VPN.
//!
//! This crate is called from Go via cgo. It contains:
//!
//! 1. The Meow handshake protocol (replaces `tailcat/disco.go`).
//! 2. Type-state machines that make invalid protocol states
//!    unrepresentable at compile time.
//! 3. Crypto primitives (ed25519 signing, X25519 key exchange,
//!    ChaCha20-Poly1305 AEAD) used by the protocol.
//!
//! In stage 1.5 (this commit), the crate is a **skeleton**: it
//! compiles, links from Go, and exports a single function
//! `tunnelcat_proto_version()`. The real protocol lands in stage 2.
//!
//! The C ABI surface lives in `cbindgen.toml`. Anything marked
//! `#[no_mangle] pub extern "C" fn` is exported.
//!
//! ## Why a separate crate
//!
//! Splitting the protocol out from the Go side gives us three
//! things:
//!
//! - **Type safety.** Rust's type system catches protocol
//!   violations at compile time. Go can't express "you cannot
//!   call receive_meowed before send_meow" without a runtime check.
//! - **Fuzzability.** We can `cargo fuzz` the protocol layer
//!   without involving the Go runtime.
//! - **Reusability.** A future Rust client (mobile? embedded?)
//!   can link this crate directly without going through cgo.

#![deny(unsafe_op_in_unsafe_fn)]
// We allow `unsafe` only in the small FFI shim at the bottom of
// this file. Anywhere else, it's a code smell.
#![allow(unused_unsafe)]

use std::ffi::CStr;
use std::os::raw::c_char;

/// A version string identifying this build of the protocol crate.
///
/// Returned as a `*const c_char` pointing to a static, NUL-terminated
/// UTF-8 string. The caller (Go) must NOT free this pointer.
#[no_mangle]
pub extern "C" fn tunnelcat_proto_version() -> *const c_char {
    // We need a NUL-terminated C string. A bare `&'static str`
    // is NOT NUL-terminated (Rust strings are length-prefixed,
    // not NUL-terminated), so `as_ptr()` would give Go a pointer
    // to non-NUL-terminated bytes and `C.GoString` would read
    // past the end into adjacent memory — which is what
    // happened before this fix.
    //
    // `cvt!` would be cleaner but adds a dependency. For a
    // compile-time-known string, `concat!` plus an explicit
    // NUL terminator is the simplest correct thing.
    static VERSION: &[u8] =
        concat!(env!("CARGO_PKG_NAME"), " v", env!("CARGO_PKG_VERSION"), "\0").as_bytes();
    // SAFETY: VERSION is a `&'static [u8]` whose final byte is
    // explicitly 0. Casting `*const u8` to `*const c_char` is
    // a no-op on every platform Go runs on.
    VERSION.as_ptr() as *const c_char
}

/// Test that a C string round-trips through Rust correctly.
///
/// This is the smoke test for the Go↔Rust boundary in stage 1.5.
/// Stage 2 will replace it with the real protocol exports.
#[no_mangle]
pub extern "C" fn tunnelcat_proto_echo(input: *const c_char) -> *const c_char {
    // SAFETY: caller (Go) is responsible for passing a valid
    // NUL-terminated UTF-8 C string or NULL. We check for null
    // and return a static error string in that case.
    if input.is_null() {
        return "error: null input\0".as_ptr() as *const c_char;
    }

    let c_str = unsafe { CStr::from_ptr(input) };
    match c_str.to_str() {
        Ok(s) => {
            // We allocate a new CString so the Go side gets a
            // pointer it can read. The Go cgo shim copies the
            // bytes immediately and never frees it, so leaking
            // the CString is fine for this smoke test.
            let owned = std::ffi::CString::new(format!("echo: {}\n", s))
                .unwrap_or_else(|_| std::ffi::CString::new("error: bad input").unwrap());
            owned.into_raw()
        }
        Err(_) => "error: invalid utf-8\0".as_ptr() as *const c_char,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;

    #[test]
    fn version_is_nonempty() {
        let v = unsafe { CStr::from_ptr(tunnelcat_proto_version()) };
        assert!(v.to_str().unwrap().contains("tunnelcat-proto"));
    }

    #[test]
    fn version_is_nul_terminated() {
        // The C ABI contract for tunnelcat_proto_version is
        // "returns a pointer to a NUL-terminated UTF-8 string."
        // A bare `&'static str` is NOT NUL-terminated (Rust
        // strings are length-prefixed). Our version uses an
        // explicit `\0` at the end of a `[u8]` static; this
        // test enforces that contract.
        let p = tunnelcat_proto_version();
        // Read up to 100 bytes; the NUL must be inside that range.
        let bytes: &[u8] = unsafe { std::slice::from_raw_parts(p as *const u8, 100) };
        let nul_at = bytes.iter().position(|&b| b == 0);
        assert!(
            nul_at.is_some(),
            "tunnelcat_proto_version must return a NUL-terminated string"
        );
        // And CStr::from_ptr must agree.
        let s = unsafe { CStr::from_ptr(p) };
        assert!(s.to_str().is_ok(), "version is not valid UTF-8");
    }

    #[test]
    fn echo_round_trips() {
        let input = CString::new("hello from rust").unwrap();
        let out_ptr = tunnelcat_proto_echo(input.as_ptr());
        let out = unsafe { CStr::from_ptr(out_ptr) };
        assert_eq!(out.to_str().unwrap(), "echo: hello from rust\n");
        // Reclaim the leaked CString to avoid the leak in tests.
        unsafe {
            let _ = std::ffi::CString::from_raw(out_ptr as *mut c_char);
        }
    }
}
