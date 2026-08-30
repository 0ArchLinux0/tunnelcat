# AGENTS.md — tunnelcat

> **This file is auto-loaded for any agent session that opens a
> file in this repo.** It is the local rule layer. It is
> deliberately short: the project canon (`canon/PROJECTS.md`)
> owns the "what we're building," this file owns the "what
> you must do as an agent while building it."

---

## 1. Cold-start recipe (read this first, every session)

1. `cat canon/PROJECTS.md` — the plan, the stages, the
   non-goals. Read the "single active goal" line first.
2. `ls -t memory/sessions/*.md 2>/dev/null | head -1 | xargs cat` —
   the most recent handoff. This is what the previous session
   actually shipped and where the next session should pick up.
3. `git log --oneline -20` — what was actually committed.
4. `git status` — what is uncommitted or untracked.
5. `go build ./... && go vet ./...` — is the baseline still clean?

If any of these fails, **stop and surface it.** Do not start
the next stage from a broken baseline.

## 2. The networking-knowledge rule (HARD CONSTRAINT)

**This project is built of the network, not just on it.** The
substrate — IP, TCP, UDP, NAT, WireGuard, magicsock, disco,
netstack, DERP — IS the product. An agent that writes VPN
code without internalizing the network model is guessing,
and the user will see it.

### 2.1 Before any new stage: the "why this touches the network" note

At the **start of any new stage** in `canon/PROJECTS.md`, the
agent's first message must include a 2–4 sentence section
titled `### Why this stage touches the network` that names:

- Which OSI layer (1–4) the stage operates at
- Which transport protocol (TCP, UDP, or both) and why
- Which existing primitive in `tailscale.com` or `gvisor.dev`
  the stage is composing with

If the agent cannot write that section from memory, the
correct action is to read
`~/.pi/agent/skills/networking-fundamentals/SKILL.md` first,
not to start coding.

### 2.2 Before any commit that touches network code: the pre-flight checklist

Memorize the 7-item pre-flight checklist at the bottom of the
networking-fundamentals skill. Before any commit that
modifies a `.go` file containing `net.`, `tailcat.`,
`magicsock.`, `wgengine.`, `netstack.`, or `disco.`, the
agent must be able to answer all 7 questions about that file.

If the agent cannot, the commit gets a `WIP` prefix and a
note in the handoff explaining what's still unverified.

### 2.3 Diagnostic reflex

If a bug takes more than 10 minutes to diagnose, the agent
must run `tcpdump` (or `wireshark`, or `go test` with a
printout) before adding more code. Network bugs are observed,
not guessed at. The diagnostic command set is in the skill.

## 3. The atomic-to-heavy rule

The project follows `~/.pi/agent/skills/atomic-to-heavy/SKILL.md`.
Short version:

- **One binary, two modes.** Don't fork the project to add a
  feature. Feature-gate it.
- **Default = atomic.** `tunnelcat up` with no flags does the
  minimum useful thing.
- **Coverage floor: 70% lines on the atomic path.** CI fails
  below this.
- **Docs outlive the session.** New flag → new section in
  `docs/`. Removed feature → section deleted. Every doc page
  has a `Last verified:` line.

## 4. Session discipline

- **One session = one stage.** Don't start stage N+1 until
  stage N has shipped a commit on its own session branch.
- **Never push to `main` directly.** Use
  `~/.pi/agent/bin/branch-push ~/code_repo/tailcat <slug>`.
- **Never edit `tailcat.go` directly** (the upstream vendored
  file). If we need a data-plane change, file an issue
  upstream and vendor the patch in `internal/`.
- **Every commit message includes a `networking-layer:`
  tag** in the body for any commit that touches network
  code, e.g.:

  ```
  feat(tunnelcat): stage 5 — daemon auto-reconnect

  Adds long-running mode that re-registers with the coord
  server on network change.

  networking-layer: transport/UDP (magicsock + DERP),
  application/HTTP (coord re-register)
  ```

  This is a grep target for the future agent: `git log
  --grep='networking-layer:'` shows every commit's network
  impact at a glance.

## 5. End-of-session handoff

Append to `memory/sessions/YYYYMMDD-<slug>.md` with:

1. **What landed** — files written, commits made, with paths
   and the `networking-layer:` tags.
2. **What didn't** — anything from the plan that didn't ship,
   with reason.
3. **What's next** — the next session's stage 1, in one
   sentence.
4. **Networking notes** — anything the next session needs to
   know about the network model (e.g. "we discovered that
   the home router uses symmetric NAT; DERP is required
   for that site").
5. **Open questions** — anything future-you should ask before
   continuing.

## 6. STATUS.md is the user's trace (always update it)

`STATUS.md` at the repo root is how the user tracks what the
agent is doing without reading commits or being in the room.
**Update it at the start and end of every work session.**

At session start, fill in the "in progress" section with what
you're about to do. At session end, after committing, move the
entry to the "what the agent did most recently" table and clear
the in-progress section.

The user has explicitly said: "the only problem is to trace
everything I have to do." STATUS.md is the answer to that
problem. Do not let it drift.

## 6. Cross-references

- `canon/PROJECTS.md` — the plan, the stages, the non-goals
- `~/.pi/agent/skills/networking-fundamentals/SKILL.md` —
  the substrate knowledge this project requires
- `~/.pi/agent/skills/atomic-to-heavy/SKILL.md` — design
  philosophy
- `~/.pi/agent/skills/plan-with-stages/SKILL.md` — the plan
  format
- `~/.pi/agent/skills/session-replay/SKILL.md` — the 9-step
  workflow
- `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md`
  — the deep-dive
