// Tests for `tunnelcat doctor`.
package main

import (
	"strings"
	"testing"
)

func TestDoctorAllGreen(t *testing.T) {
	// Pre-create an identity so the first check passes.
	dir := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "identity", "init", "--name=default"})
	})

	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "doctor"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0", code)
		}
	})
	if !strings.Contains(stdout, "diagnostic report") {
		t.Errorf("expected 'diagnostic report' in stdout, got: %q", stdout)
	}
	// At least one ✓ should be present (the identity check).
	if !strings.Contains(stdout, "✓") {
		t.Errorf("expected at least one ✓, got: %q", stdout)
	}
}

func TestDoctorMissingIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	stdout := captureStdout(t, func() {
		code := run([]string{"tunnelcat", "doctor"})
		if code != 0 {
			t.Errorf("exit code = %d, want 0 (doctor always exits 0)", code)
		}
	})
	if !strings.Contains(stdout, "no identity") {
		t.Errorf("expected 'no identity' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "✗") {
		t.Errorf("expected ✗ for missing identity, got: %q", stdout)
	}
}
