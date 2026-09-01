// Tests for the identity package. The user asked for "a lot
// of tests" so this file is extensive: every function gets
// happy-path, sad-path, edge-case, and adversarial tests.
//
// All tests use a temp HOME dir via TUNNELCAT_CONFIG_DIR so
// they don't touch the user's real ~/.config/tunnelcat/.
package identity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"tailscale.com/types/key"
)

// withTempHome sets TUNNELCAT_CONFIG_DIR to a fresh temp dir
// for the duration of the test, and returns the path. The dir
// is cleaned up automatically when the test ends.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", dir) // also override, in case
	return dir
}

// ─────────────────────────────────────────────────────────────
// validName
// ─────────────────────────────────────────────────────────────

func TestValidName(t *testing.T) {
	good := []string{"default", "studio-mac", "home_server", "laptop-2", "a", strings.Repeat("x", 64)}
	for _, n := range good {
		if !validName(n) {
			t.Errorf("validName(%q) = false, want true", n)
		}
	}
	bad := []string{
		"",                         // empty
		strings.Repeat("x", 65),    // too long
		"with space",                // space
		"with/slash",                // slash
		"with\\backslash",           // backslash
		"with:colon",                // colon (invalid on Windows)
		"with*star",                 // star
		"..",                        // path traversal
		".",                         // path traversal
		"../etc/passwd",             // path traversal
		"with?question",             // question mark
		"with\"quote",               // quote
		"with|pipe",                 // pipe
		"with\nnewline",             // newline
	}
	for _, n := range bad {
		if validName(n) {
			t.Errorf("validName(%q) = true, want false", n)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// New
// ─────────────────────────────────────────────────────────────

func TestNew(t *testing.T) {
	id, err := New("test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if id.Name != "test" {
		t.Errorf("Name = %q, want %q", id.Name, "test")
	}
	if id.Key.IsZero() {
		t.Error("Key is zero, want a fresh key")
	}
	if id.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero, want the current time")
	}
	// CreatedAt should be recent (within the last 5 seconds).
	if d := time.Since(id.CreatedAt); d < 0 || d > 5*time.Second {
		t.Errorf("CreatedAt = %v, want within 5s of now", id.CreatedAt)
	}
}

func TestNewInvalidName(t *testing.T) {
	for _, n := range []string{"", "with space", "../escape"} {
		_, err := New(n)
		if err == nil {
			t.Errorf("New(%q) succeeded, want error", n)
		}
	}
}

func TestNewProducesUniqueKeys(t *testing.T) {
	a, _ := New("a")
	b, _ := New("b")
	if a.Key.Equal(b.Key) {
		t.Error("two New() calls produced the same key — randomness is broken")
	}
}

// ─────────────────────────────────────────────────────────────
// Save + Load round-trip
// ─────────────────────────────────────────────────────────────

func TestSaveLoadRoundTrip(t *testing.T) {
	withTempHome(t)

	id, err := New("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(id); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load("default")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil, want non-nil")
	}
	if !loaded.Key.Equal(id.Key) {
		t.Error("loaded key != saved key")
	}
	if loaded.Name != id.Name {
		t.Errorf("loaded Name = %q, want %q", loaded.Name, id.Name)
	}
	if !loaded.CreatedAt.Equal(id.CreatedAt) {
		t.Errorf("loaded CreatedAt = %v, want %v", loaded.CreatedAt, id.CreatedAt)
	}
}

func TestLoadMissingFile(t *testing.T) {
	withTempHome(t)
	loaded, err := Load("nonexistent")
	if err != nil {
		t.Errorf("Load of missing file: got error %v, want nil", err)
	}
	if loaded != nil {
		t.Errorf("Load of missing file: got %v, want nil", loaded)
	}
}

func TestSaveNilIdentity(t *testing.T) {
	if err := Save(nil); err == nil {
		t.Error("Save(nil) succeeded, want error")
	}
}

func TestSaveInvalidName(t *testing.T) {
	id := &Identity{Name: "with space", Key: key.NewNode(), CreatedAt: time.Now()}
	if err := Save(id); err == nil {
		t.Error("Save with invalid name succeeded, want error")
	}
}

// ─────────────────────────────────────────────────────────────
// File format
// ─────────────────────────────────────────────────────────────

func TestFileFormatIsJSON(t *testing.T) {
	withTempHome(t)
	id, _ := New("fmt")
	if err := Save(id); err != nil {
		t.Fatal(err)
	}
	path, _ := Path("fmt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		t.Errorf("file is not valid JSON: %v\nfile contents:\n%s", err, data)
	}
	if f.Version != fileVersion {
		t.Errorf("file.Version = %d, want %d", f.Version, fileVersion)
	}
	if f.Name != "fmt" {
		t.Errorf("file.Name = %q, want %q", f.Name, "fmt")
	}
}

func TestFileHasChecksum(t *testing.T) {
	withTempHome(t)
	id, _ := New("chk")
	if err := Save(id); err != nil {
		t.Fatal(err)
	}
	path, _ := Path("chk")
	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)

	want := sha256.Sum256(f.KeyRaw)
	got, _ := hex.DecodeString(f.KeySHA256)
	if !bytes.Equal(want[:], got) {
		t.Errorf("checksum in file (%x) != sha256(key_raw) (%x)", got, want)
	}
}

