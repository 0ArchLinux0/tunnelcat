// Command tunnelcat is the tunnelcat CLI.
//
// Stage 1 surface:
//   tunnelcat up                          start a server, print token
//   tunnelcat dial <token> [--port N]     dial a server through the tunnel,
//                                         pipe stdin/stdout
//   tunnelcat --version                   print version, exit
//   tunnelcat --help                      print usage, exit
//
// The CLI is split into a thin main() that calls run(args). This
// makes the dispatch logic testable without the os.Exit machinery
// that would make `go test ./cmd/tunnelcat/` impossible.
//
// networking-layer: application/Go (wraps tailscale.com via
// the tailcat Go library; no data-plane code in this file).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tailscale/tailcat"
	"github.com/tailscale/tailcat/internal/rustbridge"
)

// version is set at build time via -ldflags "-X main.version=...".
// bin/release.sh does this. Default is "dev" for unbranded builds.
var version = "dev"

const usage = `tunnelcat — a control-plane-free mesh VPN built on tailcat's data plane

Usage:
  tunnelcat up                          listen and print a connection token
  tunnelcat dial <token> [--port N]     dial a server through the tunnel
  tunnelcat --version                   print version and exit
  tunnelcat --help                      print help and exit

Options for 'dial':
  --port N        TCP port on the server side to dial (default: 12345)
  --timeout DUR   connect timeout (default: 30s)

Subcommands planned but not yet implemented:
  tunnelcat identity ...          manage this device's identity (stage 2)
  tunnelcat status                show online peers (stage 3+)
  tunnelcat ssh <peer>            shell into a peer (stage 6)
  tunnelcat resolve <peer>        resolve a peer name to an IP (stage 6)
  tunnelcat ping <peer>           test connectivity (stage 3+)

Examples:
  # On machine A (the server):
  $ tunnelcat up
  🐈 Server listening with new address: tcomABC...

  # On machine B (the client), with machine A's token:
  $ tunnelcat dial tcomABC... --port 22
  # (then type into the SSH session that opens)

See canon/PROJECTS.md and canon/TAILCAT-API.md for the bigger picture.
`

func main() {
	// main() is a shim. The real work is in run(). This split
	// makes the dispatch logic testable in main_test.go without
	// fighting os.Exit semantics.
	os.Exit(run(os.Args))
}

// run is the actual entry point. It returns the process exit
// code rather than calling os.Exit, so tests can call it and
// check the return value.
func run(args []string) int {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	// Top-level flags: --version and --help. Both must work
	// even when the user types nothing else (`tunnelcat --help`).
	// We use a fresh FlagSet so subcommand flags don't pollute
	// the top-level parse.
	top := flag.NewFlagSet("tunnelcat", flag.ContinueOnError)
	top.SetOutput(os.Stderr)
	versionFlag := top.Bool("version", false, "print version and exit")
	helpFlag := top.Bool("help", false, "print help and exit")
	// Parse starting at args[1:] to skip the program name.
	if err := top.Parse(args[1:]); err != nil {
		// flag.ContinueOnError means Parse already printed the
		// error. We just need to exit non-zero.
		return 2
	}

	if *versionFlag {
		return printVersion()
	}
	if *helpFlag {
		fmt.Print(usage)
		return 0
	}

	rest := top.Args()
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}

	// Subcommand dispatch.
	switch rest[0] {
	case "up":
		return runUp()
	case "dial":
		return runDial(rest[1:])
	case "identity", "status", "ssh", "resolve", "ping":
		fmt.Fprintf(os.Stderr, "tunnelcat %s: not yet implemented (see canon/PROJECTS.md)\n", rest[0])
		return 1
	default:
		fmt.Fprintf(os.Stderr, "tunnelcat: unknown subcommand %q\n\n", rest[0])
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}

// printVersion prints the Go version (build-time) and the Rust
// crate version (queried at runtime). This is a useful smoke
// test for the FFI bridge: if Version() fails, the bridge is
// broken and the user sees a clear error.
func printVersion() int {
	rv, err := rustbridge.Version()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat: %v\n", err)
		return 1
	}
	fmt.Printf("tunnelcat %s (%s)\n", version, rv)
	return 0
}

