// Package contacts manages the per-device contact list for
// tunnelcat.
//
// The contact list is a YAML file at
// $XDG_CONFIG_HOME/tunnelcat/contacts.yaml. Each contact is a
// named peer: their public key, optional last-seen / last-addr
// metadata, and an optional note.
//
// File format (version 1):
//
//   version: 1
//   contacts:
//     - name: studio-mac
//       pubkey: nodekey:abc...
//       added_at: 2026-08-30T15:00:00Z
//       last_seen: 2026-08-30T15:30:00Z
//       last_addr: "100.64.0.2:22"
//       note: "main workstation"
//
// All fields except name and pubkey are optional. The package
// does not validate pubkey format — callers should validate
// (tunnelcat's CLI does this with a regex) before Save.
//
// Security: the file contains only PUBLIC keys, not private
// ones. Mode 0600 is still enforced as a defense-in-depth
// measure (and to match the identity file's behavior).
package contacts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go4.org/mem"
	"gopkg.in/yaml.v3"
)

const fileVersion = 1

// Contact is a single named peer.
type Contact struct {
	Name     string     `yaml:"name"`
	Pubkey   string     `yaml:"pubkey"`
	AddedAt  time.Time  `yaml:"added_at,omitempty"`
	LastSeen *time.Time `yaml:"last_seen,omitempty"`
	LastAddr string     `yaml:"last_addr,omitempty"`
	Note     string     `yaml:"note,omitempty"`
}

// File is the on-disk contact list.
type File struct {
	Version int       `yaml:"version"`
	Contacts []Contact `yaml:"contacts"`
}

// configDir returns the directory where the contacts file is
// stored. Same precedence as internal/identity: TUNNELCAT_CONFIG_DIR
// > XDG_CONFIG_HOME > ~/.config/tunnelcat.
func configDir() (string, error) {
	if x := os.Getenv("TUNNELCAT_CONFIG_DIR"); x != "" {
		return x, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "tunnelcat"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("contacts: cannot determine home dir: %w", err)
	}
	return filepath.Join(home, ".config", "tunnelcat"), nil
}

// Path returns the file path for the contacts list.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("contacts: cannot create %s: %w", dir, err)
	}
	return filepath.Join(dir, "contacts.yaml"), nil
}

// loadMu serializes Load+Save on the contacts file.
var loadMu sync.Mutex

// withLockedFile loads, calls fn, and saves — all under the
// mutex. If fn returns an error, the save is skipped. This is
// the right primitive for any read-modify-write operation.
func withLockedFile(fn func(*File) error) error {
	loadMu.Lock()
	defer loadMu.Unlock()

	f, err := loadNoLock()
	if err != nil {
		return err
	}
	if err := fn(f); err != nil {
		return err
	}
	return saveLocked(f)
}

// Load reads the contacts file. If the file does not exist,
// returns an empty File (not nil, not an error). The caller
// is expected to check the result and Save if they want to
// persist changes.
//
// Load is the primitive that takes the mutex. Use Save (also
// takes the mutex) for direct writes. Use withLockedFile for
// read-modify-write operations.
func Load() (*File, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	return loadNoLock()
}

// Save writes the contact list to disk atomically with mode 0600.
func Save(f *File) error {
	loadMu.Lock()
	defer loadMu.Unlock()
	return saveLocked(f)
}

// saveLocked is the inner Save that does NOT take the mutex.
// Callers must already hold loadMu.
func saveLocked(f *File) error {
	if f == nil {
		return errors.New("contacts: cannot save nil file")
	}
	if f.Version == 0 {
		f.Version = fileVersion
	}

	path, err := Path()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("contacts: marshal: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".contacts-*.tmp")
	if err != nil {
		return fmt.Errorf("contacts: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("contacts: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("contacts: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("contacts: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("contacts: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("contacts: rename: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = err
	}
	return nil
}

// Add inserts a contact. If a contact with the same name
// already exists, returns an error (use Update to overwrite,
// or Remove first).
// Add inserts a contact. If a contact with the same name
// already exists, returns an error (use Update to overwrite,
// or Remove first).
func Add(c Contact) error {
	if c.Name == "" {
		return errors.New("contacts: name is required")
	}
	if c.Pubkey == "" {
		return errors.New("contacts: pubkey is required")
	}
	if c.AddedAt.IsZero() {
		c.AddedAt = time.Now().UTC()
	}

	return withLockedFile(func(f *File) error {
		for _, existing := range f.Contacts {
			if existing.Name == c.Name {
				return fmt.Errorf("contacts: %q already exists; use 'update' to overwrite or 'remove' first", c.Name)
			}
		}
		f.Contacts = append(f.Contacts, c)
		return nil
	})
}

// Update replaces a contact by name. If not found, returns an error.
// All fields except Name are updated; the Pubkey is required.
func Update(c Contact) error {
	if c.Name == "" {
		return errors.New("contacts: name is required")
	}
	if c.Pubkey == "" {
		return errors.New("contacts: pubkey is required")
	}
	return withLockedFile(func(f *File) error {
		found := false
		for i := range f.Contacts {
			if f.Contacts[i].Name == c.Name {
				if c.AddedAt.IsZero() {
					c.AddedAt = f.Contacts[i].AddedAt
				}
				f.Contacts[i] = c
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("contacts: %q not found", c.Name)
		}
		return nil
	})
}

// Remove deletes a contact by name. Idempotent: returns nil if
// the contact didn't exist.
func Remove(name string) error {
	return withLockedFile(func(f *File) error {
		out := f.Contacts[:0]
		found := false
		for _, c := range f.Contacts {
			if c.Name == name {
				found = true
				continue
			}
			out = append(out, c)
		}
		if !found {
			return nil
		}
		f.Contacts = out
		return nil
	})
}

// Find returns the contact with the given name, or (nil, false)
// if not found.
func Find(name string) (*Contact, bool) {
	loadMu.Lock()
	defer loadMu.Unlock()
	f, err := loadNoLock()
	if err != nil {
		return nil, false
	}
	for i, c := range f.Contacts {
		if c.Name == name {
			return &f.Contacts[i], true
		}
	}
	return nil, false
}

// List returns all contacts.
func List() ([]Contact, error) {
	loadMu.Lock()
	defer loadMu.Unlock()
	f, err := loadNoLock()
	if err != nil {
		return nil, err
	}
	return f.Contacts, nil
}

// loadNoLock is the inner Load that does NOT take the mutex.
// Callers must already hold loadMu.
func loadNoLock() (*File, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &File{Version: fileVersion, Contacts: []Contact{}}, nil
		}
		return nil, fmt.Errorf("contacts: read %s: %w", path, err)
	}

	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("contacts: parse %s: %w", path, err)
	}

	if f.Version == 0 {
		return nil, fmt.Errorf("contacts: %s: missing version field", path)
	}
	if f.Version > fileVersion {
		return nil, fmt.Errorf("contacts: %s: file version %d is newer than supported (%d); upgrade tunnelcat",
			path, f.Version, fileVersion)
	}
	if f.Contacts == nil {
		f.Contacts = []Contact{}
	}
	return &f, nil
}

// keep the linter happy on the imports
var _ = mem.B