// ─────────────────────────────────────────────────────────────
// File permissions
// ─────────────────────────────────────────────────────────────

func TestFileHas0600Mode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod is a no-op on Windows")
	}
	withTempHome(t)
	id, _ := New("perm")
	if err := Save(id); err != nil {
		t.Fatal(err)
	}
	path, _ := Path("perm")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("file mode = %o, want 0600", mode)
	}
}

func TestDirectoryHas0700Mode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod is a no-op on Windows")
	}
	withTempHome(t)
	id, _ := New("dir")
	Save(id)
	dir := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "keys")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	mode := info.Mode().Perm()
	if mode != 0700 {
		t.Errorf("keys dir mode = %o, want 0700", mode)
	}
}

// ─────────────────────────────────────────────────────────────
// Corruption detection
// ─────────────────────────────────────────────────────────────

func TestLoadRejectsCorruptJSON(t *testing.T) {
	withTempHome(t)
	id, _ := New("corrupt")
	Save(id)
	path, _ := Path("corrupt")

	// Overwrite with garbage.
	if err := os.WriteFile(path, []byte("{not valid json"), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load("corrupt")
	if err == nil {
		t.Error("Load of corrupt JSON: got no error, want error")
	}
	if loaded != nil {
		t.Error("Load of corrupt JSON: got non-nil, want nil")
	}
}

func TestLoadRejectsVersionZero(t *testing.T) {
	withTempHome(t)
	id, _ := New("v0")
	Save(id)
	path, _ := Path("v0")

	// Set version to 0 (missing).
	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)
	f.Version = 0
	data, _ = json.Marshal(f)
	os.WriteFile(path, data, 0600)

	_, err := Load("v0")
	if err == nil {
		t.Error("Load with version 0: got no error, want error")
	}
}

func TestLoadRejectsFutureVersion(t *testing.T) {
	withTempHome(t)
	id, _ := New("vfut")
	Save(id)
	path, _ := Path("vfut")

	// Set version to 999 (newer than supported).
	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)
	f.Version = 999
	data, _ = json.Marshal(f)
	os.WriteFile(path, data, 0600)

	_, err := Load("vfut")
	if err == nil {
		t.Error("Load with version 999: got no error, want error")
	}
	if err != nil && !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("error message should mention 'newer than supported', got: %v", err)
	}
}

func TestLoadRejectsChecksumMismatch(t *testing.T) {
	withTempHome(t)
	id, _ := New("sum")
	Save(id)
	path, _ := Path("sum")

	// Flip a byte in the key_raw field.
	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)
	if len(f.KeyRaw) > 0 {
		f.KeyRaw[0] ^= 0xFF
	}
	data, _ = json.Marshal(f)
	os.WriteFile(path, data, 0600)

	_, err := Load("sum")
	if err == nil {
		t.Error("Load with checksum mismatch: got no error, want error")
	}
	if err != nil && !strings.Contains(err.Error(), "checksum") {
		t.Errorf("error message should mention 'checksum', got: %v", err)
	}
}

func TestLoadRejectsKeyTextMismatch(t *testing.T) {
	withTempHome(t)
	id, _ := New("ktm")
	Save(id)
	path, _ := Path("ktm")

	// Corrupt the key_text field while keeping key_raw intact.
	// The cross-check (raw bytes must match parsed key) should
	// catch this.
	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)
	f.KeyText = "nodekey:" + strings.Repeat("0", 64)
	data, _ = json.Marshal(f)
	os.WriteFile(path, data, 0600)

	_, err := Load("ktm")
	if err == nil {
		t.Error("Load with key_text mismatch: got no error, want error")
	}
}

