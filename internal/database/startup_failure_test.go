package database

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// MariaDB and MySQL write their startup errors to <datadir>/<host>.err rather
// than to stdout, so the captured log is legitimately empty in exactly the case
// a user needs it most. "nothing on stdout" used to be the whole message, which
// pointed at the wrong file: a fresh Windows install reported it while the real
// error sat unread in the data directory.
func TestStartupFailureReadsTheEnginesOwnErrorLog(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "fresh-laptop.err"),
		[]byte("[ERROR] InnoDB: Unable to lock ./ibdata1, error: 33\n[ERROR] Aborting\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "mariadb.log") // never written — the old dead end

	done := make(chan struct{})
	close(done) // the daemon already exited

	m := New(&config.Engine{})
	err := m.startupFailure("mariadb", dataDir, logPath, exec.Command("mariadb"), done, 30*time.Second, true)
	msg := err.Error()

	for _, want := range []string{
		"fresh-laptop.err",     // the log that actually holds the reason
		"Unable to lock",       // ...and its content
		"mariadb.log is empty", // said out loud, instead of pretending there is nothing
		"the daemon exited",    // distinguished from "still running"
		"FIRST start",          // the longer budget only applies there
		"mariadb did not start within 30s",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message should mention %q; got:\n%s", want, msg)
		}
	}
	if strings.Contains(msg, "no output captured") {
		t.Errorf("the dead-end wording must be gone; got:\n%s", msg)
	}
}

// An interrupted first initialization must not brick the install: dirHasFiles
// would otherwise skip initialization forever and the daemon could never start.
// The system schema is what separates "half-initialized" from "real database".
func TestNeedsReinit(t *testing.T) {
	partial := t.TempDir()
	if err := os.WriteFile(filepath.Join(partial, "aria_log_control"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !needsReinit("mariadb", partial) {
		t.Error("a non-empty data dir with no mysql/ schema is half-initialized and must be rebuilt")
	}
	if !needsReinit("postgresql", partial) {
		t.Error("a non-empty cluster with no base/ dir is half-initialized and must be rebuilt")
	}

	real := t.TempDir()
	if err := os.MkdirAll(filepath.Join(real, "mysql"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "mysql", "user.frm"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if needsReinit("mariadb", real) {
		t.Error("a data dir WITH the system schema is a real database — never rebuild it")
	}

	marked := t.TempDir()
	if err := os.WriteFile(filepath.Join(marked, initMarkerName), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if needsReinit("mariadb", marked) {
		t.Error("a data dir we already initialized must be left alone")
	}
	if needsReinit("sqlite", t.TempDir()) {
		t.Error("sqlite has no data dir to re-initialize")
	}
}

// A daemon still running gets said so, because that is a different problem
// (slow first start) from a daemon that died.
func TestStartupFailureSaysWhenTheDaemonWasStillRunning(t *testing.T) {
	m := New(&config.Engine{})
	err := m.startupFailure("postgresql", t.TempDir(), filepath.Join(t.TempDir(), "postgresql.log"),
		exec.Command("postgres"), make(chan struct{}), 3*time.Minute, false)
	msg := err.Error()
	if !strings.Contains(msg, "still running and has been stopped") {
		t.Errorf("want the still-running wording, got:\n%s", msg)
	}
	if strings.Contains(msg, "FIRST start") {
		t.Errorf("the first-start note must only appear for a first start, got:\n%s", msg)
	}
	if !strings.Contains(msg, "3m0s") {
		t.Errorf("the timeout in the message should be the one that applied, got:\n%s", msg)
	}
}

func TestNewestErrTailPicksTheMostRecentLog(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.err")
	newer := filepath.Join(dir, "newer.err")
	if err := os.WriteFile(old, []byte("old failure\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newer, []byte("recent failure\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Unrelated files must not be mistaken for the engine's error log.
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}

	gotPath, gotTail := newestErrTail(dir)
	if filepath.Base(gotPath) != "newer.err" {
		t.Errorf("newestErrTail picked %q, want newer.err", filepath.Base(gotPath))
	}
	if !strings.Contains(gotTail, "recent failure") {
		t.Errorf("tail = %q, want the newest log's content", gotTail)
	}

	// No error log at all is a different, still-useful answer.
	if p, tail := newestErrTail(t.TempDir()); p != "" || tail != "" {
		t.Errorf("empty dir should yield nothing, got (%q, %q)", p, tail)
	}
}
