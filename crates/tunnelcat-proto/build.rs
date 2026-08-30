// build.rs — minimal. We previously tried cbindgen but it kept
// emitting C++ headers (cstdarg, etc.) that broke cgo's C-only
// compilation. For stage 1.5 the C header is hand-written and
// committed at `include/tunnelcat_proto.h`. When the protocol
// surface grows past ~5 functions in stage 2 we will revisit
// cbindgen with the correct config (the trick is `cpp_compat = false`
// plus explicit `usize` -> `size_t` mappings).

fn main() {
    // Re-run build.rs if the lib changes.
    println!("cargo:rerun-if-changed=src/lib.rs");
    println!("cargo:rerun-if-changed=include/tunnelcat_proto.h");
}