func TestLoadRejectsMissingChecksum(t *testing.T) {
	withTempHome(t)
	id, _ := New("ms")
	Save(id)
	path, _ := Path("ms")

	data, _ := os.ReadFile(path)
	var f file
	json.Unmarshal(data, &f)
	f.KeySHA256 = ""
	data, _ = json.Marshal(f)
	os.WriteFile(path, data, 0600)

	_, err := Load("ms")
	if err == nil {
		t.Error("Load with missing checksum: got no error, want error")
	}
}

// ─────────────────────────────────────────────────────────────
// Atomicity
// ─────────────────────────────────────────────────────────────

func TestSaveIsAtomic(t *testing.T) {
	withTempHome(t)
	id, _ := New("atom")
	Save(id)
	path, _ := Path("atom")

	// Save a second time. There should be no temp file left
	// behind; the rename should have replaced the file.
	Save(id)
	dir := filepath.Dir(path)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".identity-") {
			t.Errorf("temp file %s left behind after Save", e.Name())
		}
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	withTempHome(t)
	a, _ := New("over")
	Save(a)
	b, _ := New("over")
	Save(b)
	loaded, err := Load("over")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Key.Equal(a.Key) {
		t.Error("Load returned the old key after Save with a new key")
	}
	if !loaded.Key.Equal(b.Key) {
		t.Error("Load returned neither old nor new key")
	}
}

// ─────────────────────────────────────────────────────────────
// PublicKey
// ─────────────────────────────────────────────────────────────

func TestPublicKeyString(t *testing.T) {
	id, _ := New("pk")
	pub := PublicKeyString(id)
	if !strings.HasPrefix(pub, "nodekey:") {
		t.Errorf("PublicKeyString = %q, want prefix 'nodekey:'", pub)
	}
	if len(pub) != len("nodekey:")+64 {
		t.Errorf("PublicKeyString length = %d, want %d (nodekey: + 64 hex)",
			len(pub), len("nodekey:")+64)
	}
}

func TestPublicKeyNilIdentity(t *testing.T) {
	if got := PublicKeyString(nil); got != "" {
		t.Errorf("PublicKeyString(nil) = %q, want empty string", got)
	}
}

func TestPublicKeyDeterministic(t *testing.T) {
	id, _ := New("det")
	p1 := PublicKeyString(id)
	p2 := PublicKeyString(id)
	if p1 != p2 {
		t.Errorf("PublicKeyString not deterministic: %q vs %q", p1, p2)
	}
}

// ─────────────────────────────────────────────────────────────
// List
// ─────────────────────────────────────────────────────────────

func TestListEmpty(t *testing.T) {
	withTempHome(t)
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Errorf("List() with no identities = %v, want empty", names)
	}
}

func TestListReturnsSavedNames(t *testing.T) {
	withTempHome(t)
	Save(mustNew(t, "alpha"))
	Save(mustNew(t, "beta"))
	Save(mustNew(t, "gamma"))
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "beta", "gamma"}
	if !equalStrings(names, want) {
		t.Errorf("List() = %v, want %v", names, want)
	}
}

func TestListIgnoresNonIdentityFiles(t *testing.T) {
	withTempHome(t)
	Save(mustNew(t, "real"))
	dir := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "keys")
	os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("not an identity"), 0600)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0600)
	names, _ := List()
	if !equalStrings(names, []string{"real"}) {
		t.Errorf("List() = %v, want [real] (should ignore .txt and .json)", names)
	}
}

func TestListIgnoresInvalidNames(t *testing.T) {
	withTempHome(t)
	Save(mustNew(t, "good"))
	dir := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "keys")
	// A file with a name that doesn't pass validName. This
	// shouldn't happen normally (we always use validName), but
	// a user might drop a weird file in there.
	os.WriteFile(filepath.Join(dir, "with space.private.json"), []byte("{}"), 0600)
	names, _ := List()
	if !equalStrings(names, []string{"good"}) {
		t.Errorf("List() = %v, want [good] (should ignore invalid name)", names)
	}
}

// ─────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────

