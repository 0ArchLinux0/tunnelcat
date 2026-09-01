// Tests for the tunnelcat CLI. The run() function returns an
// int instead of calling os.Exit, which makes it testable
// without forking the test process.
//
// networking-layer: n/a (testing the CLI dispatch, not the
// data plane).
package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// withStdin runs fn with os.Stdin replaced by a buffer of the
// given bytes. Returns the buffer so the test can verify what
// was read.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdin = r
	defer func() { os.Stdin = old }()

	go func() {
		defer w.Close()
		_, _ = io.WriteString(w, input)
	}()

	fn()
}

// captureStdout runs fn with os.Stdout replaced by a buffer.
// Returns what was written.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	return buf.String()
}

// captureStderr runs fn with os.Stderr replaced by a buffer.
// Returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close()
	<-done
	return buf.String()
}

// TestRunNoArgs: no arguments → usage to stderr, exit 2.
func TestRunNoArgs(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr should contain usage, got: %q", stderr)
	}
}

// TestRunVersion: --version → version string to stdout, exit 0.
// This is the smoke test for the FFI bridge.
func TestRunVersion(t *testing.T) {
	stdout := captureStdout(t, func() {
		stderr := captureStderr(t, func() {
			code := run([]string{"tunnelcat", "--version"})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
		})
		_ = stderr
	})
	if !strings.HasPrefix(stdout, "tunnelcat ") {
		t.Errorf("stdout should start with 'tunnelcat ', got: %q", stdout)
	}
	if !strings.Contains(stdout, "tunnelcat-proto") {
		t.Errorf("stdout should mention the Rust crate, got: %q", stdout)
	}
}

// TestRunHelp: --help → usage to stdout, exit 0.
func TestRunHelp(t *testing.T) {
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "--help"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "Usage:") {
		t.Errorf("stdout should contain usage, got: %q", stdout)
	}
	if !strings.Contains(stdout, "tunnelcat up") {
		t.Errorf("stdout should mention 'tunnelcat up', got: %q", stdout)
	}
}

// TestRunUnknownSubcommand: subcommands that aren't registered
// → exit 2 with an "unknown subcommand" message.
func TestRunUnknownSubcommand(t *testing.T) {
	for _, sub := range []string{"identity", "status", "ssh", "resolve", "ping", "frobnicate"} {
		t.Run(sub, func(t *testing.T) {
			stderr := captureStderr(t, func() {
				code := run([]string{"tunnelcat", sub})
				if code != 2 {
					t.Errorf("exit code = %d, want 2", code)
				}
			})
			if !strings.Contains(stderr, "unknown subcommand") {
				t.Errorf("stderr should say 'unknown subcommand', got: %q", stderr)
			}
		})
	}
}

// TestRunDialMissingToken: tunnelcat dial with no token → exit 2.
func TestRunDialMissingToken(t *testing.T) {
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "dial"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "missing <token>") {
		t.Errorf("stderr should mention missing token, got: %q", stderr)
	}
}

// TestUsageString: the usage string has the required sections.
// This is a regression test: if someone accidentally deletes a
// line from the usage, the test fails.
func TestUsageString(t *testing.T) {
	required := []string{
		"tunnelcat up",
		"tunnelcat dial",
		"--version",
		"--help",
		"Subcommands:",
	}
	for _, want := range required {
		if !strings.Contains(usage, want) {
			t.Errorf("usage should contain %q", want)
		}
	}
	// Verify the registry contains the expected subcommands.
	if _, ok := subcommands["up"]; !ok {
		t.Error("subcommand 'up' not registered")
	}
	if _, ok := subcommands["dial"]; !ok {
		t.Error("subcommand 'dial' not registered")
	}
}
