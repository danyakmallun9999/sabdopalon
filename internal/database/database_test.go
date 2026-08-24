package database

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestHelperProcess doubles as a cross-platform sleeper child for liveness
// tests: run with DB_TEST_SLEEPER=1 it just sleeps until killed.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("DB_TEST_SLEEPER") != "1" {
		t.Skip("helper only runs as a spawned child")
	}
	time.Sleep(30 * time.Second)
}

func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess$", "-test.v=false")
	cmd.Env = append(os.Environ(), "DB_TEST_SLEEPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleeper: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	return cmd
}

func TestPortBusyProbe(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if !portBusy(port) {
		t.Errorf("port %d should read busy while a listener holds it", port)
	}
	ln.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !portBusy(port) {
			return
		}
		time.Sleep(50 * time.Millisecond) // kernel may hold the socket briefly
	}
	t.Errorf("port %d still reads busy after the listener closed", port)
}

func TestReadPidFile(t *testing.T) {
	dir := t.TempDir()

	single := filepath.Join(dir, "pid")
	if err := os.WriteFile(single, []byte("4242\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := readPidFile(single); !ok || got != 4242 {
		t.Errorf("single-line pidfile: got %d ok=%v", got, ok)
	}

	multi := filepath.Join(dir, "postmaster.pid")
	if err := os.WriteFile(multi, []byte("777\n/home/data\n5433\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, ok := readPidFile(multi); !ok || got != 777 {
		t.Errorf("multi-line pidfile: got %d ok=%v", got, ok)
	}

	bad := filepath.Join(dir, "bad")
	if err := os.WriteFile(bad, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readPidFile(bad); ok {
		t.Error("garbage pidfile must not parse")
	}
	if _, ok := readPidFile(filepath.Join(dir, "missing")); ok {
		t.Error("missing pidfile must not parse")
	}
	// pid <= 0 is never valid
	zero := filepath.Join(dir, "zero")
	_ = os.WriteFile(zero, []byte("0"), 0o644)
	if _, ok := readPidFile(zero); ok {
		t.Error("pid 0 must be rejected")
	}
}

func TestProcessAliveAndMatches(t *testing.T) {
	me := os.Getpid()
	if !processAlive(me) {
		t.Fatal("our own pid must read alive")
	}
	if runtime.GOOS != "windows" && processAlive(1) == false { // init/systemd always exists on unix
		t.Log("warning: pid 1 not alive?!")
	}
	// A dead (reaped) pid must read dead — use our exited sleeper.
	sleep := startSleeper(t)
	pid := sleep.Process.Pid
	if !processAlive(pid) {
		t.Fatalf("sleeper pid %d should be alive", pid)
	}
	_ = sleep.Process.Kill()
	_, _ = sleep.Process.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for processAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if processAlive(pid) {
		t.Errorf("killed sleeper pid %d still reads alive", pid)
	}

	// Our own test binary does not look like mariadbd/postgres.
	for _, want := range []string{"mariadbd", "postgres"} {
		if processMatches(me, want) {
			t.Errorf("processMatches(%d, %q) must be false for the test binary", me, want)
		}
	}
}

func TestCheckPortOwnerForeignHoldsPort(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	// Empty data dir: nothing of ours can own this port → loud error.
	pid, perr := checkPortOwner("mariadb", filepath.Join(t.TempDir(), "mariadb"), port,
		func(int, string) bool { return false })
	if pid != 0 || perr == nil {
		t.Fatalf("expected foreign-port error, got pid=%d err=%v", pid, perr)
	}
	if want := fmt.Sprintf("port %d is already in use", port); !contains(perr.Error(), want) {
		t.Errorf("error %q missing %q", perr, want)
	}

	// Same port, but a fresh pidfile naming a live process that matches our
	// binary → adoption.
	dataDir := filepath.Join(t.TempDir(), "mariadb")
	_ = os.MkdirAll(dataDir, 0o755)
	sleep := startSleeper(t)
	if err := os.WriteFile(pidFilePath("mariadb", dataDir),
		[]byte(fmt.Sprintf("%d\n", sleep.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	pid, perr = checkPortOwner("mariadb", dataDir, port,
		func(int, string) bool { return true })
	if perr != nil || pid != sleep.Process.Pid {
		t.Fatalf("expected adoption of pid %d, got pid=%d err=%v", sleep.Process.Pid, pid, perr)
	}
}

func TestWaitOwnedPidStaleFileCleared(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mariadb")
	_ = os.MkdirAll(dir, 0o755)

	sleep := startSleeper(t) // stands in for a live foreign daemon
	stale := startSleeper(t)
	stalePid := stale.Process.Pid
	_ = stale.Process.Kill()
	_, _ = stale.Process.Wait() // pid now dead

	// Pidfile records the DEAD pid → must be treated as stale and cleared,
	// never reported as a foreign owner.
	path := pidFilePath("mariadb", dir)
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d\n", stalePid)), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := waitOwnedPid("mariadb", dir, sleep.Process.Pid, 1200*time.Millisecond)
	if ok {
		t.Fatalf("ownership cannot succeed here, got ok=true pid=%d", got)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stale pidfile should have been removed; stat err=%v", err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	content := "line one\n\nline two\n2026-08-24 [ERROR] Bind failed\ndone\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := logTail(path, 2)
	want := "2026-08-24 [ERROR] Bind failed | done"
	if got != want {
		t.Errorf("logTail = %q, want %q", got, want)
	}
	if logTail(filepath.Join(t.TempDir(), "nope.log"), 3) != "" {
		t.Error("missing log must yield empty tail")
	}
}