func TestDeleteExisting(t *testing.T) {
	withTempHome(t)
	Save(mustNew(t, "del"))
	if err := Delete("del"); err != nil {
		t.Fatal(err)
	}
	loaded, _ := Load("del")
	if loaded != nil {
		t.Error("Delete: Load returned non-nil after delete")
	}
}

func TestDeleteNonExistent(t *testing.T) {
	withTempHome(t)
	// Delete of a non-existent file should be idempotent (no error).
	if err := Delete("never-existed"); err != nil {
		t.Errorf("Delete of non-existent: got error %v, want nil", err)
	}
}

func TestDeleteInvalidName(t *testing.T) {
	if err := Delete("../escape"); err == nil {
		t.Error("Delete with invalid name: got no error, want error")
	}
}

// ─────────────────────────────────────────────────────────────
// Concurrency
// ─────────────────────────────────────────────────────────────

func TestConcurrentSaveDoesNotCorrupt(t *testing.T) {
	withTempHome(t)
	// Spawn N goroutines that all save the same identity. The
	// mutex in loadLock should serialize them. After the dust
	// settles, the file should be loadable and the key should
	// be one of the saved keys.
	const N = 20
	ids := make([]*Identity, N)
	for i := 0; i < N; i++ {
		ids[i], _ = New("concurrent")
	}
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(id *Identity) {
			defer wg.Done()
			if err := Save(id); err != nil {
				t.Errorf("Save: %v", err)
			}
		}(ids[i])
	}
	wg.Wait()
	loaded, err := Load("concurrent")
	if err != nil {
		t.Fatalf("Load after concurrent saves: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil after concurrent saves")
	}
	// The loaded key should be one of the ones we saved.
	found := false
	for _, id := range ids {
		if loaded.Key.Equal(id.Key) {
			found = true
			break
		}
	}
	if !found {
		t.Error("Loaded key is not one of the saved keys (file may be torn)")
	}
}

func TestConcurrentLoadAndSave(t *testing.T) {
	withTempHome(t)
	Save(mustNew(t, "mix"))
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			_, _ = Load("mix")
		}
	}()
	for i := 0; i < 50; i++ {
		id, _ := New("mix")
		_ = Save(id)
	}
	<-done
	// Final load should still succeed.
	if _, err := Load("mix"); err != nil {
		t.Errorf("Load after concurrent mix: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────
// Path / config dir
// ─────────────────────────────────────────────────────────────

func TestPathCreatesKeyDir(t *testing.T) {
	withTempHome(t)
	path, err := Path("pcd")
	if err != nil {
		t.Fatal(err)
	}
	// The keys dir should have been created.
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("keys dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("keys path is not a directory")
	}
}

func TestPathHonorsTunnelcatConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent/should/not/be/used")
	path, err := Path("h")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "keys", "h.private.json")
	if path != want {
		t.Errorf("Path = %q, want %q (TUNNELCAT_CONFIG_DIR not honored)", path, want)
	}
}

func TestPathFallsBackToXDG(t *testing.T) {
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home) // for os.UserHomeDir
	path, err := Path("xdg")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(xdg, "tunnelcat", "keys", "xdg.private.json")
	if path != want {
		t.Errorf("Path = %q, want %q (XDG_CONFIG_HOME not honored)", path, want)
	}
}

func TestPathFallsBackToHomeDotConfig(t *testing.T) {
	// When neither env var is set, fall back to ~/.config/tunnelcat
	xdg := t.TempDir()
	home := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	// os.UserHomeDir on Unix reads $HOME. We need to make sure
	// it does. The UserHomeDir function may also use other
	// sources (passwd, etc.), so we rely on the env override.
	path, err := Path("home")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "tunnelcat", "keys", "home.private.json")
	if path != want {
		t.Errorf("Path = %q, want %q (HOME fallback not honored)", path, want)
	}
	_ = xdg // not used; keeping for symmetry
}

// ─────────────────────────────────────────────────────────────
// Path traversal attack
// ─────────────────────────────────────────────────────────────

func TestPathRejectsTraversalAttempts(t *testing.T) {
	withTempHome(t)
	// validName should catch these, but let's also verify Path
	// doesn't allow them.
	for _, name := range []string{"../etc/passwd", "..\\windows", "subdir/file", "/abs"} {
		_, err := Path(name)
		if err == nil {
			t.Errorf("Path(%q) succeeded, want error", name)
		}
	}
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

func mustNew(t *testing.T, name string) *Identity {
	t.Helper()
	id, err := New(name)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
