//go:build windows

// Package proxy — orphans_windows.go: clean up after a Sabdopalon that was
// killed rather than stopped.
//
// Every site runs php -S children. A clean exit stops them (StopAll), but a
// killed process — an installer replacing the app does exactly that, and the
// desktop shell hides to the tray rather than quitting — leaves them running.
// They are invisible in the app, keep their ports and keep holding their memory:
// one machine had 24 workers and ~1.9 GB of PHP left behind, with 3.4 GB free,
// which is precisely the memory pressure that makes a worker die and a page come
// out blank.
//
// Measured on that machine: the workers are started through the bin/php.cmd
// shim, so php.exe's parent is a cmd.exe that dies with Sabdopalon — the parent
// check below is what actually identifies an orphan there.
package proxy

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// ReapOrphanWorkers kills `php -S 127.0.0.1:<port>` processes that serve a site
// under root and whose parent process is gone, and returns how many it stopped.
//
// Three things keep this from being a blunt "kill every php.exe":
//
//   - the command line must contain "-S 127.0.0.1:", which is what our workers
//     are — `artisan queue:listen`, `artisan migrate`, or a `php -S` the user
//     started elsewhere never match it;
//   - the working directory named on that command line must be under root, so
//     another tool's server is not touched;
//   - the parent process must no longer exist. A worker of a *running*
//     Sabdopalon always has one, so a second instance never steals its workers.
//
// PowerShell is used because the command line of another process is the only
// thing that separates these processes — they are all the same php.exe — and
// Windows exposes it through WMI, not through the Go standard library. The count
// comes back through a file rather than stdout: capturing a child's output
// through a pipe is exactly the kind of stdio confinement that can fail on a
// locked-down host, and a memory cleanup must not depend on being allowed to
// pipe.
func ReapOrphanWorkers(root string) int {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0
	}
	dir, err := os.MkdirTemp("", "sab-reap")
	if err != nil {
		return 0
	}
	defer os.RemoveAll(dir)
	outFile := filepath.Join(dir, "killed.txt")
	scriptFile := filepath.Join(dir, "reap.ps1")

	script := `$ErrorActionPreference = 'SilentlyContinue'
$out = $env:SABDOPALON_REAP_OUT
$root = $env:SABDOPALON_REAP_ROOT
$me = $PID
$killed = 0
Get-CimInstance Win32_Process -Filter "Name='php.exe'" | Where-Object {
    $_.ParentProcessId -ne $me -and
    -not (Get-Process -Id $_.ParentProcessId) -and
    $_.CommandLine -and
    $_.CommandLine -match '-S\s+127\.0\.0\.1:\d+' -and
    $_.CommandLine.ToLower().Contains($root.ToLower())
} | ForEach-Object {
    Stop-Process -Id $_.ProcessId -Force
    $killed++
}
Set-Content -Path $out -Value $killed
`
	if err := os.WriteFile(scriptFile, []byte(script), 0o600); err != nil {
		return 0
	}
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", scriptFile)
	cmd.Env = append(os.Environ(),
		"SABDOPALON_REAP_ROOT="+root,
		"SABDOPALON_REAP_OUT="+outFile,
	)
	winproc.Quiet(cmd)
	// A missing PowerShell or a WMI hiccup must never stop the app from
	// starting: the leak is what is being fixed here, not a hard dependency.
	_ = cmd.Run()

	data, err := os.ReadFile(outFile)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || n < 0 {
		return 0
	}
	return n
}
