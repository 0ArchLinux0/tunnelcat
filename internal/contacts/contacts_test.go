// Tests for the contacts package.
package contacts

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func withTempHome(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TUNNELCAT_CONFIG_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

func TestLoadMissingFile(t *testing.T) {
	withTempHome(t)
	f, err := Load()
	if err != nil {
		t.Fatalf("Load of missing file: %v", err)
	}
	if f == nil {
		t.Fatal("Load returned nil for missing file")
	}
	if len(f.Contacts) != 0 {
		t.Errorf("Load of missing file: contacts = %v, want empty", f.Contacts)
	}
}

func TestAddAndLoad(t *testing.T) {
	withTempHome(t)
	c := Contact{Name: "alpha", Pubkey: "nodekey:abc"}
	if err := Add(c); err != nil {
		t.Fatal(err)
	}
	f, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Contacts) != 1 {
		t.Fatalf("Load after Add: got %d contacts, want 1", len(f.Contacts))
	}
	if f.Contacts[0].Name != "alpha" {
		t.Errorf("contact name = %q, want %q", f.Contacts[0].Name, "alpha")
	}
	if f.Contacts[0].AddedAt.IsZero() {
		t.Error("AddedAt is zero, want auto-set to now")
	}
}

func TestAddDuplicate(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "dup", Pubkey: "nodekey:abc"})
	err := Add(Contact{Name: "dup", Pubkey: "nodekey:def"})
	if err == nil {
		t.Error("Add of duplicate name: got no error, want error")
	}
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should say 'already exists', got: %v", err)
	}
}

func TestAddRequiresNameAndPubkey(t *testing.T) {
	withTempHome(t)
	if err := Add(Contact{Name: "", Pubkey: "nodekey:abc"}); err == nil {
		t.Error("Add with empty name: got no error, want error")
	}
	if err := Add(Contact{Name: "x", Pubkey: ""}); err == nil {
		t.Error("Add with empty pubkey: got no error, want error")
	}
}

func TestUpdate(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "upd", Pubkey: "nodekey:abc"})
	// Update with new pubkey.
	if err := Update(Contact{Name: "upd", Pubkey: "nodekey:def"}); err != nil {
		t.Fatal(err)
	}
	c, ok := Find("upd")
	if !ok {
		t.Fatal("Find after Update: not found")
	}
	if c.Pubkey != "nodekey:def" {
		t.Errorf("Update didn't change pubkey: got %q", c.Pubkey)
	}
}

func TestUpdateNotFound(t *testing.T) {
	withTempHome(t)
	if err := Update(Contact{Name: "nonexistent", Pubkey: "nodekey:abc"}); err == nil {
		t.Error("Update of non-existent: got no error, want error")
	}
}

func TestRemove(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "rm", Pubkey: "nodekey:abc"})
	if err := Remove("rm"); err != nil {
		t.Fatal(err)
	}
	if _, ok := Find("rm"); ok {
		t.Error("Find after Remove: still found")
	}
}

func TestRemoveMissing(t *testing.T) {
	withTempHome(t)
	// Remove of non-existent should be idempotent.
	if err := Remove("nonexistent"); err != nil {
		t.Errorf("Remove of non-existent: got error %v, want nil", err)
	}
}

func TestFind(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "f", Pubkey: "nodekey:abc"})
	Add(Contact{Name: "g", Pubkey: "nodekey:def"})
	c, ok := Find("f")
	if !ok {
		t.Fatal("Find f: not found")
	}
	if c.Pubkey != "nodekey:abc" {
		t.Errorf("Find f: pubkey = %q, want %q", c.Pubkey, "nodekey:abc")
	}
	_, ok = Find("nonexistent")
	if ok {
		t.Error("Find nonexistent: ok = true, want false")
	}
}

func TestList(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "a", Pubkey: "nodekey:1"})
	Add(Contact{Name: "b", Pubkey: "nodekey:2"})
	Add(Contact{Name: "c", Pubkey: "nodekey:3"})
	list, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 {
		t.Errorf("List: got %d, want 3", len(list))
	}
	names := make([]string, len(list))
	for i, c := range list {
		names[i] = c.Name
	}
	want := []string{"a", "b", "c"}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("List[%d] = %q, want %q", i, names[i], n)
		}
	}
}

func TestFileFormat(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "fmt", Pubkey: "nodekey:abc", Note: "test"})
	data, _ := os.ReadFile(filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "contacts.yaml"))
	if !strings.Contains(string(data), "version: 1") {
		t.Errorf("file should contain 'version: 1', got:\n%s", data)
	}
	if !strings.Contains(string(data), "nodekey:abc") {
		t.Errorf("file should contain the pubkey, got:\n%s", data)
	}
}

func TestFileHas0600Mode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod is a no-op on Windows")
	}
	withTempHome(t)
	Add(Contact{Name: "perm", Pubkey: "nodekey:abc"})
	path := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "contacts.yaml")
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("file mode = %o, want 0600", info.Mode().Perm())
	}
}

func TestRejectBadVersion(t *testing.T) {
	withTempHome(t)
	path := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "contacts.yaml")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("version: 999\ncontacts: []\n"), 0600)
	_, err := Load()
	if err == nil {
		t.Error("Load with future version: got no error, want error")
	}
}

func TestRejectCorruptYAML(t *testing.T) {
	withTempHome(t)
	path := filepath.Join(os.Getenv("TUNNELCAT_CONFIG_DIR"), "contacts.yaml")
	os.MkdirAll(filepath.Dir(path), 0700)
	os.WriteFile(path, []byte("not: valid: yaml: ["), 0600)
	_, err := Load()
	if err == nil {
		t.Error("Load with corrupt YAML: got no error, want error")
	}
}

func TestConcurrentAdd(t *testing.T) {
	withTempHome(t)
	const N = 20
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			Add(Contact{
				Name:   "c" + string(rune('0'+i)),
				Pubkey: "nodekey:abc",
			})
		}(i)
	}
	wg.Wait()
	list, _ := List()
	if len(list) != N {
		t.Errorf("List after %d concurrent Adds: got %d contacts, want %d", N, len(list), N)
	}
}

func TestSaveAtomicNoLeftover(t *testing.T) {
	withTempHome(t)
	Add(Contact{Name: "atom", Pubkey: "nodekey:abc"})
	dir := os.Getenv("TUNNELCAT_CONFIG_DIR")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".contacts-") {
			t.Errorf("temp file %s left behind", e.Name())
		}
	}
}

func TestAddedAtPreserved(t *testing.T) {
	withTempHome(t)
	original := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	Add(Contact{Name: "pres", Pubkey: "nodekey:abc", AddedAt: original})
	// Reload and update pubkey.
	Update(Contact{Name: "pres", Pubkey: "nodekey:def"})
	c, _ := Find("pres")
	if !c.AddedAt.Equal(original) {
		t.Errorf("AddedAt changed on Update: was %v, now %v", original, c.AddedAt)
	}
}