// runUp starts a tailcat server, prints the connection token
// to stdout, and blocks until SIGINT/SIGTERM. The token is what
// the client pastes into `tunnelcat dial <token>`.
//
// By default the server has no TCP handler (Stage 1's smallest
// working product: pipe stdin/stdout through the tunnel to a
// single TCP connection on the server side). Stage 6's services
// YAML adds the named-port mapping.
//
// The "shell-style comments" like the cat emoji 🐈 are deliberately
// kept: they match tailcat's own output and make copy-pasted
// terminal output recognizable in chat / Notion.
func runUp() int {
	// Set up a signal handler so Ctrl-C cleanly closes the
	// server. Without this, Ctrl-C would leave the WireGuard
	// peer mapping dangling in DERP until the process actually
	// exits, which can take a few seconds.
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := &tailcat.Server{
		// Key is nil → ephemeral key. Each runUp generates a
		// fresh Curve25519 keypair. Stage 2's `identity init`
		// will save a stable key to disk and pass it here.
		// Logf is nil → log.Printf, the default.
		// ServedTCPPorts is nil → admit all ports through the
		// filter. Stage 6's services YAML will narrow this.
	}

	// Set up a simple echo handler on any port. When the client
	// dials us, we just echo everything they send back to them.
	// This is the smallest possible end-to-end test.
	srv.OnTCP = func(port uint16) func(net.Conn) {
		return func(c net.Conn) {
			defer c.Close()
			_, _ = io.Copy(c, c) // echo
		}
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat up: %v\n", err)
		return 1
	}
	defer srv.Close()

	// Print the token. tailcat's ConnBlob already includes a
	// "tc..." prefix; we just print it raw so the user can
	// pipe it to a file or copy it.
	fmt.Printf("🐈 Server listening with new address: %s\n", srv.ConnBlob())
	fmt.Fprintln(os.Stderr, "press Ctrl-C to stop")

	// Block until SIGINT or fatal error. tailcat's server
	// doesn't expose a "wait for shutdown" method, so we just
	// select on the context and on a fake error channel.
	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "tunnelcat: shutting down")
	return 0
}

// runDial dials a server through the tunnel and pipes stdin/stdout
// to the resulting connection. The server's `OnTCP` handler echoes,
// so anything you type on the client side comes back.
func runDial(args []string) int {
	fs := flag.NewFlagSet("dial", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Uint("port", 12345, "TCP port on the server side to dial (default: 12345 = echo)")
	timeout := fs.Duration("timeout", 30*time.Second, "connect timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "tunnelcat dial: missing <token> argument")
		fmt.Fprintln(os.Stderr, "Usage: tunnelcat dial <token> [--port N]")
		return 2
	}
	token := tailcat.ConnBlob(rest[0])

	cl := tailcat.NewClient(token)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Dial the requested port. port=0 is the "any/echo" port
	// that the server's OnTCP handler covers.
	conn, err := cl.DialTCPPort(ctx, uint16(*port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat dial: %v\n", err)
		return 1
	}
	defer conn.Close()

	// Pipe stdin → conn and conn → stdout. We run the two
	// directions concurrently. If one direction errors, we
	// close the conn and let the other goroutine see the
	// error and exit too.
	errCh := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		if err != nil && !errors.Is(err, io.EOF) {
			errCh <- fmt.Errorf("stdin → conn: %w", err)
		}
		// Half-close the write side so the server sees EOF.
		if tcp, ok := conn.(closeWriter); ok {
			_ = tcp.CloseWrite()
		}
		errCh <- nil
	}()
	go func() {
		_, err := io.Copy(os.Stdout, conn)
		if err != nil && !errors.Is(err, io.EOF) {
			errCh <- fmt.Errorf("conn → stdout: %w", err)
		}
		errCh <- nil
	}()

	// Wait for either direction to error or both to finish.
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

// closeWriter is implemented by *net.TCPConn (and tailcat's
// userspace netstack conns). It lets us half-close after stdin
// hits EOF, which is the polite thing to do for protocols like
// SSH that expect the client to close its write side first.
type closeWriter interface {
	CloseWrite() error
}
