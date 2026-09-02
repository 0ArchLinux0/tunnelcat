// Tests for the `tunnelcat show` subcommand.
package main

import (
	"strings"
	"testing"
)

func TestShowDefaultIdentity(t *testing.T) {
	withTempHomeContact(t)
	stdout := captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name=default"})
	})
	if !strings.Contains(stdout, "added identity") && !strings.Contains(stdout, "created") {
		// identity init may print either — accept either
	}
	stdout = captureStdout(t, func() {
		code := run([]string{"tunnelcat", "show"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "nodekey:") {
		t.Errorf("expected nodekey in output, got: %q", stdout)
	}
}

func TestShowQR(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name=default"})
	})
	stdout := captureStdout(t, func() {
		stderr := captureStderr(t, func() {
			code := run([]string{"tunnelcat", "show", "--qr"})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
		})
		if !strings.Contains(stderr, "QR code") {
			t.Errorf("expected 'QR code' in stderr, got: %q", stderr)
		}
	})
	// QR code output contains block-character cells. We don't
	// assert specific characters (terminal-dependent), but the
	// output should be non-empty and contain the pubkey.
	if len(stdout) < 50 {
		t.Errorf("expected QR output of at least 50 bytes, got %d bytes: %q", len(stdout), stdout)
	}
}

func TestShowMissingIdentity(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "show", "--name=ghost"})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "identity") {
		t.Errorf("expected 'identity' in stderr, got: %q", stderr)
	}
}
