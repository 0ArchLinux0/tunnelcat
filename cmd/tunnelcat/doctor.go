// doctor.go — the `tunnelcat doctor` subcommand.
//
// Runs 5 diagnostic checks and prints a one-line report for each.
// Each line is prefixed with "✓" (passed) or "✗" (failed) and
// a one-line fix suggestion for failures.
//
// Checks:
//  1. Default identity file exists and is valid
//  2. Contacts file is parseable (missing is OK)
//  3. Reachability to derp.tailscale.com on port 443 (TCP, 5s)
//  4. UDP outbound not blocked (best-effort: try a known port)
//  5. At least one identity is present (different from #1)
//
// networking-layer: application/Go (diagnostic; touches the
// network only briefly for the DERP TCP check).
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/tailscale/tailcat/internal/contacts"
	"github.com/tailscale/tailcat/internal/identity"
)

func init() {
	register("doctor", "run diagnostic checks and print a report", runDoctor)
}

const doctorUsage = `Usage:
  tunnelcat doctor [--derp=HOST]

Flags:
  --derp=HOST   DERP host to check (default: derp.tailscale.com)
`

func runDoctor(args []string) int {
	derp := "derp.tailscale.com"
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Print(doctorUsage)
			return 0
		}
		if v, ok := stripPrefix(a, "--derp="); ok {
			derp = v
		}
	}
	fmt.Println("tunnelcat doctor — diagnostic report")
	fmt.Println()

	// 1. Default identity
	check("default identity present and valid", func() string {
		id, err := identity.Load("default")
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		if id == nil {
			return "no identity \"default\" — run: tunnelcat identity init"
		}
		return ""
	})

	// 2. Contacts file parseable
	check("contacts file parseable", func() string {
		_, err := contacts.List()
		if err != nil {
			return fmt.Sprintf("error: %v (try: rm %s)", err, mustPath())
		}
		return ""
	})

	// 3. DERP TCP reachability
	check("can reach DERP relay (TCP 443, 5s)", func() string {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", derp+":443")
		if err != nil {
			return fmt.Sprintf("cannot reach %s:443: %v (check firewall or try a different network)", derp, err)
		}
		conn.Close()
		return ""
	})

	// 4. UDP outbound (best-effort)
	check("UDP outbound (best-effort)", func() string {
		c, err := net.Dial("udp", "1.1.1.1:53")
		if err != nil {
			return fmt.Sprintf("UDP outbound blocked: %v (try: tunnelcat up --derp=alternate)", err)
		}
		c.Close()
		return ""
	})

	// 5. At least one identity
	check("at least one identity exists", func() string {
		dir, err := identityConfigDirOrErr()
		if err != nil {
			return err.Error()
		}
		entries, err := os.ReadDir(dir + "/keys")
		if err != nil {
			if os.IsNotExist(err) {
				return "no keys directory — run: tunnelcat identity init"
			}
			return err.Error()
		}
		if len(entries) == 0 {
			return "no identities — run: tunnelcat identity init"
		}
		return ""
	})

	return 0
}

// check prints one line of the report.
func check(name string, fn func() string) {
	suggestion := fn()
	if suggestion == "" {
		fmt.Printf("  ✓ %s\n", name)
		return
	}
	fmt.Printf("  ✗ %s — %s\n", name, suggestion)
}

func stripPrefix(s, prefix string) (string, bool) {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return "", false
}

func mustPath() string {
	p, err := contacts.Path()
	if err != nil {
		return "your contacts file"
	}
	return p
}

func identityConfigDirOrErr() (string, error) {
	// Use the same precedence as identity: TUNNELCAT_CONFIG_DIR
	// > XDG_CONFIG_HOME > ~/.config/tunnelcat.
	if x := os.Getenv("TUNNELCAT_CONFIG_DIR"); x != "" {
		return x, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return x + "/tunnelcat", nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home dir: %w", err)
	}
	return home + "/.config/tunnelcat", nil
}
