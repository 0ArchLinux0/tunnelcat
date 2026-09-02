# STATUS.md — tunnelcat project tracker

> **Read this file to know what tunnelcat is right now.**
> Updated by the agent at the end of every meaningful work session.
> No need to read commit history — everything is here.

---

## TL;DR (read this first, 30 seconds)

**tunnelcat v0.1.0 is shipped.** M0 (seed) closed. M1
("I can give this to one friend") has a 17-sub-step
working plan but no code yet. Repo at
github.com/0ArchLinux0/tunnelcat. Next: start M1 by
reading `canon/plans/M1-20260830.md` (the working plan)
and the M0 closure at `canon/closures/M0-20260830.md`.

---

## What the agent did most recently (last 8 entries, newest first)

| When | What | Commit | Where to read more |
|---|---|---|---|
| 2026-09-01 | M1.7 contact CLI (add/list/show/remove) | `54c0e74` | `cmd/tunnelcat/contact.go` |
| 2026-09-01 | M1.6 contacts pkg + deadlock fix | `7f36615` | `internal/contacts/` |
| 2026-09-01 | CI fixed (rust build before go; removed upstream workflows) | `4e0c80c` | `.github/workflows/test.yml` |
| 2026-08-30 | (about to commit) | (this commit) | v0.1.0 release published |
| 2026-08-30 | rustbridge cgo-gated; release script v2 | `d51a330` | `internal/rustbridge/`, `bin/release.sh` |
| 2026-08-30 | M0 roadmap (M0-M8) | `40afef9` | `canon/ROADMAP.md` |
| 2026-08-30 | Stage 1 ships — working CLI + e2e test | `c247769` (branch) | `cmd/tunnelcat/` |
| 2026-08-30 | cmd/tunnelcat/main.go skeleton + release + rebuild | `1faddc9` | `cmd/tunnelcat/main.go`, `bin/release.sh`, `bin/rebuild.sh` |

**Full log:** `git log --oneline` in `~/code_repo/tailcat`.

---

## What the agent is doing RIGHT NOW (in progress, not yet committed)

> The agent updates this section at the start and end of every
> work session. If this section is empty, the agent is idle.

<!-- AGENT: keep this section current. When you start a task,
add a row. When you finish and commit, remove the row and add
a "what the agent did most recently" entry above. -->

1. **M1.8 in progress** — `tunnelcat dial <contact-name>`.
   Blocked: need the exact `tailcat.ConnBlob` constructor
   from a pubkey. `contacts.Find(name)` returns the pubkey;
   the next step is to build the ConnBlob from it. Hold until
   the constructor is confirmed from upstream docs.

---

## What's on the queue (planned, not started)

> These are the items the agent plans to do while the user is
> studying. Each one is small enough to be one commit. The agent
> works through this list top-to-bottom and reports here.

1. ~~**Git push prep** — `bin/setup-fork.sh` written, syntax-verified,
   ready for the user to run when they have a GitHub fork.~~ ✓
2. ~~**"Verify your study" quiz** — 15 questions + answer key at
   `~/.pi/agent/knowledge/quizzes/networking-fundamentals.md`,
   pushed to Notion as P3 (no Telegram).~~ ✓
3. ~~**Go test harness** — `internal/testharness/harness.go`
   + `harness_test.go` skeleton for Stage 1's "two devices can
   talk" test. Compiles clean; tests are `t.Skip` until Stage 1
   implements `Start()`.~~ ✓
4. ~~**Rust crate skeleton** — `crates/tunnelcat-proto/`
   compiles, `internal/rustbridge/` cgo shim links against the
   static lib, **3 tests pass** (Version, Echo, EchoUnicode
   including Korean characters round-trip). cbindgen didn't
   pan out for stage 1.5 (C++ headers by default); C header is
   hand-written at `crates/tunnelcat-proto/include/`. Will revisit
   in stage 2 when the surface is larger.~~ ✓
5. ~~**tailcat API cheat sheet** — `canon/TAILCAT-API.md`
   (10.5 KB). Covers Server, Client, ConnBlob, Meow protocol
   (corrects the 68-byte figure from the deep-dive to 69 bytes
   — 4 magic + 1 type + 32 WG key + 32 disco key). Includes
   a 50-line preview of Stage 1. Pushed to Notion as P2.~~ ✓
6. ~~**User study plan** — `~/.pi/agent/knowledge/study-plans/tunnelcat-user.md`
   (8.4 KB). Tailored to the user's known languages
   (Python, C/C++, Node.js, Java). 6–8 hours over a week.
   Rust is deferred to Stage 1.5. Pushed to Notion as P2.~~ ✓
