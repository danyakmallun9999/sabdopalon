package app

import (
	"os"
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
//
// The rule is "is this location durable?", not "is this the default location?".
// The older rule accepted the override only when it equalled
// %USERPROFILE%\sabdopalon\bin, so an install anywhere else - C:\sabdopalon,
// another drive, a portable extract - got no PATH entry at all and shipped
// php/composer/mysql that worked only inside Sabdopalon's own terminal.
func TestShouldWirePersistentPathOnlyForDurableLocations(t *testing.T) {
	// A scratch folder under %TEMP%: dev run, test harness, relocated extraction.
	if shouldWirePersistentPath(filepath.Join(t.TempDir(), "bin")) {
		t.Error("a scratch bin dir must not be written into the persistent PATH")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory to build a durable path from")
	}

	// The default install location.
	if !shouldWirePersistentPath(sysinstall.UserBinDir()) {
		t.Error("the default per-user bin dir must keep its PATH entry")
	}

	// An install that is real but NOT the default location. This is the case
	// that used to be skipped, and it is the common one on Windows: people
	// install to C:\sabdopalon.
	other := filepath.Join(home, "another-place", "sabdopalon", "bin")
	if !shouldWirePersistentPath(other) {
		t.Errorf("a durable install root outside the default location (%s) must be wired", other)
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
