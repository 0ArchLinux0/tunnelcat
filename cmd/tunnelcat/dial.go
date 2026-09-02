// dial.go — the `tunnelcat dial <token>` subcommand.
//
// Dials a server through the tunnel and pipes stdin/stdout
// to the resulting connection. The server's `OnTCP` handler
// echoes, so anything you type on the client side comes back.
//
// networking-layer: application/Go (this is the data-plane
// call site; the actual data plane is tailscale.com).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tailscale/tailcat"
	"github.com/tailscale/tailcat/internal/contacts"
	"github.com/tailscale/tailcat/internal/identity"
)

func init() {
	register("dial", "dial a server through the tunnel", runDial)
}

// runDial dials a server through the tunnel and pipes stdin/stdout
// to the resulting connection. The server's `OnTCP` handler echoes,
// so anything you type on the client side comes back.
//
// The first argument can be either:
//   - a contact name (looked up in the local contacts file),
//     in which case the stored ConnBlob for that contact is used
//   - a raw tailcat.ConnBlob token (the "tc..." string)
//
// If the arg starts with "tc" we treat it as a token; otherwise
// we look it up as a contact. The contact path requires the
// contact to have a ConnBlob set (do `tunnelcat contact
// set-blob <name> <token>` after the friend sends the token).
func runDial(args []string) int {
	fs := flag.NewFlagSet("dial", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Uint("port", 12345, "TCP port on the server side to dial (default: 12345 = echo)")
	timeout := fs.Duration("timeout", 30*time.Second, "connect timeout")
	identityName := fs.String("identity", "", "use stored identity NAME (default: ephemeral)")
	logLevel := fs.String("log-level", "warn", "data-plane log level: info|warn|error|silent")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) < 1 {
		fmt.Fprintln(os.Stderr, "tunnelcat dial: missing <token-or-name> argument")
		fmt.Fprintln(os.Stderr, "Usage: tunnelcat dial <token-or-name> [--port N] [--identity=NAME] [--log-level=LEVEL]")
		return 2
	}
	arg := rest[0]
	token := resolveDialArg(arg)
	if token == "" {
		// resolveDialArg already printed the error.
		return 1
	}

	cl := tailcat.NewClient(token)
	cl.Logf = LogLevelFromString(*logLevel)
	if *identityName != "" {
		id, err := identity.Load(*identityName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tunnelcat dial: identity %q: %v\n", *identityName, err)
			return 1
		}
		cl.Key = id.Key
	}
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

// resolveDialArg maps a dial argument to a tailcat.ConnBlob.
// If the arg starts with "tc" it's a raw token. Otherwise we
// look it up in the local contact list. Returns "" on error
// (and prints the error to stderr).
func resolveDialArg(arg string) tailcat.ConnBlob {
	if strings.HasPrefix(arg, "tc") {
		return tailcat.ConnBlob(arg)
	}
	c, ok := contacts.Find(arg)
	if !ok {
		fmt.Fprintf(os.Stderr, "tunnelcat dial: %q is not a \"tc\" token and is not a known contact (add it with `tunnelcat contact add`)\n", arg)
		return ""
	}
	if c.ConnBlob == "" {
		fmt.Fprintf(os.Stderr, "tunnelcat dial: contact %q has no ConnBlob set; have your friend send their token and run `tunnelcat contact set-blob %q <token>`\n", arg, arg)
		return ""
	}
	return tailcat.ConnBlob(c.ConnBlob)
}
