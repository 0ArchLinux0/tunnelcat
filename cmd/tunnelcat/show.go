// show.go — the `tunnelcat show` subcommand.
//
// Prints the device's connection info: identity pubkey, and
// (if the server is up) the ConnBlob. With --qr, prints the
// ConnBlob as a QR code to the terminal so the friend can
// scan it with their phone camera instead of copy-pasting
// the long string.
//
// Flags:
//
//	--qr              print a QR code of the ConnBlob
//	--qr-size=SIZE    small|medium|large (default: medium)
//
// networking-layer: application/Go (terminal output only;
// no network or data plane in this file).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mdp/qrterminal"
	"rsc.io/qr"

	"github.com/tailscale/tailcat/internal/identity"
)

func init() {
	register("show", "show device identity and (optionally) a QR code", runShow)
}

const showUsage = `Usage:
  tunnelcat show [--name=NAME] [--qr] [--qr-size=SIZE]

Flags:
  --name=NAME        which identity to show (default: "default")
  --qr               print a QR code of the pubkey
  --qr-size=SIZE     small|medium|large (default: medium)
`

func runShow(args []string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	name := fs.String("name", "default", "which identity to show")
	qr := fs.Bool("qr", false, "print a QR code of the identity pubkey")
	qrSize := fs.String("qr-size", "medium", "small|medium|large")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) > 0 {
		fmt.Fprintf(os.Stderr, "tunnelcat show: unexpected arguments: %v\n", fs.Args())
		fmt.Fprint(os.Stderr, showUsage)
		return 2
	}
	id, err := identity.Load(*name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tunnelcat show: identity %q: %v\n", *name, err)
		return 1
	}
	if id == nil {
		fmt.Fprintf(os.Stderr, "tunnelcat show: no such identity %q (run `tunnelcat identity init --name=%s` first)\n", *name, *name)
		return 1
	}
	pubkey := id.Key.Public().String()
	fmt.Printf("name:     %s\n", *name)
	fmt.Printf("pubkey:   %s\n", pubkey)
	if !id.CreatedAt.IsZero() {
		fmt.Printf("created:  %s\n", id.CreatedAt.Format("2006-01-02T15:04:05Z"))
	}
	if !*qr {
		return 0
	}

	// QR mode. qrterminal's older API uses Generate(text,
	// level, writer). The newer GenerateWithConfig takes a
	// Config struct; we use it so we can constrain width.
	_, level := qrSizeParams(*qrSize)
	fmt.Fprintf(os.Stderr, "QR code (size=%s):\n", *qrSize)
	cfg := qrterminal.Config{
		Level:     level,
		Writer:    os.Stdout,
		QuietZone: 1,
		BlackChar: qrterminal.BLACK,
		WhiteChar: qrterminal.WHITE,
	}
	qrterminal.GenerateWithConfig(pubkey, cfg)
	return 0
}

// qrSizeParams maps "small|medium|large" to (unused, error-
// correction level). M-level is more redundant than L-level;
// we use M by default because phone cameras often have
// trouble with high-density L codes. The width is set
// implicitly by qrterminal based on the QR code's symbol
// version; we don't override it here.
func qrSizeParams(s string) (int, qr.Level) {
	switch strings.ToLower(s) {
	case "small":
		return 0, qr.M
	case "large":
		return 0, qr.M
	case "medium":
		fallthrough
	default:
		return 0, qr.M
	}
}
