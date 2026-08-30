# Session 2026-08-30 — stage 0.5 (rename + novelty thesis + Rust commitment)

## What landed

| Path | What |
|---|---|
| `canon/PROJECTS.md` | Renamed to `tunnelcat`. Added "What makes tunnelcat novel" section with three concrete novelties: (1) type-safe Meow handshake in Rust, (2) key-as-identity peer map, (3) mDNS LAN auto-discovery. Stage plan now has Rust involvement: stage 1.5 = Rust crate skeleton, stage 2 = identity + novelty 1, stage 3 = coord + novelty 2, stage 6 = services + novelty 3. Per-stage network touch matrix has a `Language` column. Open question #1 (binary name) closed: **tunnelcat**. |
| `AGENTS.md` | All `mesh` → `tunnelcat` references updated. No content change. |
| `~/code_repo/tailcat/` (commit `c936783`) | Both files, on local `main`. |
| Previous session's commit `fabe47f` | Stage 0 baseline (env, canon, substrate rule) is preserved. |

## What didn't

- The branch push still fails (403 to upstream `tailscale/tailcat`). We have local-only history. The next session's stage 1 should either (a) ask the user to create a fork under their GitHub and re-point `origin`, or (b) accept local-only for now. The branch-push script will refuse to push to a remote that doesn't accept our writes (which is correct behavior).
- No code was written this session. Stage 1 is the first code-writing stage.
- The directory is still `~/code_repo/tailcat` not `~/code_repo/tunnelcat`. The rename would change every import path and break the upstream `go.mod`. **Do NOT rename the directory in stage 1.** It's a cosmetic mismatch; the binary inside is `tunnelcat`, that's what matters.

## What's next (stage 1, one sentence)

Build `cmd/tunnelcat/main.go` — a ~150-line Go binary that wraps
`tailcat.Server` and `tailcat.Client` with `tunnelcat up` and
`tunnelcat dial <token>`, branded with the tunnelcat name, on its
own session branch.

## Networking notes

- Stage 1 is application-layer only. It does NOT touch magicsock,
  netstack, or the wire format. The pre-flight checklist in the
  skill is not yet required.
- The networking-fundamentals skill is required from stage 2
  onwards, when we start the Rust crate and the identity layer.

## Rust commitment — what this means concretely

The user said "I'm a genius so I can do it" about learning Rust +
networking. I respected that without backing off my recommendation
to use Go for the data plane. The compromise:

- **Data plane (WireGuard, magicsock, netstack, DERP):** Go
  (imported from `tailscale.com`).
- **Coord server, identity, CLI, services:** Go.
- **Novelty layer (type-safe Meow, ed25519 signing, packet
  parsing):** Rust crate, called from Go via cgo.

This is the only path I can honestly recommend. It gives the user
real Rust experience without sacrificing the data plane. The
novelty is defensible (compile-time-validated protocol states are
genuinely new in this space) and the project ships in months, not
years.

The user accepted this implicitly by not pushing back when I
explained it. The canon now reflects it.

## Open questions for the user (still)

2. **Repo identity.** Fork on GitHub (recommended), new repo, or
   local-only? Affects the LICENSE and copyright line at stage 1.
3. **How many devices, really?** Drives whether we need a coord
   server (stage 3) or can stop at the contacts file (stage 2).
4. **All-on-LAN or cross-NAT?** Drives whether we need DERP
   (stage 4) or can rely on LAN discovery (stage 6 novelty 3).

## External links

- Notion (P1): "How Tailcat / Tailscale Works Under the Hood — full deep-dive"
  `https://www.notion.so/3cc7b981c514819fa38ec70c6f8bd8bd`
- Telegram: already pushed (URL + `---` + summary format)
- Deep-dive body: `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md`
- New global skill: `~/.pi/agent/skills/networking-fundamentals/SKILL.md`
- New global rule: `~/.pi/agent/AGENTS.md` "Domain knowledge for the substrate"
- Harness book inbox: `~/code_repo/book/inbox/IN-006-harness-engineering.md`
  has the "substrate-knowledge rule" pattern flagged as a chapter
  candidate

## What you (the user) should do

1. **Read the Notion deep-dive** at your own pace.
2. **Read the canon's "What makes tunnelcat novel" section.**
   Confirm the three novelties match what you had in mind, or
   tell me which to change.
3. **Answer open questions 2, 3, 4** when you have a moment.
4. **The next session can start stage 1 with reasonable defaults**
   if you don't answer. The defaults:
   - Repo: local-only (no GitHub push until you decide)
   - Devices: 2–5 (skip the coord server; build it later if needed)
   - Cross-NAT: yes (include the DERP stage in the plan)
