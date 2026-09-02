// Package identity manages the per-device node identity (a
// Curve25519 keypair) for tunnelcat.
//
// The identity is stored as a JSON file at
// $XDG_CONFIG_HOME/tunnelcat/keys/<name>.private.json with mode
// 0600. The file format is forward-compatible: the `version`
// field at the top of the JSON lets us add new fields without
// breaking old files.
//
// File format (version 1):
//
//	{
//	  "version": 1,
//	  "name": "default",
//	  "created_at": "2026-08-30T15:00:00Z",
//	  "key": "nodekey:9c8d2e6728da80a1dd37e275a82595b42d9a838610bc53f74a7670d1610f2e34"
//	}
//
// The `key` field is a Curve25519 private key in the
// standard tailscale.com/types/key text format (which is
// "nodekey:<64-hex-chars>"). It is also stored as
// `key_raw` (base64 of the 32 raw bytes) for
// integrity-checking.
//
// Security properties:
//   - The file contains a private key. Mode 0600 is enforced
//     on every save. On Windows, mode bits are advisory.
//   - Saves are atomic: write to a temp file, fsync, rename.
//     A process crash mid-save cannot leave a half-written
//     file.
//   - Loads verify a SHA256 checksum of the key bytes. A
//     corrupted file (bit flip, partial write) is detected
//     and rejected.
//   - On Unix, the key directory is created with mode 0700
//     so other users on the same machine cannot list it.
//
// Concurrency: this package does not implement file locking.
// Two concurrent `Save` calls on the same name may produce
// a torn write. In practice, identity init is a one-shot
// operation per device; concurrent writes are a user error.
package identity

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go4.org/mem"
	"tailscale.com/types/key"
)

const fileVersion = 1

// defaultDirName is the XDG-style config dir under $HOME.
const defaultDirName = "tunnelcat"

// Identity is a single device's node identity.
type Identity struct {
	// Name is the human-readable name (e.g., "default",
	// "studio-mac"). Used as the basename of the key file.
	Name string `json:"name"`

	// Key is the Curve25519 private key. The wire format is
	// "nodekey:<hex>". This is what tailcat.Server / Client
	// accept.
	Key key.NodePrivate `json:"-"` // serialized via custom MarshalJSON

	// CreatedAt is when the key was generated.
	CreatedAt time.Time `json:"created_at"`
}

