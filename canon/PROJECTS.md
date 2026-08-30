# Tunnelcat — full mesh VPN built on tailcat's data plane

> **Last updated:** 2026-08-30 · **Status:** 🟡 Stage 0.5 (M0 shipped, planning M1)
> **Repo:** `~/code_repo/tunnelcat` (will be moved; currently at `~/code_repo/tailcat`)
> **Owner:** you · **Substrate:** Go 1.27 + `tailscale.com` data plane + a small Rust crate for the novelty layer (protocol types, crypto, parsing)
>
> **Long-term roadmap:** see `canon/ROADMAP.md` for the M0–M8
> plan, the gates, the 12-month timeline, and the "what we
> are NOT building" list. This file is the per-stage plan;
> the roadmap is the multi-month plan.

---

## The single active goal

**Tunnelcat — build a control-plane-free full mesh VPN for ~2–20 personal
devices, by extending tailcat's data plane with (a) a tiny optional
coordination server and (b) a friendlier CLI/identity layer, in Go.**

Everything else is either feeding it or parked.

---

## Why this exists (one paragraph)

Tailcat today is a 1:1 netcat-over-WireGuard tool with no mesh, no identity,
and no way for N devices to find each other without N×(N-1)/2 tokens being
passed out-of-band. We want Tailscale-the-product's "log in once, see all my
machines" experience without Tailscale's SaaS control plane. Tailcat already
gives us the entire data plane (WireGuard, magicsock, DERP, netstack, disco
handshake). The missing pieces are a coordination layer and a friendlier
surface — both small.

## What we are NOT building

- A public Tailscale competitor. The coordination server is for our own
  devices, not a SaaS.
- A kernel-mode WireGuard driver. Userspace only (netstack).
- A replacement for the Tailscale data plane. We `import tailscale.com`.
- iOS/Android native clients. CLI + web UI only.
- ACL engine, MagicDNS, subnet routers, exit nodes — all later stages,
  all opt-in, all feature-gated per the atomic-to-heavy skill.

## What makes tunnelcat novel (and not just "tailcat with a different name")

Three concrete novelties, each in a separate stage so we ship each one
and learn from it before adding the next:

### Novelty 1 — Type-safe Meow handshake (Rust crate, stage 2)

The current Meow protocol in `tailcat/disco.go` is a hand-rolled byte
format. It works, but it has no compile-time guarantees. **We replace
it with a Rust crate** that uses Rust's type system to make invalid
states (e.g. "Meow sent but Meowed received before the deadline",
"signature verified but counter < last seen") unrepresentable at
compile time. The crate is called from Go via cgo, so the data plane
stays in `tailscale.com` and the novelty lives in the protocol layer.

**Why this is real novelty:** the type-state pattern in Rust has been
used in protocol implementations (e.g. `quinn-proto`, `rust-libp2p`),
but not in a Tailscale-style data plane. We get to claim the
"type-safe VPN protocol" niche and show concrete compile-time errors
for protocol violations.

### Novelty 2 — Hybrid identity model (Go, stage 3)

Tailscale's identity is "your Tailscale account owns these devices."
Headscale's identity is "this user is alice, this user is bob." Ours
is different: **identity is the WireGuard public key itself**, and
the contact list is a signed, append-only log distributed peer-to-peer
(not pulled from a central server). No coord server is the source of
truth for identity — the key IS the identity.

**Why this is real novelty:** removes the "what if the coord server
is compromised" failure mode entirely. The cost: more careful key
management UX. The benefit: zero-trust identity that doesn't need
a SaaS.

### Novelty 3 — Local-network auto-discovery (Go, stage 6+)

When two tunnelcat devices are on the same LAN, they shouldn't need
to talk through DERP or hole-punch at all — they're already reachable
directly. We add an mDNS-based discovery layer that detects
"this peer is on my LAN, just use the direct local address" and
skips the relay. The connection establishment is ~5ms instead of
~200ms.

**Why this is real novelty:** Tailscale has tailnet-local discovery
but it goes through the coordination server. Headscale doesn't have
it. Zero-tier has something like it but only on their own subnet.
A direct peer-to-peer LAN discovery that doesn't need a server at
all is genuinely missing from the ecosystem.

### What we are NOT claiming as novelty

