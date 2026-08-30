// End-to-end integration test: starts a real `tunnelcat up`
// subprocess, parses its token, starts a real `tunnelcat dial`
// subprocess, sends bytes through stdin, and verifies the echo
// comes back through stdout.
//
// This is the "two devices can talk" verify for Stage 1.
// It is intentionally slow (it spawns two real processes and
// uses real DERP) so it's gated behind the TUNNELCAT_E2E
// env var. CI runs it on main only.
//
// networking-layer: application/Go (the subprocesses use the
// full data plane: WireGuard + magicsock + DERP).
package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestE2EEchoRoundTrip is the Stage 1 verify. It:
//   1. starts a real `tunnelcat up` subprocess
//   2. parses the connection token from its stdout
//   3. starts a real `tunnelcat dial <token>` subprocess
//   4. pipes "hello through tunnel\n" into the dial's stdin
//   5. reads the dial's stdout, expects the same line back
//   6. cleans up both subprocesses
//
// Gated on TUNNELCAT_E2E=1 because it requires:
//   - a built `tunnelcat` binary
//   - outbound network access to the public DERP relays
//   - ~5-15 seconds of wall time
func TestE2EEchoRoundTrip(t *testing.T) {
	if os.Getenv("TUNNELCAT_E2E") != "1" {
		t.Skip("set TUNNELCAT_E2E=1 to run end-to-end tests (requires network + 5-15s)")
	}

	bin := tunnelcatBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: start the server.
	serverCmd := exec.CommandContext(ctx, bin, "up")
	serverStdout, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("server stdout pipe: %v", err)
	}
	serverStderr, err := serverCmd.StderrPipe()
	if err != nil {
		t.Fatalf("server stderr pipe: %v", err)
	}
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("server start: %v", err)
	}
	defer func() {
		_ = serverCmd.Process.Signal(os.Interrupt)
		_ = serverCmd.Wait()
	}()

	// Step 2: read the token from the server's stdout.
	// The token is on a line that starts with "🐈 Server listening with new address: "
	var token string
	tokenCh := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(serverStdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "🐈 Server listening with new address: ") {
				tok := strings.TrimPrefix(line, "🐈 Server listening with new address: ")
				tok = strings.TrimSpace(tok)
				tokenCh <- tok
				return
			}
		}
	}()
	go func() {
		// Drain stderr to keep the server from blocking.
		_, _ = io.Copy(io.Discard, serverStderr)
	}()

	select {
	case token = <-tokenCh:
		if !strings.HasPrefix(token, "tcom") {
			t.Fatalf("token does not look right: %q", token)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for server to print a token")
	}

	t.Logf("got token: %s...", token[:min(50, len(token))])

	// Step 3: start the client.
	clientCmd := exec.CommandContext(ctx, bin, "dial", token, "--port", "12345", "--timeout", "15s")
	clientStdin, err := clientCmd.StdinPipe()
	if err != nil {
		t.Fatalf("client stdin pipe: %v", err)
	}
	clientStdout, err := clientCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("client stdout pipe: %v", err)
	}
	clientStderr, err := clientCmd.StderrPipe()
	if err != nil {
		t.Fatalf("client stderr pipe: %v", err)
	}
	if err := clientCmd.Start(); err != nil {
		t.Fatalf("client start: %v", err)
	}
	defer func() {
		_ = clientCmd.Process.Signal(os.Interrupt)
		_ = clientCmd.Wait()
	}()

	// Step 4: send a line, expect it back.
	// We use a pipe and a goroutine to write so we can detect
	// a hang separately from "the read never came back".
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(clientStdin, "hello through tunnel\n")
		writeDone <- err
	}()

	// Step 5: read until we see the echo.
	var out bytes.Buffer
	readDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&out, clientStdout)
		readDone <- err
	}()

	// Give the tunnel ~5s to bring up the WireGuard handshake
	// and echo. (Typical is 1-2s; 5s is generous.)
	echoTimer := time.NewTimer(10 * time.Second)
	defer echoTimer.Stop()

	for {
		select {
		case <-echoTimer.C:
			t.Fatalf("timed out waiting for echo; got so far: %q", out.String())
		case <-time.After(100 * time.Millisecond):
			if strings.Contains(out.String(), "hello through tunnel") {
				goto echoReceived
			}
		}
	}
echoReceived:

	// Close stdin so the client knows to stop.
	if err := <-writeDone; err != nil {
		t.Logf("stdin write error (expected after close): %v", err)
	}
	_ = clientStdin.Close()
	_ = clientCmd.Wait()
	drainStderr(clientStderr)

	// Step 6: verify the echo.
	if !strings.Contains(out.String(), "hello through tunnel") {
		t.Fatalf("expected echo, got: %q", out.String())
	}
	t.Logf("echo received, tunnel works end-to-end")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// tunnelcatBinary locates the `tunnelcat` binary for the e2e
// test. We look in this order:
//   1. TUNNELCAT_BIN env var (explicit override)
//   2. /tmp/tunnelcat (the standard build path)
//   3. ./tunnelcat (relative to the test working dir)
// Fails the test if none of these exist.
func tunnelcatBinary(t *testing.T) string {
	t.Helper()
	candidates := []string{
		os.Getenv("TUNNELCAT_BIN"),
		"/tmp/tunnelcat",
		"./tunnelcat",
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	t.Fatal("no tunnelcat binary found; set TUNNELCAT_BIN or `go build -o /tmp/tunnelcat ./cmd/tunnelcat` first")
	return ""
}

// drainStderr reads from r in a goroutine and discards the
// bytes. Used to keep a subprocess's stderr pipe from filling up
// and blocking the process.
func drainStderr(r io.Reader) {
	go func() {
		_, _ = io.Copy(io.Discard, r)
	}()
}

// Sanity check: the test file should at least compile and the
// helpers should be reachable. This is a guard against typos in
// helper names.
func TestE2EHelpersCompile(t *testing.T) {
	_ = tunnelcatBinary
	_ = drainStderr
	_ = min
	_ = fmt.Sprint("ok")
}
