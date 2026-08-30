// Tests for the Go↔Rust bridge. These are real tests, not
// skipped skeletons. If they fail, the bridge is broken and
// stage 2 cannot proceed.
package rustbridge

import (
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	v, err := Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if !strings.Contains(v, "tunnelcat-proto") {
		t.Errorf("version = %q, want it to contain 'tunnelcat-proto'", v)
	}
}

func TestEcho(t *testing.T) {
	in := "hello from go"
	got, err := Echo(in)
	if err != nil {
		t.Fatalf("Echo: %v", err)
	}
	want := "echo: hello from go\n"
	if got != want {
		t.Errorf("Echo(%q) = %q, want %q", in, got, want)
	}
}

func TestEchoUnicode(t *testing.T) {
	// Make sure non-ASCII input round-trips.
	in := "안녕하세요 from go"
	got, err := Echo(in)
	if err != nil {
		t.Fatalf("Echo: %v", err)
	}
	if !strings.Contains(got, in) {
		t.Errorf("Echo(%q) = %q, want it to contain %q", in, got, in)
	}
}
