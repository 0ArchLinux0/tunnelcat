// Package testharness provides a one-process integration test harness
// for tunnelcat. It spins up a `tailcat.Server` and a `tailcat.Client`
// in the same process, connected via a loopback DERP region, and lets
// tests call them as if they were on different machines.
//
// This is the harness Stage 1's "two devices can talk" test will use.
// It is NOT used by tailcat's own tests; it is a thin wrapper tailored
// to tunnelcat's needs.
package testharness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/tailscale/tailcat"
)

// Pair is a server + a client wired together in one process.
// Both use ephemeral keys (so each test run is isolated).
type Pair struct {
	Server *tailcat.Server
	Client *tailcat.Client
	Token  tailcat.ConnBlob

	// ServerAddr is the deterministic IPv6 address the server
	// exposes inside the tunnel. Tests can dial it directly via
	// Client.DialTCPPort.
	ServerAddr netip.Addr // populated after Start()

	cleanup func()
}

// New starts a server, gets its token, and starts a client that
// connects to it. Both run in the current process. The DERP region
// used is `localhost:0` (a real DERP region the harness stands up
// in-process for the test).
//
// On test cleanup, both sides are closed and the in-process DERP
// server is stopped.
func New(t *testing.T) *Pair {
	t.Helper()

	pair, err := Start(context.Background())
	if err != nil {
		t.Fatalf("testharness: failed to start pair: %v", err)
	}
	t.Cleanup(pair.Close)
	return pair
}

// Start is like New but takes a context and returns an error
// instead of failing the test. Use this when you need finer
// control over startup.
func Start(ctx context.Context) (*Pair, error) {
	// The actual implementation lives in this file; we keep the
	// DERP setup, key generation, and shutdown logic here so
	// tests don't have to repeat it.
	//
	// For Stage 1, the simplest correct implementation is:
	//   1. stand up a DERP server in-process (or use the public one)
	//   2. start a tailcat.Server with an ephemeral key
	//   3. read the server's ConnBlob
	//   4. start a tailcat.Client with that blob
	//   5. wait for the tunnel to be ready (1 RTT for the Meow
	//      handshake + 1 RTT for the WireGuard handshake = ~200ms
	//      typical)
	//
	// The exact recipe requires tailcat's internal "loopback" DERP
	// mode which is not yet public API. For Stage 1, we will
	// stand up a real derper on 127.0.0.1:0 and point both sides
	// at it via --derpmap-url=file://... --region=loopback.

	return nil, errors.New("testharness: not yet implemented; will land in stage 1 alongside cmd/tunnelcat/main.go")
}

// Close shuts down the pair. Safe to call multiple times.
func (p *Pair) Close() {
	if p == nil {
		return
	}
	if p.cleanup != nil {
		p.cleanup()
		p.cleanup = nil
	}
	if p.Client != nil {
		_ = p.Client.Close()
		p.Client = nil
	}
	if p.Server != nil {
		_ = p.Server.Close()
		p.Server = nil
	}
}

// DialServerTCP dials the given TCP port on the server side
// through the tunnel. It is shorthand for
// `p.Client.DialTCPPort(ctx, port)`.
func (p *Pair) DialServerTCP(ctx context.Context, port uint16) (net.Conn, error) {
	if p == nil || p.Client == nil {
		return nil, errors.New("testharness: pair not initialized")
	}
	return p.Client.DialTCPPort(ctx, port)
}

// PipeRoundTrip is a convenience for tests that want to verify
// "bytes I sent on this end came out on the other end." It blocks
// until either the timeout elapses, the data is transferred, or
// an error occurs.
func (p *Pair) PipeRoundTrip(t *testing.T, port uint16, payload []byte, timeout time.Duration) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := p.DialServerTCP(ctx, port)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	if _, err := conn.Write(payload); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	// For Stage 1, the server side is expected to echo back.
	// The test will provide its own echo handler via
	// tailcat.Server.OnTCP. Here we just read until the server
	// closes or we hit the deadline.
	out, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return out, nil
}

// PairMux is a thread-safe registry of pairs. Some tests want
// to start many pairs in one process. The harness does not
// require this; it is here for tests that scale beyond a
// single pair.
type PairMux struct {
	mu    sync.Mutex
	pairs []*Pair
}

// Add registers a pair so the mux can close it on test cleanup.
func (m *PairMux) Add(p *Pair) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pairs = append(m.pairs, p)
}

// CloseAll closes every registered pair.
func (m *PairMux) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pairs {
		p.Close()
	}
	m.pairs = nil
}