7. **CI workflow** — `.github/workflows/test.yml` runs `go test`,
   `cargo test`, and the cgo smoke test on every push. Also
   runs `gofmt -l` and `cargo fmt --check` to catch style drift.
8. ~~**deep-dive doc fix** — Meow packet is 69 bytes (4 magic +
   1 type + 32 + 32), not 68 as the deep-dive said. Fixed in
   the local article and re-pushed to Notion (same page id).
   ~~ ✓
9. ~~**release script** — `bin/release.sh` cross-compiles
   tunnelcat for 5 targets (linux/darwin/windows × amd64/arm64)
   with CGO_ENABLED=1 when a cross toolchain is available.
   Falls back to CGO_ENABLED=0 if not. Embeds the version via
   -ldflags. Prints sha256 for each binary. Does NOT create
   the GitHub release (needs user push access).~~ ✓
10. ~~**`tunnelcat --version` smoke test** — `cmd/tunnelcat/main.go`
    is a real CLI binary that handles `--version`, `--help`,
    unknown subcommands, and unimplemented subcommands with the
    right exit codes. Verified end-to-end with `/tmp/tunnelcat`.
    Side effects of doing this: (a) found a NUL-terminator bug
    in the Rust version function (fixed in this commit);
    (b) found a cgo cache gotcha (Go doesn't re-link the .a
    unless the C header changes) — added `bin/rebuild.sh` to
    force a clean rebuild after Rust changes, and documented
    the gotcha in `internal/rustbridge/bridge.go` package doc.~~ ✓

---

## What's blocked (waiting on the user)

> Items that can't be done without a decision or action from the
> user. The agent lists them here so the user knows what to bring
> to the next session.

- **Q2: Repo identity.** Fork upstream (Option A, default) or
  new empty repo (Option B)? Needed before any GitHub push.
  Until decided, all commits stay local.
- **Q3: Number of devices.** 2? 5–10? 20+? Drives whether
  Stage 3 (coord server) is needed.
- **Q4: Cross-NAT or LAN-only?** Drives whether Stage 4 (DERP
  relay) is needed.
- **Q5: GitHub username.** So the agent can prepare the fork URL
  and the re-point script. (Not blocking Stage 1; only blocks
  the first push.)

---

## Where to look in the repo (grep-friendly index)

| I want to find... | Look here |
|---|---|
| **What is tunnelcat right now?** | `STATUS.md` (this file) |
| **The M0 closure (done)** | `canon/closures/M0-20260830.md` |
| **The M1 working plan (next)** | `canon/plans/M1-20260830.md` |
| The plan (stages, non-goals, novelty) | `canon/PROJECTS.md` |
| The long-term roadmap (M0–M8) | `canon/ROADMAP.md` |
| The tailcat API cheat sheet | `canon/TAILCAT-API.md` |
| The repo-local rules for the agent | `AGENTS.md` |
| How the network substrate works (the "why") | `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md` |
| The agent's reference card for networking | `~/.pi/agent/skills/networking-fundamentals/SKILL.md` |
| The 15-question self-test | `~/.pi/agent/knowledge/quizzes/networking-fundamentals.md` |
| The user study plan | `~/.pi/agent/knowledge/study-plans/tunnelcat-user.md` |
| The most recent session's handoff | `memory/sessions/` (newest first) |
| The Go test harness | `internal/testharness/` |
| The Rust crate | `crates/tunnelcat-proto/` |
| The Go↔Rust bridge | `internal/rustbridge/` |
| The fork-setup script | `bin/setup-fork.sh` |
| The release script | `bin/release.sh` |
| The rebuild script | `bin/rebuild.sh` |
| The macOS install script | `bin/install-mac.sh` |
| The Windows install script | `bin/install-windows.bat` |
| **The sibling project, tapauth** | `~/code_repo/tapauth/` (or `github.com/0ArchLinux0/tapauth`) — phone-as-authenticator; bootstrap only, no code yet |

---

## Quick state checks (for the user)

Run these in a terminal to see if the project is healthy:

```bash
# Is the build clean?
cd ~/code_repo/tailcat && go build ./... && go vet ./... && echo "OK"

# What did the agent do last?
cd ~/code_repo/tailcat && git log --oneline -5

# What's uncommitted right now?
cd ~/code_repo/tailcat && git status

# What did the agent do in a specific session?
cat ~/code_repo/tailcat/memory/sessions/<date>-<slug>.md
```

If `go build ./...` ever fails on a clean `main`, that's a bug in
the agent's last commit — flag it.

---

_Last updated: 2026-08-30 by the agent (after stage 0.5 handoff)._
