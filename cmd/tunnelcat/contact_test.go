// Tests for the `tunnelcat contact` subcommand.
package main

import (
	"strings"
	"testing"
)

func withTempHomeContact(t *testing.T) {
	t.Helper()
	t.Setenv("TUNNELCAT_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestContactNoVerb(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr should contain usage, got: %q", stderr)
	}
}

func TestContactAdd(t *testing.T) {
	withTempHomeContact(t)
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "contact", "add", "alice", "nodekey:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "added contact") {
		t.Errorf("stdout should say 'added contact', got: %q", stdout)
	}
}

func TestContactAddInvalidPubkey(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact", "add", "alice", "not-a-pubkey"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "invalid pubkey") {
		t.Errorf("stderr should say 'invalid pubkey', got: %q", stderr)
	}
}

func TestContactAddMissingArgs(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact", "add", "alice"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "expected") {
		t.Errorf("stderr should say 'expected', got: %q", stderr)
	}
}

func TestContactAddDuplicate(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
	})
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr should say 'already exists', got: %q", stderr)
	}
}

func TestContactList(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	})
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "bob",
			"nodekey:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "contact", "list"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "alice") {
		t.Errorf("stdout should contain alice, got: %q", stdout)
	}
	if !strings.Contains(stdout, "bob") {
		t.Errorf("stdout should contain bob, got: %q", stdout)
	}
}

func TestContactShow(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "contact", "show", "alice"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "nodekey:aaaa") {
		t.Errorf("stdout should contain pubkey, got: %q", stdout)
	}
}

func TestContactShowMissing(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact", "show", "ghost"})
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "no such contact") {
		t.Errorf("stderr should say 'no such contact', got: %q", stderr)
	}
}

func TestContactRemove(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "contact", "remove", "alice"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "removed") {
		t.Errorf("stdout should say 'removed', got: %q", stdout)
	}
}

func TestContactUnknownVerb(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "contact", "frobnicate"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "unknown verb") {
		t.Errorf("stderr should say 'unknown verb', got: %q", stderr)
	}
}
