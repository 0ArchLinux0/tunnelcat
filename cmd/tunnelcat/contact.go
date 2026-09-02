// contact.go — the `tunnelcat contact` subcommand.
//
// Manages the local contact list (the user's mental model of
// "the devices I trust"). Backed by the `internal/contacts`
// package.
//
// Subcommands:
//
//	tunnelcat contact add <name> <pubkey>    add a peer
//	tunnelcat contact list                    list all contacts
//	tunnelcat contact remove <name>           remove a peer
//	tunnelcat contact show <name>             show one contact
//
// networking-layer: application/Go (local file IO only;
// no network or data plane in this file).
package main

import (
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/tailscale/tailcat/internal/contacts"
)

func init() {
	register("contact", "manage the local contact list of trusted peers", runContact)
}

const contactUsage = `Usage:
  tunnelcat contact add <name> <pubkey>    add a peer to the contact list
  tunnelcat contact list                    list all contacts (one per line, name then pubkey)
  tunnelcat contact show <name>              show one contact's details
  tunnelcat contact remove <name>           remove a peer from the contact list
  tunnelcat contact set-blob <name> <token> store a peer ConnBlob so dial <name> works
`

// pubkeyRegex matches the canonical nodekey format: "nodekey:"
// followed by exactly 64 hex characters.
var pubkeyRegex = regexp.MustCompile(`^nodekey:[0-9a-f]{64}$`)

func runContact(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, contactUsage)
		return 2
	}
	switch args[0] {
	case "add":
		return runContactAdd(args[1:])
	case "list":
		return runContactList(args[1:])
	case "show":
		return runContactShow(args[1:])
	case "remove":
		return runContactRemove(args[1:])
	case "set-blob":
		return runContactSetBlob(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "tunnelcat contact: unknown verb %q\n\n", args[0])
		fmt.Fprint(os.Stderr, contactUsage)
		return 2
	}
}

func runContactAdd(args []string) int {
	fs := flag.NewFlagSet("contact add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	note := fs.String("note", "", "optional note about this contact")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) != 2 {
		fmt.Fprintln(os.Stderr, "tunnelcat contact add: expected <name> <pubkey>")
		fmt.Fprintln(os.Stderr, "Usage: tunnelcat contact add <name> <pubkey> [--note TEXT]")
		return 2
	}
	name, pubkey := rest[0], rest[1]
	if !pubkeyRegex.MatchString(pubkey) {
		fmt.Fprintf(os.Stderr, "tunnelcat contact add: invalid pubkey %q (expected nodekey:<64 hex chars>)\n", pubkey)
		return 2
	}
	c := contacts.Contact{
		Name:   name,
		Pubkey: pubkey,
		Note:   *note,
	}
	if err := contacts.Add(c); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat contact add: %v\n", err)
		return 1
	}
	fmt.Printf("✓ added contact %q\n", name)
	return 0
}

func runContactList(args []string) int {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, "tunnelcat contact list: unexpected arguments: %v\n", args)
		return 2
	}
	list, err := contacts.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat contact list: %v\n", err)
		return 1
	}
	for _, c := range list {
		fmt.Printf("%s\t%s\n", c.Name, c.Pubkey)
	}
	return 0
}

func runContactShow(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "tunnelcat contact show: expected <name>")
		return 2
	}
	c, ok := contacts.Find(args[0])
	if !ok {
		fmt.Fprintf(os.Stderr, "tunnelcat contact show: no such contact %q\n", args[0])
		return 1
	}
	fmt.Printf("name:     %s\n", c.Name)
	fmt.Printf("pubkey:   %s\n", c.Pubkey)
	if !c.AddedAt.IsZero() {
		fmt.Printf("added_at: %s\n", c.AddedAt.Format("2006-01-02T15:04:05Z"))
	}
	if c.LastSeen != nil {
		fmt.Printf("last_seen: %s\n", c.LastSeen.Format("2006-01-02T15:04:05Z"))
	}
	if c.LastAddr != "" {
		fmt.Printf("last_addr: %s\n", c.LastAddr)
	}
	if c.Note != "" {
		fmt.Printf("note:     %s\n", c.Note)
	}
	return 0
}

func runContactRemove(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "tunnelcat contact remove: expected <name>")
		return 2
	}
	if err := contacts.Remove(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat contact remove: %v\n", err)
		return 1
	}
	fmt.Printf("✓ removed contact %q\n", args[0])
	return 0
}

func runContactSetBlob(args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "tunnelcat contact set-blob: expected <name> <token>")
		return 2
	}
	name, blob := args[0], args[1]
	if !strings.HasPrefix(blob, "tc") {
		fmt.Fprintf(os.Stderr, "tunnelcat contact set-blob: token must start with \"tc\"; got %q\n", blob)
		return 2
	}
	c, ok := contacts.Find(name)
	if !ok {
		fmt.Fprintf(os.Stderr, "tunnelcat contact set-blob: no such contact %q (add it first with `tunnelcat contact add`)\n", name)
		return 1
	}
	c.ConnBlob = blob
	if err := contacts.Update(*c); err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat contact set-blob: %v\n", err)
		return 1
	}
	fmt.Printf("✓ set ConnBlob for %q\n", name)
	return 0
}
