# Tunnelcat long-term roadmap

> **Last updated:** 2026-08-30 · **Owner:** you · **Substrate:** Go + Rust
> **This file is the long-term plan.** `canon/PROJECTS.md` is the
> per-stage plan; this file is the multi-month plan. Read this when
> you want to know "what is tunnelcat becoming?"
>
> The roadmap is **opinionated, ordered, and gated**. Each milestone
> has a gate — a real, testable condition that has to be true before
> we ship it. We do not skip gates. We do not promise features we
> can't deliver. We do not ship a "real working product" before the
> gate for that product is met.

---

## The north star

**Tunnelcat is a self-hosted, control-plane-free mesh VPN that any
technically-comfortable person can install on their own machines
without giving any company (us or Tailscale) an account.**

Specifically:

- **It works on a LAN without any external service.** Two devices
  on the same WiFi should discover each other in <1 second and
  tunnel with zero round-trips to anywhere.
- **It works across NAT with a relay the user controls.** The
  default relay is `tailcat.dev` (free, public). The user can swap
  in their own with one flag. No account required for either.
- **It works at any scale from 1 to 50 devices for the user's
  own machines.** Past 50, we point at headscale.
- **It has real novelty** (the type-state protocol, the key-as-
  identity model, the LAN discovery) that distinguishes it from
  "yet another WireGuard wrapper."
- **It is auditable** — the data plane is the audited
  `tailscale.com` code; the control plane is small enough for one
  person to read in a day.
- **It does not lock the user in.** Tokens are interoperable with
  upstream `tailcat`. Keys are interoperable with WireGuard. The
  user can leave at any time.

What we are NOT trying to be (this list is load-bearing):

- **Not a Tailscale competitor.** We will never have SSO, SCIM,
  device approval flows, admin console, audit logs, or any of
  the things a SaaS needs. If you want those, use Tailscale.
- **Not a corporate VPN.** No RADIUS, no LDAP, no per-user
  policies, no split-tunneling rules.
- **Not a Tor/I2P replacement.** Anonymity is not a goal.
- **Not a mesh routing protocol.** We're a transport-layer
  overlay, not a network-layer routing fabric.
- **Not multi-tenant.** One coord server = one user's devices.
  If you want to share a coord server with friends, fork.

---

## The milestone plan

> A milestone is a **shippable thing** — something a user can
> install, run, and tell their friend "look at this." Each
> milestone has a **gate** that has to be green before we ship it.

### M0 — The seed (✅ done as of 2026-08-30)

**Shippable thing:** a fork of `tailscale/tailcat` with a working
`tunnelcat up` and `tunnelcat dial` that forms a real WireGuard
tunnel between two of your own machines.

**Gate (met):**
- ✅ `go build ./...` and `go test ./...` are clean
- ✅ The end-to-end test `TestE2EEchoRoundTrip` passes in <2s
- ✅ 12 unit tests pass
- ✅ The user can `tunnelcat up` on machine A and `tunnelcat dial`
  on machine B and see the echo

**What's in M0:** `cmd/tunnelcat/main.go`, `cmd/tunnelcat/main_test.go`,
`cmd/tunnelcat/e2e_test.go`, the data plane imported from
`tailscale.com`, the Rust crate skeleton (`crates/tunnelcat-proto/`)
with a working cgo bridge, the FFI smoke tests.

**What M0 is NOT:** a "real working product" in the sense of
"ready for someone else to use." It works for *you* on *your*
machines. The next milestones are about making it work for
*your friend's* machines too.

---

### M1 — "I can give this to one friend" (target: 1–2 weeks)

**Shippable thing:** a friend can run a one-liner on their Mac
or Linux machine and connect to your tunnel. They don't need
a GitHub account, a Tailscale account, or a config file. They
need a token. The token can be a string, a QR code, or a
DNS TXT record.

**Gate:**
- M0 gate still green
- A working `tunnelcat install` (or equivalent) command that
  downloads a binary, verifies the SHA, places it on `$PATH`
- `tunnelcat identity init` creates a stable key per device
- `tunnelcat contact add <name> <pubkey>` adds a peer to a
  contacts file
- `tunnelcat dial <name>` dials by name (looks up the contact)
- `tunnelcat show --qr` prints a QR code of the device's
  connection token
- Cross-compiled binaries ship for macOS arm64, macOS amd64,
  Linux arm64, Linux amd64, Windows amd64
- A README with a "5-minute quickstart" that a non-author
  can follow without asking for help
- CI green on a real GitHub repo

**What's in M1 (the smallest version that satisfies the gate):**
- Stage 1.5: the Rust crate skeleton lands properly with
  cbindgen OR hand-written header (we picked hand-written
  in dev-ahead)
