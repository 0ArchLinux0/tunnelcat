// Package installpath provides the XDG-aware config dir for
// tunnelcat. Used by all packages that need to know where to
// put config files (identity keys, contacts, future services).
//
// Precedence:
//  1. TUNNELCAT_CONFIG_DIR (explicit override, for tests/CI)
//  2. XDG_CONFIG_HOME      (Linux/macOS portable default)
//  3. $HOME/.config        (XDG fallback)
//
// On Windows, XDG_CONFIG_HOME isn't a thing, so the user is
// expected to set TUNNELCAT_CONFIG_DIR. We don't do platform-
// specific behavior in M1; the XDG convention is portable
// enough for the friend test.
//
// networking-layer: n/a (filesystem paths, no network).
package installpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigDir returns the directory tunnelcat should use for
// user-level config. It does NOT create the directory; callers
// that want to write to it should MkdirAll(..., 0700) first.
func ConfigDir() (string, error) {
	if x := os.Getenv("TUNNELCAT_CONFIG_DIR"); x != "" {
		return x, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tunnelcat"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("installpath: cannot determine home dir: %w", err)
	}
	return filepath.Join(home, ".config", "tunnelcat"), nil
}

// EnsureConfigDir returns ConfigDir and creates it with mode
// 0700 if it doesn't exist. Most callers want this.
func EnsureConfigDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("installpath: mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// ErrNotFound is returned by [Lookup] when the user has no
// $HOME. We expose it as a sentinel so callers can distinguish
// "config doesn't exist" from "we can't even figure out where
// config would go."
var ErrNotFound = errors.New("installpath: cannot determine config dir")
