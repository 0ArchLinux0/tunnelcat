// Tests for the testharness. Right now these are skeletons that
// document the test cases Stage 1 will verify. Once Start() is
// implemented (in stage 1), these tests will run for real.
package testharness

import (
	"context"
	"testing"
	"time"
)

// TestPairStartsAndCloses is the most basic test: can we even
// start a server and a client, and close them cleanly? If this
// fails, nothing else in Stage 1 is going to work.
func TestPairStartsAndCloses(t *testing.T) {
	t.Skip("skipped: Start() is not yet implemented; will land in stage 1")

	pair, err := Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if pair.Server == nil {
		t.Fatal("expected non-nil Server")
	}
	if pair.Client == nil {
		t.Fatal("expected non-nil Client")
	}
	if pair.Token == "" {
		t.Fatal("expected non-empty Token")
	}
	pair.Close()
	// Closing twice should not panic.
	pair.Close()
}

// TestPipeRoundTrip is the actual Stage 1 verify: send bytes
// through the tunnel, get them back, compare. If this passes,
// the CLI is functionally correct (modulo the rest of the CLI
// surface).
func TestPipeRoundTrip(t *testing.T) {
	t.Skip("skipped: Start() is not yet implemented; will land in stage 1")

	pair := New(t)
	payload := []byte("hello tunnelcat")
	got, err := pair.PipeRoundTrip(t, 12345, payload, 5*time.Second)
	if err != nil {
		t.Fatalf("PipeRoundTrip: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

// TestMultiplePairsIndependent makes sure two pairs in the same
// process don't interfere with each other. Useful for
// "one process simulates two devices" tests in later stages.
func TestMultiplePairsIndependent(t *testing.T) {
	t.Skip("skipped: Start() is not yet implemented; will land in stage 1")

	mux := &PairMux{}
	t.Cleanup(mux.CloseAll)

	for i := 0; i < 3; i++ {
		p, err := Start(context.Background())
		if err != nil {
			t.Fatalf("pair %d: Start: %v", i, err)
		}
		mux.Add(p)
	}

	// Smoke: each pair should have a unique token.
	seen := make(map[string]bool)
	mux.mu.Lock()
	for _, p := range mux.pairs {
		tok := string(p.Token)
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
	mux.mu.Unlock()
}
