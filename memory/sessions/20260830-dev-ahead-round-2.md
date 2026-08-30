# Session 2026-08-30 — dev-ahead round 2

> **Continuation of the dev-ahead work** that the previous
> session started. The user is still studying. 4 more dev-ahead
> items completed this round.

## What landed (4 commits, 10 dev-ahead items total)

| # | What | Commit |
|---|---|---|
| 7 | CI workflow (`.github/workflows/test.yml`) | `0bd93ec` |
| 8 | Deep-dive doc fix (68 → 69 bytes for Meow) | (no Go commit; updated `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md` and re-pushed to Notion in place) |
| 9 | `bin/release.sh` — cross-compile script | `1faddc9` |
| 10 | `cmd/tunnelcat/main.go` skeleton + `bin/rebuild.sh` | `1faddc9` |

**Note on item 8:** the deep-dive doc fix didn't produce a Go
commit because the deep-dive lives in `~/.pi/agent/knowledge/`,
not in the repo. The change is committed to the user's pi
knowledge tree (via the notion-pipe update). The repo's canon
cheat sheet (`canon/TAILCAT-API.md`) was already correct from
the previous session.

## Two real bugs I found and fixed while doing this

### Bug 1: Rust version string was not NUL-terminated

**Symptom:** `tunnelcat --version` printed
`tunnelcat dev (tunnelcat-proto v0.1.0` followed by garbage
and Rust UB-check warnings (`Vec::set_len`, `is_aligned_to`).

**Root cause:** the Rust function used
`concat!(env!("CARGO_PKG_NAME"), " v", env!("CARGO_PKG_VERSION")).as_ptr()`
to return a `*const c_char`. But Rust `&str` is length-prefixed,
not NUL-terminated. `as_ptr()` returns a pointer to the bytes
with no terminator. Go's `C.GoString` reads until the first
NUL, which is past the end of the static, into adjacent
memory, where it finds bytes that look like a Vec header and
trips the Rust runtime's UB checks.

**Fix:** use a `&'static [u8]` with an explicit `\0` at the end:
```rust
static VERSION: &[u8] =
    concat!(env!("CARGO_PKG_NAME"), " v", env!("CARGO_PKG_VERSION"), "\0").as_bytes();
```

**Test added:** `version_is_nul_terminated` checks the contract
explicitly, so this can't regress.

**Why it wasn't caught earlier:** the rustbridge unit tests
called `Version()` and got the right answer. The reason is
that the adjacent bytes in `.rodata` happened to contain
other ASCII strings (the echo function's error messages),
so the garbage wasn't visually distinct from real output.
A bigger binary or different linker order might have made
this crash instead of print garbage. The lesson: **always
NUL-terminate when crossing the FFI boundary to a C consumer.**

### Bug 2: cgo cache is keyed on the C header, not the static lib

**Symptom:** after fixing Bug 1, `cargo build` succeeded but
`go build` still produced a binary with the old (buggy)
behavior. The static lib had the fix; the Go binary didn't
have it. The Rust UB checks still fired.

**Root cause:** Go's cgo build cache is keyed on the C
source/header contents. When I changed the Rust function body
but not the C header (which is hand-written, not auto-generated
from the lib), cgo thought nothing had changed and re-used the
old cached link.

**Fix:** `bin/rebuild.sh` does `cargo clean` + `go clean -cache`
+ `cargo build` + `go build`. Run this after any Rust change.
Documented in the `internal/rustbridge` package doc so the
next agent (or the user) doesn't lose an hour to this.

**Why this matters for the project:** in Stage 2, we'll be
modifying the Rust crate frequently. Without `bin/rebuild.sh`,
we'd see "stale behavior" bugs that look like "the code I
wrote isn't running." Every commit message that touches the
crate should mention this script.

## What works now (verified end-to-end)

```bash
$ /tmp/tunnelcat --version
tunnelcat dev (tunnelcat-proto v0.1.0)

$ /tmp/tunnelcat --help
tunnelcat — a control-plane-free mesh VPN built on tailcat's data plane
Usage:
  tunnelcat up                    listen and print a connection token
  tunnelcat dial <token>          connect to a server via its token
  ...

$ /tmp/tunnelcat
[prints usage to stderr, exit 2]

$ /tmp/tunnelcat frobnicate
tunnelcat: unknown subcommand "frobnicate"
[prints usage to stderr, exit 2]

$ /tmp/tunnelcat up
tunnelcat up: not yet implemented (lands in stage 1)
[exit 1]

$ /tmp/tunnelcat dial abc
tunnelcat dial: not yet implemented (lands in stage 1)
[exit 1]
```

All exit codes are correct. The CLI is a real, working
program. The Stage 1 session will replace the placeholder
bodies of `up` and `dial` with real `tailcat.Server` and
`tailcat.Client` calls.

## What didn't (and what I noticed)

- **The deep-dive Meow fix didn't produce a Go commit.**
  The deep-dive lives in `~/.pi/agent/knowledge/notion-articles/`,
  not in the repo. The change is recorded in the user's
  pi knowledge tree, not the tunnelcat git history. This is
  intentional (the deep-dive is a learning artifact, not
  source code), but it means "git log" won't show the fix.
  The fix is in the Notion page history instead.
- **The first version of `main_test.go` had compile errors
  and an awkward capture-IO pattern.** I deleted it. The
  test refactor is in Stage 1's scope: refactor `main()` into
  a testable `run(args []string)` function and write proper
  tests there. The user gets to write those tests as their
  first Go unit tests — a useful learning exercise.
- **`bin/release.sh` is a sketch, not a finished tool.** It
  cross-compiles for 5 targets but only the host target will
  succeed in a typical dev environment. Cross toolchains
  (`x86_64-linux-gnu-gcc`, `oa64-clang` for macOS, etc.) are
  not pre-installed. The script falls back to `CGO_ENABLED=0`
  for unbuildable targets, which produces a working binary
  but without the Rust bridge. A real release pipeline
  (goreleaser or similar) is a stage-7+ concern.

## What the next session will do

When the user says "go" (or "start stage 1" or anything that
means they're ready), the next session will:

1. Read `STATUS.md` to see current state.
2. Verify `bin/rebuild.sh && /tmp/tunnelcat --version` is clean.
3. Refactor `cmd/tunnelcat/main.go` so `main()` is just a
   shim that calls `run(os.Args)`, with `run()` testable.
4. Walk the user through writing the body of `run()`'s
   `up` and `dial` subcommands, calling `tailcat.Server` and
   `tailcat.Client` (per `canon/TAILCAT-API.md`).
5. Write the first proper Go unit tests.
6. Commit Stage 1 to a session branch.
7. Update `STATUS.md` and write a handoff.

## External links

- Status tracker: `~/code_repo/tailcat/STATUS.md`
- Plan: `~/code_repo/tailcat/canon/PROJECTS.md`
- Cheat sheet: `~/code_repo/tailcat/canon/TAILCAT-API.md`
- Study plan: `~/.pi/agent/knowledge/study-plans/tunnelcat-user.md`
- Quiz: `~/.pi/agent/knowledge/quizzes/networking-fundamentals.md`
- Deep-dive: Notion P1 `3cc7b981c514819fa38ec70c6f8bd8bd`
  (updated to 69-byte Meow figure)
- Networking skill: `~/.pi/agent/skills/networking-fundamentals/SKILL.md`
- New global rule: `~/.pi/agent/AGENTS.md` "Domain knowledge for the substrate"
- CI workflow: `.github/workflows/test.yml`
- Rebuild script: `bin/rebuild.sh`
- Release script: `bin/release.sh`