- "Faster than Tailscale" — no, we use the same data plane.
- "More secure than Tailscale" — we use the same WireGuard + Curve25519.
- "Cross-platform" — same as Tailscale (Linux/macOS/Windows).
- "Open source" — yes, but so is everything in this space.

The novelty is the **protocol type safety** + **key-as-identity** +
**LAN discovery** triangle. Three small, defensible, novel things
that compose into a real product. That's the thesis.

---

## Project inventory (this repo's neighbors)

| # | Project | Path | Status | What it owns | Last touched | Note |
|---|---|---|---|---|---|---|
| 1 | **tunnelcat** (this) | `~/code_repo/tailcat` (→ will rename dir to `tunnelcat`) | 🟡 Stage 0 | VPN mesh built on tailcat | 2026-08-30 | Fork of `tailscale/tailcat`; we add a `cmd/tunnelcat/` binary and a `coord/` package |
| – | tailcat upstream | `github.com/tailscale/tailcat` | 🟢 | The data plane we import | upstream | Pin a specific tag; rebase monthly |
| – | `tailscale.com` | Go module | 🟢 | WireGuard, magicsock, DERP, netstack | n/a | Imported as a Go module — do not fork |

---

## Architecture (lock this in early, change it later only with reason)

```
                    ┌────────────────────────┐
                    │  coord server (ours)   │  optional, one binary, ~1k LOC
                    │  - "who is online"     │  - in-memory or sqlite
                    │  - signed peer list    │  - signs peers with ed25519
                    │  - no traffic relay    │  - never sees encrypted WG bytes
                    └──────────┬─────────────┘
                               │  HTTPS (signed peer map)
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
        ┌──────────┐    ┌──────────┐    ┌──────────┐
        │ device A │    │ device B │    │ device C │
        │ tunnelcat CLI │    │ tunnelcat CLI │    │ tunnelcat CLI │
        │ netstack │    │ netstack │    │ netstack │
        │ magicsock│◄──►│ magicsock│◄──►│ magicsock│   direct P2P
        │ wireguard│    │ wireguard│    │ wireguard│   over WireGuard
        └──────────┘    └──────────┘    └──────────┘
                               ▲
                               │  fallback only
                        ┌──────┴──────┐
                        │  DERP relay │  ours or public tailcat.dev
                        └─────────────┘
```

**Key contract:** the coord server never sees plaintext. It only signs and
distributes the public keys + allowed IPs of each peer. All actual traffic
is device-to-device over WireGuard, just like tailcat today.

---

## Stage plan

> Ordered by **what blocks what**, not by what's easiest.
> Each stage ends with a runnable thing on a session branch.

### Stage 0 — cold start, env, baseline build (this session's job)

- **Goal:** `go build ./...` succeeds in this repo, and the next session
  can cold-start from this file in under 60 seconds.
- **Where:** `~/code_repo/tailcat/canon/PROJECTS.md` (this file),
  `go.mod`, `go.sum`.
- **Change:** write the canon, install Go, verify upstream tailcat
  builds clean as the baseline.
- **Verify:** `go build ./...` exits 0; `go vet ./...` exits 0;
  `git status` clean except for `canon/` and `memory/`.
- **Commit:** `chore(tunnelcat): stage 0 — env, canon, baseline build`
- **Non-goals:** adding any new code; changing upstream tailcat source;
  picking a name for the final binary (do that in stage 1).

### Stage 1 — name, brand, and the smallest possible delta

- **Goal:** the repo has a working `cmd/tunnelcat/` binary that wraps
  `tailcat.Server` and `tailcat.Client` with a single subcommand:
  `tunnelcat up` and `tunnelcat dial <token>`. Same data plane as
  tailcat, just our own CLI surface. Binary name: **tunnelcat**.
