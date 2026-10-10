//go:build !windows

// Package proxy — orphans_unix.go: clean up after a Sabdopalon that was killed
// rather than stopped. See orphans_windows.go for why this matters.
package proxy

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ReapOrphanWorkers kills `php -S 127.0.0.1:<port>` processes that serve a site
// under root and whose parent is gone (PID 1 — on Unix an orphan is reparented
// to init), and returns how many it stopped.
//
// The command line is what separates our workers from an `artisan queue:listen`
// the user started on purpose, and the parent check keeps the workers of a
// still-running Sabdopalon out of it.
func ReapOrphanWorkers(root string) int {
	root = strings.TrimSpace(root)
	if root == "" {
		return 0
	}
	out, err := exec.Command("ps", "-ax", "-o", "pid=,ppid=,args=").Output()
	if err != nil {
		return 0
	}
	killed := 0
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil || ppid != 1 { // still owned by a live parent
			continue
		}
		args := strings.Join(fields[2:], " ")
		if !strings.Contains(args, "php") || !strings.Contains(args, "-S 127.0.0.1:") {
			continue
		}
		if !strings.Contains(args, root) {
			continue
		}
		if p, err := os.FindProcess(pid); err == nil {
			if err := p.Kill(); err == nil {
				killed++
			}
		}
	}
	return killed
}
