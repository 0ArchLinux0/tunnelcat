// identity.go — the `tunnelcat identity` subcommand.
//
// Manages this device's node identity (a Curve25519 keypair).
// Backed by the `internal/identity` package.
//
// Subcommands:
//
//	tunnelcat identity init        create a new identity (fails
//	                               if one already exists for the
//	                               given name; use --force to
//	                               overwrite)
//	tunnelcat identity show        print the public key
//	tunnelcat identity list        list all identity names
//	tunnelcat identity delete      delete an identity (idempotent)
//
// Future (M2):
//
//	tunnelcat identity rotate     generate a new key, re-register
//	                               with the coord server
//
// networking-layer: application/Go (this subcommand does not
// touch the data plane; it only manipulates local files).
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/tailscale/tailcat/internal/identity"
)

func init() {
	register("identity", "manage this device's identity (keypair, name)", runIdentity)
}

const identityUsage = `Usage:
  tunnelcat identity init [--name N]   create a new identity; default name is "default"
  tunnelcat identity show [--name N]   print this device's public key
  tunnelcat identity list             list all identity names on this device
  tunnelcat identity delete [--name N] delete this device's identity file
`

// runIdentity dispatches the `tunnelcat identity <verb>` subcommands.
func runIdentity(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, identityUsage)
		return 2
	}
	switch args[0] {
	case "init":
		return runIdentityInit(args[1:])
	case "show":
		return runIdentityShow(args[1:])
	case "list":
		return runIdentityList(args[1:])
	case "delete":
		return runIdentityDelete(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "tunnelcat identity: unknown verb %q\n\n", args[0])
		fmt.Fprint(os.Stderr, identityUsage)
		return 2
	}
}

func runIdentityInit(args []string) int {
	fs := flag.NewFlagSet("identity init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "default", "identity name (alphanumeric, dash, underscore)")
	force := fs.Bool("force", false, "overwrite an existing identity with the same name")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !identityValidName(*name) {
		fmt.Fprintf(os.Stderr, "tunnelcat identity init: invalid name %q\n", *name)
		return 2
	}

	// If an identity with this name already exists, fail
	// unless --force is set.
	existing, err := identity.Load(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity init: %v\n", err)
		return 1
	}
	if existing != nil && !*force {
		fmt.Fprintf(os.Stderr,
			"tunnelcat identity init: identity %q already exists; use --force to overwrite\n",
			*name)
		return 1
	}

	id, err := identity.New(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity init: %v\n", err)
		return 1
	}
	if err := identity.Save(id); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity init: %v\n", err)
		return 1
	}
	fmt.Printf("✓ created identity %q\n", id.Name)
	fmt.Printf("  public key: %s\n", identity.PublicKeyString(id))
	fmt.Printf("  private key: stored at <config dir>/keys/%s.private.json (mode 0600)\n", id.Name)
	return 0
}

func runIdentityShow(args []string) int {
	fs := flag.NewFlagSet("identity show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "default", "identity name to show")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	id, err := identity.Load(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity show: %v\n", err)
		return 1
	}
	if id == nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity show: no identity named %q\n", *name)
		fmt.Fprintln(os.Stderr, "Run `tunnelcat identity init` to create one.")
		return 1
	}
	fmt.Println(identity.PublicKeyString(id))
	return 0
}

func runIdentityList(args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "tunnelcat identity list: unexpected arguments: %v\n", args)
		return 2
	}
	names, err := identity.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity list: %v\n", err)
		return 1
	}
	for _, n := range names {
		fmt.Println(n)
	}
	return 0
}

func runIdentityDelete(args []string) int {
	fs := flag.NewFlagSet("identity delete", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "default", "identity name to delete")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := identity.Delete(*name); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat identity delete: %v\n", err)
		return 1
	}
	fmt.Printf("✓ deleted identity %q\n", *name)
	return 0
}

// identityValidName wraps identity.Path's name validation. We
// can't import the internal `validName` because it's not
// exported; we re-validate here as a defense-in-depth check.
func identityValidName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
