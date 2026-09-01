// up.go — the `tunnelcat up` subcommand.
//
// Starts a tailcat server, prints a connection token, and blocks
// until SIGINT/SIGTERM. The server's OnTCP handler is a simple
// echo (in M1, this is the smallest end-to-end test; in M6 we'll
// add per-port handlers from the services YAML).
//
// networking-layer: application/Go (this is the data-plane
// call site; the actual data plane is tailscale.com).
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/tailscale/tailcat"
)

func init() {
	register("up", "listen and print a connection token", runUp)
}

// runUp starts a tailcat server, prints the connection token
// to stdout, and blocks until SIGINT/SIGTERM. The token is
// what the client pastes into `tunnelcat dial <token>`.
func runUp(args []string) int {
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
	srv.OnTCP = echoHandler

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

// echoHandler is the OnTCP handler for the server. It echoes
// everything the client sends back to them. The handler is
// called per port, so we could in principle have different
// behavior per port — M6 adds that with the services YAML.
func echoHandler(port uint16) func(net.Conn) {
	return func(c net.Conn) {
		defer c.Close()
		_, _ = io.Copy(c, c) // echo
	}
}
