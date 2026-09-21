//go:build windows

package terminal

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestConPTYProducesOutput is the regression guard for a silent, total
// failure of the embedded terminal on Windows.
//
// PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE must be given the HPCON VALUE. Feeding
// CreateProcess a pointer to the handle instead does not error: the child
// starts, runs and exits 0, and the dashboard's WebSocket upgrades happily —
// but nothing the shell writes ever reaches outPipe, so the terminal is
// simply blank. Nothing on that path is visible to the type system or to a
// compile check, which is why it survived until it was probed directly.
//
// Asserting on real bytes is the only thing that catches a regression here.
func TestConPTYProducesOutput(t *testing.T) {
	comspec := os.Getenv("ComSpec")
	if comspec == "" {
		comspec = `C:\Windows\System32\cmd.exe`
	}
	if _, err := os.Stat(comspec); err != nil {
		t.Skipf("cmd.exe not available: %v", err)
	}

	const marker = "SABDOPALON-CONPTY-PROBE"

	c, err := newConPTY()
	if err != nil {
		t.Fatalf("newConPTY: %v", err)
	}
	defer c.close()

	if err := c.startProcess(comspec, []string{"/c", "echo " + marker}, "", os.Environ()); err != nil {
		t.Fatalf("startProcess: %v", err)
	}

	// Output arrives in arbitrary chunks interleaved with ConPTY control
	// sequences, so accumulate rather than matching one read. Each read runs
	// in its own goroutine (Read blocks) and hands the chunk back over a
	// channel — the builder is never touched concurrently.
	var got []byte
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(string(got), marker) {
		type chunk struct {
			data []byte
			err  error
		}
		ch := make(chan chunk, 1)
		go func() {
			buf := make([]byte, 4096)
			n, err := c.outPipe.Read(buf)
			ch <- chunk{append([]byte(nil), buf[:n]...), err}
		}()
		select {
		case r := <-ch:
			got = append(got, r.data...)
			if r.err != nil {
				deadline = time.Now() // stop early on EOF
			}
		case <-time.After(3 * time.Second):
		}
	}

	if !strings.Contains(string(got), marker) {
		t.Fatalf("child produced no usable output through the ConPTY "+
			"(read %d bytes: %q) — the embedded terminal is dead; check that "+
			"PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE receives the HPCON value, "+
			"not a pointer to it", len(got), string(got))
	}
}
