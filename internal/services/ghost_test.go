package services

import (
	"strings"
	"testing"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// A quoted image path must still be recognised as the service binary.
//
// Command lines came straight from Win32_Process.CommandLine, where Windows
// quotes the image path as soon as it contains a space. Splitting on the first
// space left the closing quote glued to the basename ("mailpit.exe"), so the
// ".exe" strip missed and the comparison failed — the ghost survived, kept its
// port, and the service could not start again until the machine rebooted.
func TestCmdRunsBinHandlesQuotedImagePath(t *testing.T) {
	mailpit := []string{"mailpit", "mailpit.exe"}

	cases := []struct {
		name string
		args string
		want bool
	}{
		{"unquoted windows path",
			`C:\Sabdopalon\bin\mailpit\mailpit.exe --smtp 127.0.0.1:1025`, true},
		{"quoted path with spaces",
			`"C:\Program Files\Sabdopalon\bin\mailpit\mailpit.exe" --smtp 127.0.0.1:1025`, true},
		{"quoted path, no args",
			`"C:\Sabdopalon\bin\mailpit\mailpit.exe"`, true},
		{"quoted, extension omitted",
			`"C:\Sabdopalon\bin\mailpit\mailpit"`, true},
		{"unix path", `/home/u/.sabdopalon/bin/mailpit/mailpit --smtp 127.0.0.1:1025`, true},
		{"leading whitespace", `   "C:\Program Files\m\mailpit.exe" --smtp x`, true},

		{"different binary", `"C:\Program Files\Other\other.exe" --smtp 127.0.0.1:1025`, false},
		{"binary name as an argument only", `"C:\Other\run.exe" --name mailpit`, false},
		{"empty", ``, false},
		{"whitespace only", `   `, false},
	}
	for _, tc := range cases {
		if got := cmdRunsBin(tc.args, mailpit); got != tc.want {
			t.Errorf("%s: cmdRunsBin(%q) = %v, want %v", tc.name, tc.args, got, tc.want)
		}
	}
}

// firstCommandField is the piece that had to learn about quoting; pin its
// behaviour directly so a future "simplification" back to a space split fails
// loudly here rather than silently reintroducing unsweepable ghosts.
func TestFirstCommandField(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"C:\Program Files\a b\x.exe" --flag`, `C:\Program Files\a b\x.exe`},
		{`C:\bin\x.exe --flag`, `C:\bin\x.exe`},
		{`C:\bin\x.exe`, `C:\bin\x.exe`},
		{`""`, ``},
		{`"unterminated`, `unterminated`},
		{"\t/sbin/init", `/sbin/init`},
		{``, ``},
	}
	for _, tc := range cases {
		if got := firstCommandField(tc.in); got != tc.want {
			t.Errorf("firstCommandField(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The Windows-path cases above MUST pass on every host. filepath.Base treats
// "\" as a separator only on Windows, so using it here made them fail on Linux
// and macOS (Windows kept passing and hid it) — a red `go test -race` on two of
// three CI runners for a bug in the sweep. Pin the host-independent split
// directly so a future "just use filepath.Base" change fails on any platform.
func TestBinBasenameSplitsBothSeparators(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\Sabdopalon\bin\mailpit\mailpit.exe`, "mailpit.exe"},
		{`C:/Sabdopalon/bin/mailpit/mailpit.exe`, "mailpit.exe"},
		{`/home/u/.sabdopalon/bin/mailpit/mailpit`, "mailpit"},
		{`C:\Program Files\a b\x.exe`, "x.exe"},
		{`mailpit.exe`, "mailpit.exe"},
		{`mixed\and/slashes.exe`, "slashes.exe"},
		{``, ``},
	}
	for _, tc := range cases {
		if got := binBasename(tc.in); got != tc.want {
			t.Errorf("binBasename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// End-to-end: the sweep must actually collect and kill the quoted ghost, and
// must leave a same-binary process on a DIFFERENT port alone (a user's own
// mailpit is not ours to kill).
func TestSweepGhostsCollectsQuotedAndSparesForeignPort(t *testing.T) {
	origTable, origKill, origAlive := processTable, killProcessTree, processAlive
	origKillTree := killProcessTree
	defer func() {
		processTable, killProcessTree, processAlive = origTable, origKill, origAlive
		_ = origKillTree
	}()

	table := []struct {
		pid  int
		args string
	}{
		{101, `"C:\Program Files\Sabdopalon\bin\mailpit\mailpit.exe" --smtp 127.0.0.1:1025 --listen 127.0.0.1:8025`},
		{202, `C:\Sabdopalon\bin\redis\redis-server.exe --port 6379`},
		{303, `"C:\Sabdopalon\bin\redis\redis-server.exe" --port 6379`},
		{404, `"C:\Elsewhere\mailpit.exe" --smtp 127.0.0.1:2525`}, // foreign port
		{505, `C:\Windows\System32\notepad.exe`},
	}
	processTable = func(yield func(pid int, args string) bool) {
		for _, e := range table {
			if !yield(e.pid, e.args) {
				return
			}
		}
	}
	var killed []int
	killProcessTree = func(pid int) { killed = append(killed, pid) }
	processAlive = func(pid int) bool { return pid > 0 }

	m := New(&config.Engine{RootDir: t.TempDir()})
	m.SweepGhosts()

	got := map[int]bool{}
	for _, pid := range killed {
		got[pid] = true
	}
	for _, pid := range []int{101, 202, 303} {
		if !got[pid] {
			t.Errorf("pid %d (an orphan on one of our ports) was not swept", pid)
		}
	}
	for _, pid := range []int{404, 505} {
		if got[pid] {
			t.Errorf("pid %d was killed but is not ours:\n  %s", pid, strings.TrimSpace(table[pid-101].args))
		}
	}
}
