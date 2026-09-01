// Tests for the `tunnelcat identity` subcommand.
//
// These run the full CLI dispatch (not just the package), so
// they exercise the user-facing behavior: exit codes, output
// to stdout/stderr, flag parsing.
package main

import (
	"strings"
	"testing"
)

// withTempHome sets TUNNELCAT_CONFIG_DIR to a fresh temp dir
// for the duration of the test. We use this for every identity
// test so we don't touch the user's real config.
func withTempHomeForIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("TUNNELCAT_CONFIG_DIR", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func TestIdentityNoVerb(t *testing.T) {
	withTempHomeForIdentity(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr should contain usage, got: %q", stderr)
	}
}

func TestIdentityInit(t *testing.T) {
	withTempHomeForIdentity(t)
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "init"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "created identity") {
		t.Errorf("stdout should contain 'created identity', got: %q", stdout)
	}
	if !strings.Contains(stdout, "public key:") {
		t.Errorf("stdout should contain 'public key:', got: %q", stdout)
	}
}

func TestIdentityInitWithName(t *testing.T) {
	withTempHomeForIdentity(t)
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "init", "--name", "studio-mac"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, `created identity "studio-mac"`) {
		t.Errorf("stdout should mention the custom name, got: %q", stdout)
	}
}

func TestIdentityInitAlreadyExists(t *testing.T) {
	withTempHomeForIdentity(t)
	// First init succeeds.
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init"})
	})
	// Second init fails.
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity", "init"})
		if code != 1 {
			t.Errorf("second init: exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr should say 'already exists', got: %q", stderr)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr should mention --force, got: %q", stderr)
	}
}

func TestIdentityInitForce(t *testing.T) {
	withTempHomeForIdentity(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "init", "--force"})
		if code != 0 {
			t.Errorf("--force init: exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "created identity") {
		t.Errorf("--force init should succeed, got: %q", stdout)
	}
}

func TestIdentityInitInvalidName(t *testing.T) {
	withTempHomeForIdentity(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity", "init", "--name", "with space"})
		if code != 2 {
			t.Errorf("exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "invalid name") {
		t.Errorf("stderr should say 'invalid name', got: %q", stderr)
	}
}

func TestIdentityShow(t *testing.T) {
	withTempHomeForIdentity(t)
	// Init first.
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init"})
	})
	// Now show should print the public key.
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "show"})
		if code != 0 {
			t.Errorf("show: exit code = %d, want 0", code)
		}
	})
	if !strings.HasPrefix(strings.TrimSpace(stdout), "nodekey:") {
		t.Errorf("show should print a nodekey, got: %q", stdout)
	}
}

func TestIdentityShowMissing(t *testing.T) {
	withTempHomeForIdentity(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity", "show"})
		if code != 1 {
			t.Errorf("show missing: exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "no identity") {
		t.Errorf("stderr should say 'no identity', got: %q", stderr)
	}
}

func TestIdentityShowAfterInit(t *testing.T) {
	withTempHomeForIdentity(t)
	// Init, then show, then verify the keys match.
	initOut := captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name", "alpha"})
	})
	initPub := extractPublicKey(initOut)
	if initPub == "" {
		t.Fatal("could not extract public key from init output")
	}

	showOut := captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "show", "--name", "alpha"})
	})
	showPub := strings.TrimSpace(showOut)
	if showPub != initPub {
		t.Errorf("init pubkey %q != show pubkey %q", initPub, showPub)
	}
}

func TestIdentityList(t *testing.T) {
	withTempHomeForIdentity(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name", "alpha"})
	})
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name", "beta"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "list"})
		if code != 0 {
			t.Errorf("list: exit code = %d, want 0", code)
		}
	})
	lines := nonEmptyLines(stdout)
	if len(lines) != 2 {
		t.Errorf("list output has %d lines, want 2; got: %q", len(lines), stdout)
	}
}

func TestIdentityListEmpty(t *testing.T) {
	withTempHomeForIdentity(t)
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "list"})
		if code != 0 {
			t.Errorf("list empty: exit code = %d, want 0", code)
		}
	})
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("list empty should print nothing, got: %q", stdout)
	}
}

func TestIdentityDelete(t *testing.T) {
	withTempHomeForIdentity(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init"})
	})
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "delete"})
		if code != 0 {
			t.Errorf("delete: exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("delete should say 'deleted', got: %q", stdout)
	}
	// Now show should fail.
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity", "show"})
		if code != 1 {
			t.Errorf("show after delete: exit code = %d, want 1", code)
		}
	})
	if !strings.Contains(stderr, "no identity") {
		t.Errorf("show after delete: stderr should say 'no identity', got: %q", stderr)
	}
}

func TestIdentityDeleteMissing(t *testing.T) {
	withTempHomeForIdentity(t)
	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "identity", "delete"})
		// Delete is idempotent — missing file is not an error.
		if code != 0 {
			t.Errorf("delete missing: exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "deleted") {
		t.Errorf("delete missing should still say 'deleted', got: %q", stdout)
	}
}

func TestIdentityUnknownVerb(t *testing.T) {
	withTempHomeForIdentity(t)
	stderr := captureStderr(t, func() {
		code := run([]string{"tunnelcat", "identity", "frobnicate"})
		if code != 2 {
			t.Errorf("unknown verb: exit code = %d, want 2", code)
		}
	})
	if !strings.Contains(stderr, "unknown verb") {
		t.Errorf("stderr should say 'unknown verb', got: %q", stderr)
	}
}

// extractPublicKey pulls the "public key: nodekey:..." line out
// of `tunnelcat identity init`'s output.
func extractPublicKey(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "public key:") {
			parts := strings.SplitN(line, "public key:", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// nonEmptyLines returns the non-empty, trimmed lines of s.
func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}