- **Where:** `cmd/tunnelcat/main.go`, `README.md` (overwrite upstream's),
  `LICENSE` (keep BSD-3, add our copyright line).
- **Change:** ~150 lines of Go. No new data-plane code.
- **Verify:** `go run ./cmd/tunnelcat up` prints a `tc…` token;
  `go run ./cmd/tunnelcat dial <token>` connects and pipes stdin/stdout
  to the upstream side. Cross-machine test on two of your own devices.
- **Commit:** `feat(tunnelcat): stage 1 — branded CLI wrapping tailcat`
- **Non-goals:** mesh features; coord server; UI; auth. Pure cosmetic +
  ownership change.

### Stage 1.5 — set up the Rust crate skeleton (novelty prep)

- **Goal:** the repo has a working `crates/tunnelcat-proto/` Rust crate
  with a `Cargo.toml`, a `lib.rs` that compiles, and a cgo shim
  in `internal/rustbridge/` that calls it. No novel behavior yet;
  this stage just proves the Go↔Rust boundary works.
- **Where:** `crates/tunnelcat-proto/`, `internal/rustbridge/`.
- **Change:** ~200 LOC (most is boilerplate). Uses `cbindgen` to
  generate C headers from the Rust side, and cgo to call them
  from Go.
- **Verify:** `cargo build` succeeds; `go build ./...` succeeds
  with the cgo dependency; a smoke test in `internal/rustbridge/`
  exports a function from Rust, imports it in Go, calls it,
  and asserts the round-trip.
- **Commit:** `chore(rust): stage 1.5 — crate skeleton + cgo bridge`
- **Non-goals:** any protocol logic yet; this is plumbing only.

### Stage 2 — identity + the **first novelty**: type-safe Meow handshake

- **Goal:** two things in one stage.
  1. `tunnelcat identity init` creates a stable node key per
     device; `tunnelcat identity show` prints the public key;
     `~/.config/tunnelcat/contacts.yaml` lists known peers.
  2. The Meow protocol in `disco.go` is replaced by a Rust crate
     that uses **type-state** to make invalid handshake states
     unrepresentable. The Rust crate is called from Go via cgo.
- **Where:** `internal/identity/`, `cmd/tunnelcat/identity.go`,
  `crates/tunnelcat-proto/src/meow.rs`,
  `internal/rustbridge/meow.go`.
- **Change:** ~500 LOC Go + ~300 LOC Rust.
- **Verify:** same as before (A→B handshake still works) PLUS a
  compile-time test: trying to call `meow.receive_meowed()`
  before `meow.send_meow()` is a Rust compile error.
- **Commit:** `feat(meow,identity): stage 2 — Rust type-state
  Meow + identity file`
- **Non-goals:** automatic discovery; QR codes; key rotation UI;
  the actual `tailcat.Server` still uses its own Meow (we replace
  it in stage 3).
- **This is Novelty 1 from the "What makes tunnelcat novel" section.**

### Stage 3 — coordination server (headscale-lite) + **Novelty 2: key-as-identity**

- **Goal:** single Go binary `tunnelcat-coord` (the headscale-lite),
  but with the key-as-identity model: the server signs the peer
  map, but never stores identity — the public key IS the identity.
  No usernames, no accounts, no OAuth. The contact list from
  stage 2 is the source of truth for trust; the coord server
  is just a rendezvous point.
- **Where:** `cmd/tunnelcat-coord/main.go`, `internal/coord/`,
  `internal/coord/schema.sql`.
- **Change:** ~800–1200 LOC Go.
- **Verify:** run `tunnelcat-coord` on one box; on two others,
  set `TUNNELCAT_COORD=https://that-box:8443` and `tunnelcat up`
  registers automatically; `tunnelcat ping <peer-name>` finds
  the peer by name, not by token.
- **Commit:** `feat(coord): stage 3 — coord server + key-as-identity`
- **Non-goals:** auth beyond a shared pre-shared registration token;
  multi-user support; web UI; horizontal scaling.
- **This is Novelty 2 from the "What makes tunnelcat novel" section.**

### Stage 4 — DERP relay (only if needed)

- **Goal:** a single Go binary `tunnelcat-coord` that:
  1. Listens on `:443` (or `:8443`) with a self-signed TLS cert.
  2. Accepts `POST /register` from a device with its public key.
  3. Returns the signed peer map (all other registered peers' pubkeys
     + their last-known DERP region).
  4. Stores state in a single sqlite file (`coord.db`).
  5. Re-signs the peer map every 30s; clients refetch every 5 min.
- **Where:** new `cmd/tunnelcat-coord/main.go`, new `internal/coord/`
  package, new `internal/coord/schema.sql`.
- **Change:** ~800–1200 LOC. One binary, one sqlite file, no
  external dependencies. Ed25519 signatures, no JWT, no OAuth.
- **Verify:** run `tunnelcat-coord` on one box; on two others, set
  `TUNNELCAT_COORD=https://that-box:8443` and `tunnelcat up` registers
  automatically; `tunnelcat ping <peer-name>` finds the peer by name,
  not by token. Both peers show each other in `tunnelcat status`.
- **Commit:** `feat(coord): stage 3 — single-binary coord server`
- **Non-goals:** auth beyond a shared pre-shared registration token;
  multi-user support; web UI; horizontal scaling.

### Stage 5 — first "real" mesh: persistent listen + auto-reconnect

- **Goal:** ship a `tunnelcat-derper` config and a flag
  `--derp=<your-host>` that points both `tunnelcat` and `tunnelcat-coord` at
  your own relay instead of the public one. We do NOT write a new
  DERP server; we wrap `tailscale.com/cmd/derper` in a
  single-binary `tunnelcat-derper` that bundles a config file.
- **Where:** `cmd/tunnelcat-derper/`, `docs/derp.md`.
- **Change:** ~100 LOC (mostly a config wrapper + a systemd unit file).
- **Verify:** stand up `tunnelcat-derper` on a $5 VPS, point two devices
  at it with `TUNNELCAT_DERP=relay.example.com:443`, confirm
  `tunnelcat ping --until-direct` still works.
- **Commit:** `feat(derp): stage 4 — own DERP relay (wrapper)`
- **Non-goals:** writing a new DERP server; load-balancing multiple
  relays; HA. Skip if you can use `tailcat.dev` for free.

### Stage 6 — services + **Novelty 3: LAN auto-discovery**

- **Goal:** two things.
  1. A YAML file (`~/.config/tunnelcat/services.yaml`) maps
     `studio-mac:http` → `100.64.0.2:8080`. `tunnelcat resolve
     studio-mac` returns the IP. `tunnelcat ssh studio-mac` dials
     the right port. `tunnelcat curl http://studio-mac:8080` works.
  2. **LAN auto-discovery via mDNS.** When two tunnelcat devices
     are on the same network, they announce themselves as
     `_tunnelcat._udp.local` and skip DERP / hole-punching
     entirely. Connection establishment drops from ~200ms to ~5ms.
- **Where:** `internal/services/`, `internal/discovery/`,
  `cmd/tunnelcat/{resolve,ssh,curl}.go`.
- **Change:** ~500 LOC Go.
- **Verify:** on a single LAN, two devices see each other in
  `tunnelcat status` within 1 second of coming online, with
  no DERP traffic. (Run `tunnelcat --verbose status` to
  confirm the "via DERP" / "direct LAN" labels.)
- **Commit:** `feat(services,discovery): stage 6 — services YAML + mDNS`
- **Non-goals:** full MagicDNS (no `.tunnelcat` TLD); UDP services
  yet (TCP first).
- **This is Novelty 3 from the "What makes tunnelcat novel" section.**

### Stage 7+ — parked until stage 6 is in daily use

- UDP service tunneling
- Reverse port forwards (expose a port on a peer)
- File transfer / `mesh send`
- Web UI for `tunnelcat status` and the coord server
- Mobile clients (probably never — use tailcat on iOS for that)
- Per-device ACLs (currently binary: allow or don't)

---

## The atomic-to-heavy contract (apply from Stage 1)

- **One binary** (`tunnelcat`) covers all client behavior. `tunnelcat up`,
  `tunnelcat dial`, `tunnelcat identity`, `tunnelcat status`, `tunnelcat ssh`, `mesh
  resolve`. Not a separate binary per command.
- **Default invocation is atomic.** `tunnelcat up` with no flags =
  register with coord, listen on the standard port set, and exit
  when SIGINT. No daemon, no config file, no env vars required.
- **Heavy features feature-gated.** The coord server, services YAML,
  ACLs, web UI, and DERP config all live behind flags. Stripped
  builds (no tag) stay under 20 MB.
- **Coverage floor: 70% lines on the atomic path.** CI fails below
  this. Heavy paths are exempt.
- **Docs outlive the session.** Every new flag → one new section in
  `docs/`. Every removed feature → section deleted (not orphaned).
  Every doc page has a `Last verified:` line.

## Why Go, not Rust (for the record)

- We import `tailscale.com` (WireGuard, magicsock, DERP, netstack).
  Reimplementing any of these in Rust is a 6–18 month detour.
- The hot path (packet processing) is already C-callable from Go via
  the wireguard-go submodule. If a profiler later says "rewrite X
  in Rust for speed", we do that surgically. Not before.
- Tailscale themselves ship Go. We are not smarter than they are.

---

## Conventions for the agent working on this

- **One session = one stage.** Don't start stage N+1 until stage N
  has shipped a commit on its own session branch.
- **Never push to `main`.** Use `~/.pi/agent/bin/branch-push
  ~/code_repo/tailcat <slug>`. All PRs target `main` after user
  review.
- **Never edit `tailcat.go` directly.** If we need a data-plane
  change upstream doesn't have, file an issue against
  `tailscale/tailcat` first. We can vendor a patch in `internal/`
  while we wait.
- **Atomic-to-heavy violations get pushed back, not negotiated.**
  See `~/.pi/agent/skills/atomic-to-heavy/SKILL.md`.
- **The network IS the substrate.** Before starting any new stage,
  the agent must produce a 2–4 sentence "Why this stage touches
  the network" note that names the OSI layer, transport
  protocol, and the specific primitive from `tailscale.com` or
  `gvisor.dev` being composed. This is enforced by
  `AGENTS.md` and the `networking-fundamentals` skill. See
  `AGENTS.md` §2 and `~/.pi/agent/skills/networking-fundamentals/SKILL.md`.

## Per-stage "network touch" matrix

This table makes the substrate knowledge explicit per stage. The
agent must read the linked skill sections before starting each
stage. The matrix is grep-friendly: `grep "stage N" canon/PROJECTS.md`.

| Stage | OSI layer | Transport | Language | Substrate primitive | Skill sections to read |
|---|---|---|---|---|---|
| 0 — baseline build | n/a | n/a | Go | `go build` | none |
| 1 — branded CLI | application | n/a (wraps tailcat) | Go | `tailcat.NewClient`, `tailcat.Server` | §13 |
| 1.5 — Rust crate skeleton | n/a | n/a | Rust + cgo | `cbindgen`, `cargo`, `cgo` | none yet (read Rust chapters of the networking book) |
| 2 — identity + Novelty 1 (type-state Meow) | application | UDP (disco) | **Go + Rust** | `ed25519-dalek` (Rust), `tailcat/disco.go` (Go) | §5, §10, §11 |
| 3 — coord server + Novelty 2 (key-as-identity) | application | TCP (HTTPS) | Go | `net/http`, `crypto/ed25519` (Go) | §2, §3, §7, §10 |
| 4 — DERP relay | application + transport | UDP (DERP + HTTPS) | Go | `tailscale.com/cmd/derper` | §4, §5, §7, §12 (tcpdump) |
| 5 — daemon + auto-reconnect | application + transport | UDP (magicsock) | Go | `magicsock.Conn` event loop | §7, §8, §9, §11 |
| 6 — services + Novelty 3 (LAN discovery) | application | UDP (mDNS multicast) | Go | `github.com/grandcat/zeroconf` (mDNS) | §7, §9, §12 |
| 7+ (parked) | varies | UDP (currently TCP-only) | TBD | TBD | §5 (UDP) |

---

## Open questions (resolve before Stage 1)

1. **Binary name.** ✅ **Decided: `tunnelcat`.**
2. **Repo identity.** Do we (a) keep this as a fork under your personal
   GitHub, (b) make a new repo (recommended — we already have a
   not-tailscale-upstream identity), or (c) just keep it local? Affects
   the LICENSE and copyright line in stage 1.
3. **How many devices, really?** If it's 2, we can skip Stage 3
   (coord server) entirely — tailcat + a contacts file is enough.
   If it's 5+, coord is worth it. If 20+, consider headscale instead
   of building it ourselves.
4. **All-on-LAN or cross-NAT?** If your devices are mostly on the
   same WiFi, Stage 4 (own DERP) can be skipped — they all talk
   directly. DERP is only for the cross-NAT case.

---

## Cross-references

- `~/.pi/agent/skills/atomic-to-heavy/SKILL.md` — design philosophy
- `~/.pi/agent/skills/plan-with-stages/SKILL.md` — this file's format
- `~/.pi/agent/knowledge/PROJECTS-SUMMARY.md` — add a row here when
  stage 1 ships
- `https://github.com/tailscale/tailcat` — the data plane we extend
- `https://github.com/juanfont/headscale` — reference for the
  control plane we're shrinking in stage 3
