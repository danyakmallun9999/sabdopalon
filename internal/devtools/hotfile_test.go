package devtools

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// A dev server that has stopped leaves public/hot behind. Laravel keeps pointing
// every asset at the address in that file, so the page returns its HTML and the
// browser gets no JavaScript: a blank page that reads as "Sabdopalon broke my
// Laravel app" — measured on a fresh Laravel 13 project whose assets all pointed
// at http://[::1]:5173 with nothing listening there.
func TestClearStaleHotFile(t *testing.T) {
	dir := t.TempDir()
	publicDir := filepath.Join(dir, "myapp", "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hot := filepath.Join(publicDir, "hot")

	m := New(&config.Engine{Root: dir})

	// Naming a port nothing listens on → stale, must go.
	dead := freePort(t)
	if err := os.WriteFile(hot, []byte("http://[::1]:"+strconv.Itoa(dead)), 0o644); err != nil {
		t.Fatal(err)
	}
	if !m.ClearStaleHotFile("myapp") {
		t.Fatal("a hot file pointing at a dead port must be cleared")
	}
	if _, err := os.Stat(hot); err == nil {
		t.Error("the stale hot file is still there")
	}

	// Naming a live dev server → left alone, because the user is working.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	live := l.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(hot, []byte("http://localhost:"+strconv.Itoa(live)+"/"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m.ClearStaleHotFile("myapp") {
		t.Error("a hot file pointing at a live dev server must be kept")
	}
	if _, err := os.Stat(hot); err != nil {
		t.Error("the live hot file was removed")
	}

	// A site without a hot file is a no-op.
	if m.ClearStaleHotFile("nosuchsite") {
		t.Error("clearing a site with no hot file reported work")
	}

	// The sweep covers every site and counts what it cleared.
	if err := os.WriteFile(hot, []byte("http://[::1]:"+strconv.Itoa(dead)), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other", "public")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "hot"), []byte("http://[::1]:"+strconv.Itoa(dead)), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := m.CleanStaleHotFiles(); got != 2 {
		t.Errorf("CleanStaleHotFiles = %d, want 2", got)
	}
	if got := m.CleanStaleHotFiles(); got != 0 {
		t.Errorf("second sweep = %d, want 0 (nothing left to clear)", got)
	}
}

func TestPortFromHotURL(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{
		{"http://[::1]:5173", 5173},
		{"http://localhost:5173/", 5173},
		{"http://127.0.0.1:5180", 5180},
		{"https://myapp.localhost", 443},
		{"http://myapp.localhost", 80},
		{"", 0},
		{"not a url", 0},
	} {
		if got := portFromHotURL(tc.raw); got != tc.want {
			t.Errorf("portFromHotURL(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
}
