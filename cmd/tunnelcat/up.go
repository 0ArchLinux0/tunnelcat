// up.go — the `tunnelcat up` subcommand.
//
// Starts a tailcat server, prints a connection token, and blocks
// until SIGINT/SIGTERM. The server's OnTCP handler is a simple
// echo (in M1, this is the smallest end-to-end test; in M6 we'll
// add per-port handlers from the services YAML).
//
// Flags:
//   --identity=<name>   use a stored identity (instead of ephemeral key)
//   --allow=<name>      allow this contact (repeatable). If no
//                       --allow is given, the server admits all
//                       client connections (open mode). With at
//                       least one --allow, only the listed
//                       contacts' pubkeys are admitted.
//
// networking-layer: application/Go (this is the data-plane
// call site; the actual data plane is tailscale.com).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/tailscale/tailcat"
	"github.com/tailscale/tailcat/internal/contacts"
	"github.com/tailscale/tailcat/internal/identity"
	"tailscale.com/types/key"
)

func init() {
	register("up", "listen and print a connection token", runUp)
}

const upUsage = `Usage:
  tunnelcat up [--identity=NAME] [--allow=NAME]... [--log-level=LEVEL]

Flags:
  --identity=NAME   use a stored identity (default: ephemeral key)
  --allow=NAME      allow a contact (repeatable). Default: no allowlist.
  --log-level=LEVEL info|warn|error|silent (default: warn)
`

func runUp(args []string) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	identityName := fs.String("identity", "", "use stored identity NAME (default: ephemeral)")
	logLevel := fs.String("log-level", "warn", "data-plane log level: info|warn|error|silent")
	var allowFlags multiFlag
	fs.Var(&allowFlags, "allow", "allow this contact (repeatable)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "tunnelcat up: unexpected arguments: %v\n", fs.Args())
		fmt.Fprint(os.Stderr, upUsage)
		return 2
	}

	// Set up a signal handler so Ctrl-C cleanly closes the server.
	ctx, cancel := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer cancel()

	srv := &tailcat.Server{
		Logf: LogLevelFromString(*logLevel),
		// ServedTCPPorts is nil → admit all ports through the filter.
	}

	// If --identity is set, load the identity from disk and use
	// its private key. Otherwise, leave Key nil for an ephemeral
	// key (the M0 default).
	if *identityName != "" {
		id, err := identity.Load(*identityName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tunnelcat up: identity %q: %v\n", *identityName, err)
			return 1
		}
		srv.Key = id.Key
	}

	// If --allow is set, populate the AllowedClients from the
	// contact list. An empty allowlist means "admit all" (open
	// mode). A non-empty allowlist means "admit only these
	// pubkeys." This is the M1.9 first security feature.
	if len(allowFlags) > 0 {
		allowed, err := resolveAllowList(allowFlags)
		if err != nil {
			fmt.Fprintf(os.Stderr, "tunnelcat up: %v\n", err)
			return 1
		}
		srv.AllowedClients = allowed
		fmt.Fprintf(os.Stderr, "tunnelcat: allowlist active: %d contact(s) allowed\n", len(allowed))
	}

	// Set up a simple echo handler on any port.
	srv.OnTCP = echoHandler

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat up: %v\n", err)
		return 1
	}
	defer srv.Close()

	fmt.Printf("🐈 Server listening with new address: %s\n", srv.ConnBlob())
	fmt.Fprintln(os.Stderr, "press Ctrl-C to stop")

	<-ctx.Done()
	fmt.Fprintln(os.Stderr, "tunnelcat: shutting down")
	return 0
}

// multiFlag is a flag.Value that accumulates repeated --allow
// invocations into a slice.
type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// resolveAllowList converts a list of contact names into the
// corresponding key.NodePublic slice. Fails if any name is not
// a known contact, or if the contact's pubkey is malformed.
func resolveAllowList(names []string) ([]key.NodePublic, error) {
	out := make([]key.NodePublic, 0, len(names))
	for _, name := range names {
		c, ok := contacts.Find(name)
		if !ok {
			return nil, fmt.Errorf("no such contact %q (add it with `tunnelcat contact add %q <pubkey>`)", name, name)
		}
		var k key.NodePublic
		if err := k.UnmarshalText([]byte(c.Pubkey)); err != nil {
			return nil, fmt.Errorf("contact %q has malformed pubkey %q: %w", name, c.Pubkey, err)
		}
		out = append(out, k)
	}
	return out, nil
}

// echoHandler is the OnTCP handler for the server. It echoes
// everything the client sends back to them.
func echoHandler(port uint16) func(net.Conn) {
	return func(c net.Conn) {
		defer c.Close()
		_, _ = io.Copy(c, c) // echo
	}
}
