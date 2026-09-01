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
// Subcommands live in their own files (up.go, dial.go) and
// register themselves with the package-level `subcommands` map
// in their init() function. The dispatch in `run()` is a
// simple map lookup. To add a new subcommand in M1.4+, write
// a file like `identity.go` and add an init() that calls
// `register("identity", runIdentity, "<summary>")`. No edit
// to main.go required.
//
// networking-layer: application/Go (wraps tailscale.com via
// the tailcat Go library; no data-plane code in this file).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tailscale/tailcat/internal/rustbridge"
)

// version is set at build time via -ldflags "-X main.version=...".
// bin/release.sh and install.sh do this. Default is "dev" for
// unbranded builds.
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

Subcommands:
`

// subcommand is a registered subcommand. The `summary` is shown
// in the usage block (one line per subcommand).
type subcommand struct {
	summary string
	run     func(args []string) int
}

// subcommands is the registry. Each subcommand file's init()
// function adds an entry here. Order in the usage block is
// the order in this map (Go map iteration is random, so we
// use a slice of names + map for the order-preserved list).
var (
	subcommands    = map[string]subcommand{}
	subcommandKeys []string
)

func register(name, summary string, run func(args []string) int) {
	if _, exists := subcommands[name]; exists {
		panic("subcommand " + name + " already registered")
	}
	subcommands[name] = subcommand{summary: summary, run: run}
	subcommandKeys = append(subcommandKeys, name)
}

func main() {
	os.Exit(run(os.Args))
}

// run is the actual entry point. It returns the process exit
// code rather than calling os.Exit, so tests can call it and
// check the return value.
func run(args []string) int {
	if len(args) < 1 {
		fmt.Fprint(os.Stderr, usageWithSubcommands())
		return 2
	}

	// Top-level flags: --version and --help. Both must work
	// even when the user types nothing else (`tunnelcat --help`).
	top := flag.NewFlagSet("tunnelcat", flag.ContinueOnError)
	top.SetOutput(os.Stderr)
	versionFlag := top.Bool("version", false, "print version and exit")
	helpFlag := top.Bool("help", false, "print help and exit")
	if err := top.Parse(args[1:]); err != nil {
		return 2
	}

	if *versionFlag {
		return printVersion()
	}
	if *helpFlag {
		fmt.Print(usageWithSubcommands())
		return 0
	}

	rest := top.Args()
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usageWithSubcommands())
		return 2
	}

	// Subcommand dispatch via the registry.
	cmd, ok := subcommands[rest[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "tunnelcat: unknown subcommand %q\n\n", rest[0])
		fmt.Fprint(os.Stderr, usageWithSubcommands())
		return 2
	}
	return cmd.run(rest[1:])
}

// printVersion prints the Go version (build-time) and the Rust
// crate version (queried at runtime). The FFI check is a useful
// smoke test: if Version() fails, the bridge is broken and the
// user sees a clear error.
func printVersion() int {
	rv, err := rustbridge.Version()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat: %v\n", err)
		return 1
	}
	fmt.Printf("tunnelcat %s (%s)\n", version, rv)
	return 0
}

// usageWithSubcommands returns the full usage string, with the
// subcommand list appended. The subcommands are sorted
// alphabetically for a stable display.
func usageWithSubcommands() string {
	var b strings.Builder
	b.WriteString(usage)
	// Pad subcommand names to a fixed width for aligned output.
	maxLen := 0
	for _, name := range subcommandKeys {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}
	for _, name := range subcommandKeys {
		b.WriteString("  tunnelcat ")
		b.WriteString(name)
		for i := len(name); i < maxLen; i++ {
			b.WriteByte(' ')
		}
		b.WriteString("    ")
		b.WriteString(subcommands[name].summary)
		b.WriteByte('\n')
	}
	b.WriteString("\nSee canon/PROJECTS.md and canon/TAILCAT-API.md.\n")
	return b.String()
}
