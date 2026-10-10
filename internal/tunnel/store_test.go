package tunnel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStoreRemembersTheEmailList(t *testing.T) {
	dir := t.TempDir()
	s := loadStore(dir)
	if got := s.get("myapp").AllowedMail; len(got) != 0 {
		t.Fatalf("fresh store returned %v", got)
	}
	if err := s.set("myapp", Prefs{AllowedMail: []string{"a@b.com"}}); err != nil {
		t.Fatal(err)
	}

	// A second load must see it: the point of remembering is not retyping the
	// list on every share.
	again := loadStore(dir)
	if got := again.get("myapp").AllowedMail; len(got) != 1 || got[0] != "a@b.com" {
		t.Fatalf("reloaded store = %v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "config", "tunnels.json")); err != nil {
		t.Fatalf("store file missing: %v", err)
	}

	// An empty list drops the entry instead of leaving a stray site name behind.
	if err := again.set("myapp", Prefs{}); err != nil {
		t.Fatal(err)
	}
	if got := loadStore(dir).get("myapp").AllowedMail; len(got) != 0 {
		t.Errorf("empty prefs were kept: %v", got)
	}
}

func TestStoreToleratesACorruptFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config", "tunnels.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A corrupt file must not stop the app from starting; it only means nothing
	// is remembered.
	if got := loadStore(dir).get("myapp").AllowedMail; len(got) != 0 {
		t.Fatalf("corrupt store returned %v", got)
	}
}