- Stage 2: identity file + contacts YAML
- Cross-compilation via `bin/release.sh`
- A real GitHub repo with a public release

**What M1 explicitly defers:**
- The Rust novelty (the type-state Meow) — that's M2
- Any kind of coord server — that's M3
- Multiple users sharing a coord server — that's M4
- LAN discovery — that's M6
- SSH / services — that's M6

**Exit criterion for M1:** a friend runs the install command,
you run a tunnel, they see the echo. No Tailscale account
involved. **This is the first release that someone other than
the author can use.**

---

### M2 — "It has real novelty" (target: 3–4 weeks after M1)

**Shippable thing:** the type-state Meow protocol, in a Rust
crate, called from Go via cgo. Compile-time guarantees that
invalid handshake states are unrepresentable. A real-world
example of "Rust where Rust is better, Go where Go is
better" composing cleanly.

**Gate:**
- M1 gate still green
- The Rust crate exports the Meow protocol with at least 3
  distinct type states (e.g. `MeowSent`, `MeowedReceived`,
  `TunnelUp`)
- A compile-fail test: trying to call `.receive_meowed()`
  before `.send_meow()` is a Rust compile error
- The Go cgo bridge is wrapped in a Go-native interface
- The `tailcat.Server` actually uses the Rust crate's Meow
  implementation (replacing `disco.go`'s hand-rolled format
  — this requires vendoring the change)
- The cheat sheet + deep-dive + study plan in the canon
  are updated to reflect the new architecture
- The end-to-end test still passes
- A blog-post-style writeup of "how we did this and why"
  for the harness book

**What's in M2:**
- Stage 1.5 (skeleton) → Stage 2 (full novelty)
- The `internal/rustbridge` package grows from 2 functions
  to ~10
- A new `crates/tunnelcat-proto/src/meow.rs` (~300 LOC)
- cbindgen revisited with the right config to auto-generate
  the C header (we tried and failed in M0; revisit now
  with the larger surface)
- The "what makes tunnelcat novel" section of the canon
  is updated to reflect what's actually shipped

**What M2 explicitly defers:**
- Coord server (M3)
- DERP relay (M3 — bundled with coord server)
- Multi-user / shared coord (M4)
- LAN discovery (M6)
- Web UI (M5+)

**Exit criterion for M2:** the README and the docs lead
with "we wrote a type-state VPN protocol in Rust and it
calls into the audited Tailscale data plane." That's the
defensible novelty statement.

---

### M3 — "I can spin up a coord server for my friends" (target: 5–7 weeks after M1)

**Shippable thing:** a friend can run `tunnelcat coord serve`
on a $5 VPS, and 3 of your devices + 2 of theirs can register
and find each other by name. The coord server signs peer
maps but never sees plaintext. If the coord server is
compromised, the attacker can DOS the rendezvous but cannot
read traffic or impersonate peers (key-as-identity).

**Gate:**
- M2 gate still green
- A working `tunnelcat coord serve` binary, ~2k LOC Go
- The coord server stores state in sqlite (one file)
- TLS via Let's Encrypt (auto-renew)
- Pre-shared registration token (one token per "fleet")
- `tunnelcat status` shows all online peers, with their
  public keys
- `tunnelcat ping <peer>` works, reports DERP vs direct
- The key-as-identity model is documented in the deep-dive
- End-to-end test runs with 3 simulated devices against a
  real coord server
- The DERP relay story is decided: either we ship a
  `tunnelcat-derper` config wrapper (using upstream
  `derper` with a config file) or we document "use
  `tailcat.dev` for now"
- A security review of the coord server's auth model
  (one person, not formal, but documented)

**What's in M3:**
- Stage 3 (coord server)
- Stage 4 (DERP relay wrapper or doc)
- Stage 5 (daemon mode: long-running `tunnelcat up --name=foo`
  that registers, keeps the peer map fresh, auto-reconnects)
- The identity file from Stage 2 grows a "fleet" concept
- The cheat sheet, deep-dive, and study plan get the
  M3-era versions

**What M3 explicitly defers:**
- Multi-user / shared coord (M4)
- LAN discovery (M6)
- Web UI (M5+)
- SSH (M6 — uses the services from M6)
- Mobile (M8+)

**Exit criterion for M3:** 3 friends, each with 1–3 devices,
all on a shared coord server, can `tunnelcat ping`, `tunnelcat
ssh <peer>`, and `tunnelcat status` against each other. **This
is the first release that feels like Tailscale.**

---

### M4 — "I can share it with 10 friends" (target: 7–9 weeks after M1)

**Shippable thing:** a single coord server can host multiple
"fleets" (groups of devices that trust each other). Each fleet
has its own registration token, its own peer map, its own ACL
defaults. Friends don't need their own coord server unless they
want isolation.

