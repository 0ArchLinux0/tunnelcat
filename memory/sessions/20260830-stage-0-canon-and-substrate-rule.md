# Session 2026-08-30 — stage 0 (canon + substrate rule)

## What landed

| Path | What |
|---|---|
| `canon/PROJECTS.md` | Full stage plan, atomic-to-heavy contract, per-stage network touch matrix, open questions for the user. 13.2KB. |
| `AGENTS.md` | Local rule enforcing the networking-fundamentals skill. 5.1KB. The 6 sections: cold-start recipe, networking-knowledge rule, atomic-to-heavy, session discipline, handoff, cross-refs. |
| `~/code_repo/tailcat/` (commit `fabe47f`) | Both files, on local `main`. |
| `~/.pi/agent/skills/networking-fundamentals/SKILL.md` | New global skill — 12 things every agent must internalize + diagnostic command set + reading list + pre-flight checklist. 14.2KB. |
| `~/.pi/agent/AGENTS.md` | New global rule: "Domain knowledge for the substrate" — for any project built OF its substrate (network, parser, database, crypto, concurrency), the agent must load the substrate skill and produce a "Why this stage touches the substrate" note before each new stage. |
| `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md` | The deep-dive the user asked for (16 sections, 22KB), already pushed to Notion (P1) and Telegram. |
| `go 1.27.0` | Installed via Homebrew. `go build ./...` and `go vet ./...` both clean against upstream tailcat. |

## What didn't

- **Branch push failed (403).** The local `origin` still points to the upstream `https://github.com/tailscale/tailcat.git`, which is read-only for us. The local commit is fine; the next session should either (a) ask the user to fork the repo and re-point `origin`, or (b) accept local-only commits for now. The branch-push script behaved correctly — it refused to push to a remote that doesn't accept our writes.
- **No binary name picked yet.** Open question #1 in the canon.
- **Path A vs B vs hybrid not yet decided.** Open question #3 in the canon. The next session's stage 1 doesn't actually need this answer (it works in all three paths), but the user should be asked.

## What's next

**Next session, stage 1, in one sentence:** Build `cmd/mesh/main.go` — a ~150-line Go binary that wraps `tailcat.Server` and `tailcat.Client` with a single `mesh up` / `mesh dial <token>` subcommand pair, branded with our own name, on its own session branch.

## Networking notes for the next session

- **The whole project is built of the network.** Every new stage starts with a 2–4 sentence "Why this stage touches the network" note per `AGENTS.md` §2.1.
- **Stage 1 is mostly application-layer work** (CLI + JSON I/O). It does NOT touch magicsock, netstack, or the wire format. The agent can treat tailcat as a black box and wrap it.
- **The pre-flight checklist** in the skill matters more starting at stage 3 (coord server, real protocol work). Stage 1 can skip it.
- **Diagnostic reflex to develop early:** when stage 3+ introduces network code, the agent should default to `tcpdump` (or `go test` with verbose logging) before adding more code. The skill has the command set.

## Open questions for the user (carried from canon)

1. **Binary name.** `mesh`? `tailmesh`? `meow`? `kitten`? Decide at stage 1.
2. **Repo identity.** Fork under your GitHub, new repo, or local-only? Affects the LICENSE and copyright line at stage 1.
3. **How many devices, really?** If 2, we can skip the coord server entirely. If 20+, consider headscale instead of building it ourselves. If 5–10, the plan as written is right.
4. **All-on-LAN or cross-NAT?** If all on LAN, we can skip the DERP relay stage.

## External links

- Notion: "How Tailcat / Tailscale Works Under the Hood — full deep-dive" (P1, page id `3cc7b981-c514-819f-a38e-c70c6f8bd8bd`)
- Telegram: already pushed (URL + `---` + summary format, per AGENTS.md hard rule)
- The deep-dive body file: `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md`
