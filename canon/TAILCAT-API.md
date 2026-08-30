# tailcat API cheat sheet

> **What this is.** A one-page summary of the public Go API of
> `github.com/tailscale/tailcat`. It is **not** a substitute for
> the source — the source is the truth and you should read it
> when something is unclear — but it tells you the 80% you need
> to know to start writing `cmd/tunnelcat/` without having to
> read 1800 lines of `tailcat.go`.
>
> **Source files referenced:**
> - `tailcat.go` (1800 lines, the library)
> - `disco.go` (67 lines, the Meow protocol)
> - `wire.go` (111 lines, the ConnBlob wire format)
>
> **Last verified against:** `tailscale/tailcat` commit `7c2a6ea`
> (the HEAD of our clone), 2026-08-30.

---

## The minimum viable mental model

Tailcat has **two public types** that matter for Stage 1: `Server`
and `Client`. That's it. Everything else in the 1800-line file is
either internal to the data plane (which we don't touch) or
options on those two types.

```
  ┌──────────────────────────────┐
  │ tailcat.Server               │  listens for connections
  │   - Start()                  │  connects to DERP
  │   - ConnBlob() string        │  prints a token you give to peers
  │   - Close()                  │
  └──────────────────────────────┘
              │
              │  the token
              ▼
  ┌──────────────────────────────┐
  │ tailcat.Client               │  dials the server
  │   - DialTCPPort(ctx, port)   │  opens a TCP conn through the tunnel
  │   - Ping(ctx)                │  tests the tunnel
  │   - Close()                  │
  └──────────────────────────────┘
```

That's the whole Stage 1 surface. Everything else in this doc
is for later stages.

---

## `tailcat.Server` — the listening side

### Construction

```go
s := &tailcat.Server{
    // All fields are optional. Zero value works.
    // Key:        key.NodePrivate // nil = generate ephemeral
    // Logf:       logger.Logf     // nil = log.Printf
    // Region:     *tailcfg.DERPRegion  // nil = pick nearest
    // RegionID:   int             // 0 = pick nearest
    // DERPMapURL: string          // "" = tailcat.dev
    // DERPMapCache: DERPMapCache  // nil = in-memory
}
```

### Lifecycle

```go
err := s.Start()      // connects to DERP, generates key, ready
defer s.Close()        // disconnects from DERP, frees key

token := s.ConnBlob()  // ~50-byte string, share with peers
// token looks like: tcomFwWCCcjS5nKNqAod034nWoJZW0LZqDhhC8U_dKdnDRYQ8uNGFpGQEu
```

### What happens in `Start()`

1. Generates an ephemeral Curve25519 keypair (unless `Key` set).
2. Connects to the configured DERP region.
3. Sits idle, waiting for a "Meow" packet from any client.
4. When a Meow arrives, adds the client as a WireGuard peer
   and sends a "Meowed" reply.
5. Both sides then run the disco protocol for NAT traversal.

### Server-side TCP dispatch

By default, `Start()` listens on **no ports**. To accept TCP
connections, set `OnTCP`:

```go
s := &tailcat.Server{
    OnTCP: func(port uint16) func(net.Conn) {
        return func(c net.Conn) {
            // c is a connection to the client that dialed `port`
            // through the tunnel.
            io.Copy(c, c)  // echo server, for example
            c.Close()
        }
    },
}
```

If `OnTCP` is nil, the server only accepts connections through
its built-in port-0 handler (the CLI's `--serve=0` mode, which
just pipes through stdin/stdout).

### Other methods (for later stages)

- `Addr() netip.Addr` — the server's IPv6 address inside the
  tunnel. Both sides derive this deterministically from the
  server's public key, so the client can dial it without
  configuration.
- `AddAllowedClient(k key.NodePublic)` — restrict the server to
  only accept connections from this client. Stage 2 uses this
  for the contact list / allowlist.
- `DrainTCP(ctx) error` — wait for all active TCP connections
  to close. Used for clean shutdown.
- `Status() *ipnstate.Status` — debug info, useful for
  `tunnelcat status`.

---

## `tailcat.Client` — the dialing side

### Construction

```go
cl := tailcat.NewClient(tailcat.ConnBlob("tcom..."))
// Or, with options:
cl := &tailcat.Client{
    Server: tailcat.ConnBlob("tcom..."),
    Key:    myKey,   // nil = ephemeral
    Logf:   logger.Discard,
}
defer cl.Close()
```

**The tunnel is established lazily.** Just constructing a
`Client` does NOT connect to anything. The first call to
`Dial`, `DialTCPPort`, or `Ping` triggers the connection.

### The dial methods (Stage 1 surface)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

// Open a TCP connection to `port` on the server side, through
// the tunnel. This is the primary dial method.
conn, err := cl.DialTCPPort(ctx, 8080)
if err != nil {
    log.Fatal(err)
}
io.Copy(os.Stdout, conn)

// Lower-level: dial a specific IP:port inside the tunnel.
// Stage 1 probably won't use this; it's for the services YAML
// in stage 6.
conn2, err := cl.DialTCP(ctx, netip.MustParseAddrPort("[100::1]:22"))

// Lowest-level: dial any (network, addr) the tunnel supports.
// "tcp", "udp", "ip" all work because tailcat has a userspace
// netstack. For stage 1, prefer DialTCPPort.
conn3, err := cl.Dial(ctx, "tcp", "server.tailcat:22")
```

### Other methods (for later stages)

- `Ping(ctx) (PingResult, error)` — tests the tunnel, reports
  whether it went via DERP or direct P2P. Stage 5 uses this
  for `tunnelcat ping <peer>`.
- `PublicKey() key.NodePublic` — this client's public key,
  used for the allowlist in stage 2.
- `Close() error` — disconnects from DERP, frees key.

---

## `tailcat.ConnBlob` — the connection token

A `ConnBlob` is just a string (typed for safety). Internally
it's a `"tc"` prefix followed by base64-encoded CBOR. The
CBOR contains:

- The server's WireGuard public key (32 bytes)
- DERP region info, either:
  - A small integer (region ID, 1–3 bytes)
  - OR full DERP server metadata (hostname, IP, etc.)

A typical token is 50–80 bytes. You never need to parse a
token yourself — `tailcat.ParseConnBlob(cb)` does it and
returns a `ConnInfo` struct.

```go
info, err := tailcat.ParseConnBlob(cb)
fmt.Println(info.ServerPublic)  // "nodekey:9c8d2e67..."
fmt.Println(info.Region)         // []*tailcfg.DERPRegion
```

---

## The Meow protocol (what Stage 2 replaces with a Rust crate)

The file `disco.go` is 67 lines. The whole protocol:

```go
// 4-byte magic: "meow" in ASCII
var meowMagic = [4]byte{'m', 'e', 'o', 'w'}

// 1-byte message type
const (
    meowTypePing = 0x01  // client → server
    meowTypePong = 0x02  // server → client (the "meowed")
)

// A "Meow ping" packet is:
//   bytes  0..3   = "meow" magic
//   byte   4      = 0x01 (ping type)
//   bytes  5..36  = sender's WireGuard node public key (32 bytes)
//   bytes 37..68  = sender's disco public key (32 bytes)
// Total: 69 bytes.

// A "Meowed" packet is just:
//   bytes 0..3 = "meow" magic
//   byte  4    = 0x02 (pong type)
// Total: 5 bytes. No payload.
```

**Correction to the deep-dive:** the Notion article says Meow is
68 bytes. It's actually 69 bytes (4 magic + 1 type + 32 + 32).
The deep-dive should be updated. (Will do in a separate commit
on a doc-fix branch.)

### Functions in `disco.go`

| Function | Purpose |
|---|---|
| `IsMeowPacket(pkt []byte) bool` | Does the packet start with the magic? |
| `EncodeMeowPing(nodeKey, discoKey) []byte` | Build a 69-byte ping |
| `EncodeMeowed() []byte` | Build a 5-byte pong |
| `ParseMeowPing(pkt) (nodeKey, discoKey, ok)` | Decode a ping |
| `IsMeowedPacket(pkt) bool` | Is this a pong? |

**In stage 2, the Rust crate re-implements all five** with the
same wire format but using type-state to make "encode a Meowed
without first having received a Meow" a compile error.

---

## The `wire.go` file (just for reference)

Defines the CBOR encoding of a `ConnBlob`. You don't need to
read this for Stage 1; you need it for Stage 2 if you want to
generate tokens yourself instead of calling `Server.ConnBlob()`.

---

## What tailcat's package gives you that netcat doesn't

| Feature | `nc` (netcat) | `tailcat` |
|---|---|---|
| Encryption | none | WireGuard (Curve25519 + ChaCha20-Poly1305) |
| NAT traversal | none (needs port forward) | automatic (UDP hole-punching + DERP fallback) |
| Auth | none | server's public key, optional allowlist |
| Library | none | `Server` and `Client` types in `tailcat` package |
| Public API stability | "it's been the same since 1995" | "may change, see README §Stability" |
| Bundle size | ~50 KB binary | ~10 MB Go binary (or smaller with `go build -ldflags="-s -w"`) |
| TUN/TAP | no | no (uses userspace netstack) |
| Needs root | no | no |
| Cross-platform | yes | yes (macOS, Linux, Windows, WASM) |

---

## Stage 1 in 30 lines (preview)

This is what `cmd/tunnelcat/main.go` will look like by the end
of Stage 1. **Don't write this yet** — write it in the next
session, with the user, as their first hands-on Go code.

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "time"

    "github.com/tailscale/tailcat"
)

func main() {
    sub := ""
    if len(os.Args) > 1 {
        sub = os.Args[1]
    }
    switch sub {
    case "up":
        s := &tailcat.Server{}
        if err := s.Start(); err != nil {
            log.Fatal(err)
        }
        defer s.Close()
        fmt.Println(s.ConnBlob())
        select {} // hang
    case "dial":
        token := os.Args[2]
        port := flag.Uint("port", 0, "TCP port on the server to dial")
        flag.CommandLine.Parse(os.Args[3:])
        cl := tailcat.NewClient(tailcat.ConnBlob(token))
        defer cl.Close()
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        c, err := cl.DialTCPPort(ctx, uint16(*port))
        if err != nil {
            log.Fatal(err)
        }
        defer c.Close()
        go io.Copy(c, os.Stdin)
        io.Copy(os.Stdout, c)
    default:
        fmt.Fprintln(os.Stderr, "usage: tunnelcat up | dial <token> [--port N]")
        os.Exit(2)
    }
}
```

That's it. ~50 lines. Everything else is decoration.

---

## Cross-references

- `~/.pi/agent/knowledge/notion-articles/01-how-tailcat-works.md` — the
  deep-dive this cheat sheet complements
- `~/.pi/agent/skills/networking-fundamentals/SKILL.md` §11 — the
  Go socket API reference
- `tailcat.go` (the source) — for anything this sheet doesn't cover
- `canon/PROJECTS.md` §"Stage 1" — the actual stage plan

_Last verified: 2026-08-30 against tailcat commit 7c2a6ea._