**Gate:**
- M3 gate still green
- The coord server supports multiple fleets
- Each fleet has independent registration tokens, peer maps,
  rate limits
- A `tunnelcat fleet create` and `tunnelcat fleet join` UX
- A simple admin API: list fleets, revoke devices, rotate
  tokens
- Per-fleet audit log (who registered when, from what IP)
- The coord server passes a 24-hour soak test with 3 fleets
  × 5 devices each, no leaks between fleets

**What's in M4:**
- Stage 3 extended with multi-tenant support
- A web admin UI for the coord server (a single HTML page,
  not a SPA — this is a personal-scale tool, not a SaaS)
- SQLite WAL mode for the coord server (better concurrency)
- Reverse proxy guidance (Caddy, nginx) for HTTPS

**What M4 explicitly defers:**
- LAN discovery (M6) — would be useful in a "friends at
  the same party" use case but M3+M4 is enough for the
  cross-internet case
- Web UI for `tunnelcat status` (this is for the *server*
  admin, not the *client* user; mobile-friendly client
  status is M7+)
- Per-user ACLs (M5+)
- iOS/Android (M8+)

**Exit criterion for M4:** 3 friends, each running their own
device(s) on a coord server you operate, can invite a 4th
friend to their fleet without you doing anything.

---

### M5 — "It's a real product" (target: 10–14 weeks after M1)

**Shippable thing:** a polished UX. Web dashboard, status
indicators, sensible error messages, install on a Raspberry
Pi, `tunnelcat doctor` for debugging, a `--json` output mode
for scripts, a systemd unit file, a Homebrew tap, a
`winget` package.

**Gate:**
- M4 gate still green
- Web dashboard for `tunnelcat status` (one page, server-rendered)
- `tunnelcat doctor` runs 10+ diagnostic checks (DERP reachability,
  key freshness, coord server connectivity, port conflicts, etc.)
- `--json` output for all commands that produce structured data
- `tunnelcat.service` (systemd) and `tunnelcat.plist` (macOS
  launchd) for the daemon
- A Homebrew tap at `0ArchLinux0/homebrew-tap` with `brew install tunnelcat`
- A `winget` package manifest
- The deep-dive, cheat sheet, study plan, and roadmap are
  all up to date

**What's in M5:**
- All stages 1–5 polished
- A `tunnelcat doctor` subcommand
- A web dashboard (~500 LOC, single HTML page)
- A tap repo (`homebrew-tunnelcat`) with one Formula
- A winget PR upstream

**Exit criterion for M5:** a non-author can `brew install tunnelcat`,
run `tunnelcat coord serve` on a $5 VPS, register 2 devices, and
do `tunnelcat ssh <peer>`. **Without asking you a single question.**

---

### M6 — "It feels like magic on the same network" (target: 12–16 weeks after M1)

**Shippable thing:** when two tunnelcat devices are on the same
LAN, they discover each other via mDNS in <1s and tunnel directly,
no DERP, no coord server needed. The user doesn't need to know
they're on the same LAN — it just works.

**Gate:**
- M5 gate still green
- `tunnelcat status` shows LAN peers with a "📡 direct LAN"
  indicator (vs "🌐 via DERP")
- New device on the LAN appears in `tunnelcat status` within
  1 second
- The `crates/tunnelcat-proto/` gets a small LAN-discovery
  module (still in Rust, for the type safety)
- A "local" mode where no coord server is needed at all
  (2 devices, LAN only, no internet)
