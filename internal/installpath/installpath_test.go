// Tests for the installpath package.
package installpath

import (
	"path/filepath"
	"testing"
)

func TestConfigDirTunnelcatOverride(t *testing.T) {
	t.Setenv("TUNNELCAT_CONFIG_DIR", "/tmp/tc-test")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/tc-test" {
		t.Errorf("got %q, want /tmp/tc-test", got)
	}
}

func TestConfigDirXDG(t *testing.T) {
	t.Setenv("TUNNELCAT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("/tmp/xdg", "tunnelcat") {
		t.Errorf("got %q, want %q", got, filepath.Join("/tmp/xdg", "tunnelcat"))
	}
}

func TestConfigDirHomeDefault(t *testing.T) {
	t.Setenv("TUNNELCAT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/home")
	got, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/home", ".config", "tunnelcat")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEnsureConfigDirCreates(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", filepath.Join(tmp, "newdir"))
	got, err := EnsureConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(tmp, "newdir") {
		t.Errorf("got %q, want %q", got, filepath.Join(tmp, "newdir"))
	}
}