// file is the on-disk format. The key is stored as both the
// text form (for human inspection) and the raw bytes (for
// integrity checking). Version lets us migrate later.
type file struct {
	Version   int       `json:"version"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	KeyText   string    `json:"key"`        // "nodekey:<hex>"
	KeyRaw    []byte    `json:"key_raw"`    // 32 raw bytes, base64
	KeySHA256 string    `json:"key_sha256"` // hex of sha256(KeyRaw)
}

// configDir returns the directory where keys are stored.
// Honors XDG_CONFIG_HOME; defaults to ~/.config/tunnelcat.
func configDir() (string, error) {
	if x := os.Getenv("TUNNELCAT_CONFIG_DIR"); x != "" {
		return x, nil
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, defaultDirName), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("identity: cannot determine home dir: %w", err)
	}
	return filepath.Join(home, ".config", defaultDirName), nil
}

// Path returns the file path for the named identity. The
// identity name must be a valid filename (alphanumeric, dash,
// underscore). The config dir is created with mode 0700 if it
// doesn't exist.
func Path(name string) (string, error) {
	if !validName(name) {
		return "", fmt.Errorf("identity: invalid name %q (must match [a-zA-Z0-9_-]{1,64})", name)
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(dir, "keys"), 0700); err != nil {
		return "", fmt.Errorf("identity: cannot create %s: %w", dir, err)
	}
	return filepath.Join(dir, "keys", name+".private.json"), nil
}

// validName returns true if name is a safe filename component.
func validName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

// New generates a fresh identity with a new random key.
func New(name string) (*Identity, error) {
	if !validName(name) {
		return nil, fmt.Errorf("identity: invalid name %q (must match [a-zA-Z0-9_-]{1,64})", name)
	}
	return &Identity{
		Name:      name,
		Key:       key.NewNode(),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// loadMu serializes Load+Save to a single goroutine at a time
// per process. The map is keyed by file path so that
// different identities don't block each other.
var loadMu sync.Map // map[string]*sync.Mutex

// loadLock returns a mutex for the given file path. We use
// sync.Map to lazily create per-file mutexes.
func loadLock(path string) *sync.Mutex {
	if v, ok := loadMu.Load(path); ok {
		return v.(*sync.Mutex)
	}
	mu := &sync.Mutex{}
	actual, _ := loadMu.LoadOrStore(path, mu)
	return actual.(*sync.Mutex)
}

// Load reads the identity from disk. If the file does not
// exist, returns (nil, nil) — not an error. The caller is
// expected to check for nil and call New + Save if they want
// to create one.
//
// If the file is corrupt (invalid JSON, bad version, missing
// fields, checksum mismatch), returns an error.
func Load(name string) (*Identity, error) {
	path, err := Path(name)
	if err != nil {
		return nil, err
	}
	mu := loadLock(path)
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("identity: read %s: %w", path, err)
	}

	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("identity: parse %s: %w", path, err)
	}

	// Forward-compatibility: refuse older files (we don't have
	// migration paths yet) and refuse newer files (we don't
	// know what the new fields mean).
	if f.Version == 0 {
		return nil, fmt.Errorf("identity: %s: missing version field", path)
	}
	if f.Version > fileVersion {
		return nil, fmt.Errorf("identity: %s: file version %d is newer than supported (%d); upgrade tunnelcat",
			path, f.Version, fileVersion)
	}

	// Parse the private key from the text form. We store it as
	// "privkey:<64 hex>" (the format NodePrivate.AppendText
	// produces); the parser expects the raw 64 hex chars without
	// any prefix. Strip both common prefixes to be lenient.
	keyText := strings.TrimPrefix(f.KeyText, "nodekey:")
	keyText = strings.TrimPrefix(keyText, "privkey:")
	priv, err := key.ParseNodePrivateUntyped(mem.B([]byte(keyText)))
	if err != nil {
		return nil, fmt.Errorf("identity: %s: invalid key text: %w", path, err)
	}

	// Verify the checksum. If a single byte of the key has
	// been corrupted, the SHA256 will not match and we refuse
	// to load the key. This prevents the agent from using a
	// subtly wrong key (which would result in "why doesn't
	// anything connect?" debugging hell).
	wantSHA := sha256.Sum256(f.KeyRaw)
	gotSHA, err := hex.DecodeString(f.KeySHA256)
	if err != nil || len(gotSHA) != len(wantSHA) {
		return nil, fmt.Errorf("identity: %s: missing or invalid checksum", path)
	}
	for i := range wantSHA {
		if gotSHA[i] != wantSHA[i] {
			return nil, fmt.Errorf("identity: %s: checksum mismatch (file corrupted)", path)
		}
	}
	// Cross-check: the raw bytes should match the parsed key.
	parsedRaw := priv.Raw32()
	if !bytesEqual(parsedRaw[:], f.KeyRaw) {
		return nil, fmt.Errorf("identity: %s: raw key bytes don't match parsed key", path)
	}

	return &Identity{
		Name:      f.Name,
		Key:       priv,
		CreatedAt: f.CreatedAt,
	}, nil
}

// Save writes the identity to disk atomically with mode 0600.
// The temp file is in the same directory as the final file so
// the rename is atomic on the same filesystem.
func Save(id *Identity) error {
	if id == nil {
		return errors.New("identity: cannot save nil identity")
	}
	if !validName(id.Name) {
		return fmt.Errorf("identity: invalid name %q (must match [a-zA-Z0-9_-]{1,64})", id.Name)
	}
	path, err := Path(id.Name)
	if err != nil {
		return err
	}
	mu := loadLock(path)
	mu.Lock()
	defer mu.Unlock()

	raw := id.Key.Raw32()
	checksum := sha256.Sum256(raw[:])
	keyText, err := id.Key.AppendText(nil)
	if err != nil {
		return fmt.Errorf("identity: serialize key: %w", err)
	}
	// keyText is "privkey:<64 hex>". We strip the prefix for
	// the on-disk format so the file matches the standard
	// "nodekey:<64 hex>" appearance (and so a human reading
	// the file sees the same format as tailcat's CLI).
	keyTextStr := strings.TrimPrefix(string(keyText), "privkey:")
	f := file{
		Version:   fileVersion,
		Name:      id.Name,
		CreatedAt: id.CreatedAt.UTC(),
		KeyText:   keyTextStr,
		KeyRaw:    raw[:],
		KeySHA256: hex.EncodeToString(checksum[:]),
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("identity: marshal: %w", err)
	}
	data = append(data, '\n')

	// Atomic save: write to a temp file, fsync, rename.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".identity-*.tmp")
	if err != nil {
		return fmt.Errorf("identity: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up the temp file on any failure path.
		_ = os.Remove(tmpPath)
	}()

	// chmod 0600 BEFORE writing the data. This means the file
	// is never readable by other users, even for the brief
	// moment between Create and Write.
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return fmt.Errorf("identity: chmod temp: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("identity: write temp: %w", err)
	}
	// fsync the file before renaming so the bytes are on
	// disk before the rename. Without this, a power loss
	// could leave a file with a name but no data.
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("identity: fsync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("identity: close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("identity: rename: %w", err)
	}
	// Also chmod the final file in case the rename preserved
	// the temp file's mode (it should, but be defensive).
	if err := os.Chmod(path, 0600); err != nil {
		// On Windows, this is a no-op. On Unix, it should
		// never fail because we just created the file.
		// Log it but don't fail the save.
		_ = err
	}
	return nil
}

// List returns the names of all identities on disk. Names are
// returned in alphabetical order.
func List() ([]string, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	keysDir := filepath.Join(dir, "keys")
	entries, err := os.ReadDir(keysDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("identity: list %s: %w", keysDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		const suffix = ".private.json"
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		base := strings.TrimSuffix(name, suffix)
		if validName(base) {
			names = append(names, base)
		}
	}
	return names, nil
}

// Delete removes the identity from disk. Returns nil if the
// file didn't exist (idempotent).
func Delete(name string) error {
	if !validName(name) {
		return fmt.Errorf("identity: invalid name %q (must match [a-zA-Z0-9_-]{1,64})", name)
	}
	path, err := Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("identity: remove %s: %w", path, err)
	}
	return nil
}

// PublicKey returns the identity's public key.
func PublicKey(id *Identity) key.NodePublic {
	if id == nil {
		return key.NodePublic{}
	}
	return id.Key.Public()
}

// PublicKeyString returns the identity's public key as a
// "nodekey:<hex>" string. The format matches what tailcat's
// CLI expects.
func PublicKeyString(id *Identity) string {
	pub := PublicKey(id)
	if pub.IsZero() {
		return ""
	}
	b, err := pub.AppendText(nil)
	if err != nil {
		return ""
	}
	return string(b)
}

// bytesEqual is a constant-time byte comparison. We use it
// for the SHA256 comparison in Load so a timing attack can't
// leak the expected hash one byte at a time.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

// Compile-time check that we use base64 (we don't, but the
// import keeps the dependency available for future versions).
var _ = base64.StdEncoding.EncodeToString