- Documentation explains the LAN discovery architecture
  (mDNS announcement + subscription, fallback to DERP
  when mDNS doesn't work)
- A new test: 2 devices on the same LAN tunnel in <1s without
  any DERP traffic

**What's in M6:**
- Stage 6 from the original plan (services + LAN discovery)
- mDNS implementation in Rust
- An opt-in "LAN-only" mode for users who don't want any
  external traffic

---

### M7 — "I can use it for my job" (target: 16–20 weeks after M1)

**Shippable thing:** a set of "I use this at work" features:
per-device ACLs, audit log, key rotation, multiple coord servers
for redundancy, health checks, observability hooks.

**Gate:**
- M6 gate still green
- Per-device ACLs (allow/deny rules in a YAML file)
- `tunnelcat rotate` generates a new key, re-registers,
  revokes the old one (no manual coordination needed)
- The coord server supports running in HA mode (2+ nodes,
  state replicated via a shared sqlite-over-NFS or a real
  consensus protocol — to be decided)
- `tunnelcat status --json` includes enough info for
  monitoring (Prometheus exporter optional)
- A formal threat model document

---

### M8+ — "It might be a product" (target: 6+ months after M1)

**Things on the table but not committed:**

- **iOS / Android clients.** Probably via wrapping tailcat's
  existing mobile clients, not reimplementing. The Tailscale
  iOS app is BSD-licensed.
- **Web UI for the client side** (a status dashboard you
  can hit at `http://localhost:9090`).
- **Tailscale import.** A `tunnelcat import-ts` that takes a
  Tailscale tailnet export and recreates it on tunnelcat.
- **Headscale interop.** Make tunnelcat a drop-in coord
  server client for headscale, so users can use headscale
  for the multi-tenant bits and tunnelcat for the data plane.
- **A real GUI for the coord server.** (Probably a single
  React page; not a SaaS.)
- **Native packages** (apt repo, dnf repo, scoop bucket).
- **Documentation site** (mkdocs or docusaurus).
- **Conference talks / blog posts.** "We wrote a type-state
  VPN in Rust that calls into the Tailscale data plane."

---

## The "is this real?" gates (cross-cutting)

These are checks that apply at every milestone, not just
specific ones. **A milestone is not done until all of these
are green.**

| Gate | What | How we check |
|---|---|---|
| **Build clean** | `go build ./...` exits 0 | `bin/rebuild.sh && go build ./...` |
| **Vet clean** | `go vet ./...` exits 0 | Same as above |
| **Tests pass** | All unit tests + the e2e test pass | `TUNNELCAT_E2E=1 go test -count=1 ./...` |
| **Coverage** | ≥70% on the atomic path | `go test -cover` |
| **gofmt clean** | `gofmt -l .` is empty | CI |
| **No `unsafe` outside the FFI shim** | `grep -r unsafe --include="*.go"` shows only the rustbridge | `grep` |
| **No commits to upstream tailcat source** | `tailcat.go` and `disco.go` are unchanged from the fork point | `git diff upstream/main -- tailcat.go disco.go` |
| **No `networking-layer:` tag missing** | Every commit touching network code has the tag | `git log --grep='^networking-layer:'` |
| **No secrets in the repo** | No tokens, keys, or credentials committed | `gitleaks` or manual grep |
| **The "two devices can talk" demo works** | Real two-machine test passes | Manual, before each release |

---

## The 12-month picture

```
  Month:        1    2    3    4    5    6
                │    │    │    │    │    │
  M0 seed       ████
  M1 friend     ────████
  M2 novelty    ────────████
  M3 friends    ────────────████
  M4 share      ────────────────████
  M5 polish     ────────────────────████
  M6 LAN        ────────────────────────████
  M7 work       ────────────────────────────████
  M8+ mobile    (start when M5 is in daily use)
```

**M0–M2 (months 1–2):** you use it, you show one friend.
**M3 (month 3):** you and 3 friends use it daily.
**M4 (month 4):** 10 friends can join.
**M5 (month 5):** non-authors can install and use it.
**M6 (month 6):** LAN discovery lands, "it feels like magic."
**M7+ (month 6+):** work features, mobile, optional polish.

**We are not in a hurry.** Each milestone gates the next.
If M1 takes 4 weeks, M2 takes 8 weeks, and the project
takes 12 months, that's fine. The goal is a tool we trust
and that other people can use, not a deadline.

---

## What could make us ship faster (and what could make us ship slower)

**Faster:**
- You have a specific friend in mind for M1 testing (gives
  us a real test case)
- You have a $5 VPS already configured (no setup friction
  for M3)
- You already have a domain + DNS (no waiting for cert
  provisioning for M3)
- The Go and Rust ecosystems stay stable (no surprise
  breaking changes in `tailscale.com`)

**Slower:**
- We hit a real bug in `tailscale.com`'s data plane that
  requires upstream work (we have to file an issue and wait)
- We discover our security model has a hole and need to
  redesign (M3 may slip)
- The user is busy and the project sits for weeks at a time
  (totally fine, we just stop where we are)
- The novelty (M2) turns out to need more iteration than
  planned (type-state machines are tricky to design well)

**Honest assessment:** M0 is done. M1 is realistic in 1–2
weeks of focused work. M2 in 3–4 weeks. M3 in 5–7 weeks.
M4 in 7–9 weeks. M5 in 10–14 weeks. M6 in 12–16 weeks.

If you do this in your spare time, expect 6–9 months to
M5. If you focus, expect 2–3 months. **The plan is the
plan; the pace is yours.**

---

## How this roadmap updates

This file is appended to, not edited, when the plan changes.
Each significant change gets a dated section at the bottom:

```
## Updates

### 2026-08-30 — initial roadmap
- ...
```

When a milestone ships, we record it:

```
### 2026-08-30 — M0 shipped
- M0 gate met at commit <hash>
- ...
```

We do **not** delete old sections. The plan is the history
of the plan. If you want to see what we thought M3 would
look like in August 2026, you scroll down.

---

_Last updated: 2026-08-30._
