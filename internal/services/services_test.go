package services

import (
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// TestSweepGhostsKillsOrphanOnOurPort verifies that an orphaned service
// process from an earlier session (PPID=1, Go parent already dead) that
// still holds one of our registered ports is killed on startup, while a
// foreign instance of the same binary on a different port is left alone.
func TestSweepGhostsKillsOrphanOnOurPort(t *testing.T) {
	cfg := &config.Engine{}
	m := New(cfg)

	ghostArgs := "/home/u/Sabdopalon/bin/mailpit/mailpit --smtp 127.0.0.1:1025 --listen 127.0.0.1:8025"
	foreignArgs := "/usr/local/bin/mailpit --smtp 127.0.0.1:2525 --listen 127.0.0.1:8030"

	oldPT := processTable
	processTable = func(yield func(int, string) bool) {
		_ = yield(111, foreignArgs)
		_ = yield(222, ghostArgs)
	}
	t.Cleanup(func() { processTable = oldPT })

	killed := map[int]bool{}
	oldKPT := killProcessTree
	killProcessTree = func(pid int) { killed[pid] = true }
	t.Cleanup(func() { killProcessTree = oldKPT })

	oldPA := processAlive
	processAlive = func(pid int) bool { return pid == 222 }
	t.Cleanup(func() { processAlive = oldPA })

	m.SweepGhosts()

	if !killed[222] {
		t.Errorf("expected ghost mailpit (pid 222) on our port 1025 to be killed, got %v", killed)
	}
	if killed[111] {
		t.Errorf("foreign mailpit (pid 111) on port 2525 must NOT be killed, but it was")
	}
}

// TestSweepGhostsIgnoresUnrelatedBinaries verifies processes whose command
// line doesn't match any registered service binary are never touched.
func TestSweepGhostsIgnoresUnrelatedBinaries(t *testing.T) {
	cfg := &config.Engine{}
	m := New(cfg)

	oldPT := processTable
	processTable = func(yield func(int, string) bool) {
		_ = yield(333, "/usr/bin/redis-server --port 6379 --dir /tmp") // redis IS registered
		_ = yield(444, "/usr/bin/nginx --listen 9000")                 // not a service we manage
	}
	t.Cleanup(func() { processTable = oldPT })

	killed := map[int]bool{}
	oldKPT := killProcessTree
	killProcessTree = func(pid int) { killed[pid] = true }
	t.Cleanup(func() { killProcessTree = oldKPT })

	oldPA := processAlive
	processAlive = func(pid int) bool { return true }
	t.Cleanup(func() { processAlive = oldPA })

	m.SweepGhosts()

	// redis on port 6379 IS a registered service — should be swept.
	if !killed[333] {
		t.Errorf("expected ghost redis (pid 333) on our port 6379 to be killed, got %v", killed)
	}
	// nginx is not a registered service — must not be touched.
	if killed[444] {
		t.Errorf("nginx (pid 444) is not a managed service and must NOT be killed")
	}
}

// TestCmdRunsBin covers basename matching for the executable path.
func TestCmdRunsBin(t *testing.T) {
	cases := []struct {
		args     string
		binNames []string
		want     bool
	}{
		{"/home/u/Sabdopalon/bin/mailpit/mailpit --smtp 127.0.0.1:1025", []string{"mailpit"}, true},
		{"/usr/bin/redis-server --port 6379", []string{"redis-server"}, true},
		{"/usr/bin/nginx --listen 80", []string{"mailpit"}, false},
		{"", []string{"mailpit"}, false},
		{"/some/path/minio server /data", []string{"minio"}, true},
	}
	for _, c := range cases {
		if got := cmdRunsBin(c.args, c.binNames); got != c.want {
			t.Errorf("cmdRunsBin(%q, %v) = %v, want %v", c.args, c.binNames, got, c.want)
		}
	}
}

// TestCmdMentionsPort covers port-token matching in command lines.
func TestCmdMentionsPort(t *testing.T) {
	tokens := []string{":1025", " 1025 ", " 1025"}
	cases := []struct {
		args string
		want bool
	}{
		{"mailpit --smtp 127.0.0.1:1025 --listen 127.0.0.1:8025", true},
		{"mailpit --smtp 127.0.0.1:2525 --listen 127.0.0.1:8030", false},
		{"redis-server --port 6379", false},
		{"some --port 1025 --other", true},
	}
	for _, c := range cases {
		if got := cmdMentionsPort(c.args, tokens); got != c.want {
			t.Errorf("cmdMentionsPort(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}
