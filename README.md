# tunnelcat

> A 1:1 tunnel that uses the Tailscale data plane but doesn't need
> a Tailscale account. Two machines, one wire, no accounts, no
> coordination server.

```
machine A (server)         machine B (client)
$ tunnelcat up             $ tunnelcat dial <token-or-name>
🐈 Server listening...     🐈 Connected.
press Ctrl-C to stop       hello
                           hello    ← echo
```

## What is tunnelcat?

tunnelcat is a small Go binary that lets two machines form a
peer-to-peer tunnel over the Tailscale data plane (WireGuard
+ DERP relay), but without an account, a coordination server,
or any daemon. You run `tunnelcat up` on one machine, it
prints a token; you run `tunnelcat dial <token>` on the other
machine, and the two are now connected. The connection is a
TCP stream that the client and server can use however they
like: pipe a file, run an interactive shell, forward a port.

The friend test: if a friend you've never met can run a
one-liner on their laptop and connect to yours, this works.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/0ArchLinux0/tunnelcat/main/install.sh | sh
```

Or download a release tarball for your platform from
[github.com/0ArchLinux0/tunnelcat/releases](https://github.com/0ArchLinux0/tunnelcat/releases),
extract it, and add `tunnelcat` to your `$PATH`.

To build from source (requires Go 1.27+):

```sh
git clone https://github.com/0ArchLinux0/tunnelcat
cd tunnelcat
go install ./cmd/tunnelcat
```

## First connection

**On machine A (the server):**

```sh
$ tunnelcat identity init --name=studio-mac
✓ created identity "studio-mac" with pubkey nodekey:abc...
$ tunnelcat up --identity=studio-mac
🐈 Server listening with new address: tc...long-string...
press Ctrl-C to stop
```

Copy the `tc...` string.

**On machine B (the client):**

```sh
$ tunnelcat contact add studio-mac nodekey:abc...        # the pubkey from A
$ tunnelcat contact set-blob studio-mac tc...long-string...   # the token from A
$ tunnelcat dial studio-mac --port 12345
```

Anything you type now goes to the server's port 12345 and
comes back (the default echo handler). Press Ctrl-C to
disconnect.

## Manage identities and contacts

tunnelcat has two on-disk databases under `~/.config/tunnelcat/`:

- `keys/<name>.private.json` — your identities (one per device)
- `contacts.yaml` — the people you trust

```sh
# Identities (one per device you control)
tunnelcat identity init --name=laptop
tunnelcat identity show --name=laptop
tunnelcat identity show --qr       # print a QR code for the friend

# Contacts (the people you connect to)
tunnelcat contact add alice nodekey:abc...
tunnelcat contact list
tunnelcat contact show alice
tunnelcat contact remove alice

# Security: --allow on the server restricts who can connect
tunnelcat up --identity=studio-mac --allow=alice --allow=bob
```

## Troubleshooting

If something doesn't work, run `tunnelcat doctor`. It checks
five things and prints a one-line suggestion for each problem.

**Common failures:**

| Symptom | Likely cause | Fix |
|---|---|---|
| `cannot reach derp.tailscale.com` | Egress to port 443 is blocked | Try a different network, or use `--derp=<host>` |
| `contact not found` | The contact list is empty | `tunnelcat contact add <name> <pubkey>` |
| `connection rejected` | The server has `--allow` and you're not in it | `tunnelcat contact add <your-name> <your-pubkey>` on the server |
| `no ConnBlob set` | The contact has the pubkey but not the token | `tunnelcat contact set-blob <name> <token>` |
| Slow first connect | DERP relay has to do a fallback dance | Wait 5 seconds and try again |

For deeper debugging, run with `--log-level=info` to see the
data plane details.

## License

BSD 3-clause. See [LICENSE](LICENSE).

## Substrate note

tunnelcat is built *of* the network — a VPN is shaped directly
from TCP/UDP/NAT/WireGuard primitives. The agent working on
this project loads `~/.pi/agent/skills/networking-fundamentals/SKILL.md`
before any change to the data plane.
