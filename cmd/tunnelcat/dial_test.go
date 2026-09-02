// Tests for `tunnelcat dial <name>` (contact-name resolution).
package main

import (
	"strings"
	"testing"
)

func TestResolveDialArgRawToken(t *testing.T) {
	withTempHomeContact(t)
	got := resolveDialArg("tc" + strings.Repeat("x", 60))
	if string(got) != "tc"+strings.Repeat("x", 60) {
		t.Errorf("resolveDialArg raw = %q, want pass-through", got)
	}
}

func TestResolveDialArgUnknownContact(t *testing.T) {
	withTempHomeContact(t)
	stderr := captureStderr(t, func() {
		got := resolveDialArg("ghost")
		if got != "" {
			t.Errorf("expected empty result, got %q", got)
		}
	})
	if !strings.Contains(stderr, "not a known contact") {
		t.Errorf("expected 'not a known contact' in stderr, got %q", stderr)
	}
}

func TestResolveDialArgContactNoBlob(t *testing.T) {
	withTempHomeContact(t)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
	})
	stderr := captureStderr(t, func() {
		got := resolveDialArg("alice")
		if got != "" {
			t.Errorf("expected empty result, got %q", got)
		}
	})
	if !strings.Contains(stderr, "no ConnBlob set") {
		t.Errorf("expected 'no ConnBlob set' in stderr, got %q", stderr)
	}
}

func TestResolveDialArgContactWithBlob(t *testing.T) {
	withTempHomeContact(t)
	blob := "tc" + strings.Repeat("z", 60)
	captureStdout(t, func() {
		run([]string{"tunnelcat", "contact", "add", "alice",
			"nodekey:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})
		run([]string{"tunnelcat", "contact", "set-blob", "alice", blob})
	})
	got := resolveDialArg("alice")
	if string(got) != blob {
		t.Errorf("resolveDialArg = %q, want %q", got, blob)
	}
}
