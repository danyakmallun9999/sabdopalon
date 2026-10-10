package app

import (
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/services"
	"github.com/sabdopalon/sabdopalon/internal/sysinstall"
)

func TestWaitGroupTimeout(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		time.Sleep(20 * time.Millisecond)
		wg.Done()
	}()
	if !waitGroupTimeout(&wg, 5*time.Second) {
		t.Error("expected success once the group finished")
	}

	var blocked sync.WaitGroup
	blocked.Add(1) // never Done
	start := time.Now()
	if waitGroupTimeout(&blocked, 100*time.Millisecond) {
		t.Error("expected a timeout when the group never finishes")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("timeout waited %v, want ~100ms — shutdown must not stall on it", elapsed)
	}
}

// A stop that arrives before the daemons are started must end the work
// immediately. Passing nil managers is the point: touching either one would
// panic, so returning cleanly proves the flag is checked BEFORE any start.
func TestStartDaemonsHonoursStopBeforeStarting(t *testing.T) {
	a := &App{Cfg: &config.Engine{}}
	var stopped atomic.Bool
	stopped.Store(true)

	done := make(chan struct{})
	go func() {
		a.startDaemons(nil, nil, &stopped)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("startDaemons did not return with the stop flag set")
	}
}

// A transient SABDOPALON_BIN_DIR must never be written to the user's persistent
// PATH. It outlives the run that set it: a bind-time measurement with the
// override pointing at a temp root left that temp folder in the real user PATH,
// ahead of the real install, until it was removed by hand.
func TestShouldWirePersistentPathRejectsTransientOverride(t *testing.T) {
	// Dev run / test harness / relocated extraction: not the stable folder.
	t.Setenv("SABDOPALON_BIN_DIR", filepath.Join(t.TempDir(), "bin"))
	if shouldWirePersistentPath() {
		t.Error("a transient bin-dir override must not be wired into the persistent PATH")
	}

	// The CLI never sets the override, and its <root>/bin is a real location.
	t.Setenv("SABDOPALON_BIN_DIR", "")
	if !shouldWirePersistentPath() {
		t.Error("without an override the install's own bin/ must keep its PATH entry")
	}

	// The desktop shell passes exactly the stable per-user folder.
	t.Setenv("SABDOPALON_BIN_DIR", sysinstall.UserBinDir())
	if !shouldWirePersistentPath() {
		t.Error("the desktop layout (override == per-user bin) must keep its PATH entry")
	}
}

// startDaemons must always come back. It used to sit in a 30s TCP probe (plus
// 10s of pidfile waiting) per enabled engine while the dashboard was not yet
// listening, which is exactly what left the setup screen frozen at 95%.
func TestStartDaemonsReturnsWithMissingEngines(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root}
	cfg.Root = filepath.Join(root, "sites")
	cfg.Data = filepath.Join(root, "data")
	cfg.Logs = filepath.Join(root, "logs")
	cfg.Database.Engine = "mariadb"
	cfg.Database.MariaDBEnabled = true
	cfg.Database.MariaDBPort = 3306
	cfg.Database.PGEnabled = true
	cfg.Database.PGPort = 5433

	a := &App{Cfg: cfg}
	var stopped atomic.Bool

	done := make(chan struct{})
	go func() {
		a.startDaemons(database.New(cfg), services.New(cfg), &stopped)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("startDaemons blocked: an engine probe is holding the caller")
	}
}
