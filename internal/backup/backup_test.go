package backup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// On Windows the bundled tools are mariadb-dump.exe / mysqldump.exe.
// findDumpBinary used to build extensionless candidates, so a perfectly
// healthy MariaDB bundle was reported as "dump binary not found" and MariaDB
// backup could never run there.
func TestFindDumpBinaryFindsBundledExecutable(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SABDOPALON_BIN_DIR", root)

	name := "mariadb-dump"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	want := filepath.Join(root, "mariadb", "bin", name)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := &Manager{cfg: &config.Engine{}}
	got := m.findDumpBinary("mariadb")
	if got != want {
		t.Fatalf("findDumpBinary = %q, want the bundled %q", got, want)
	}
	if runtime.GOOS == "windows" && !strings.HasSuffix(got, ".exe") {
		t.Fatalf("on Windows the bundled path must keep its .exe suffix, got %q", got)
	}
}
