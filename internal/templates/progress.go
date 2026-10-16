// Package templates — progress.go: where a template's messages go.
package templates

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Progress is the stream a template writes to while it builds a project: the
// terminal for `sabdopalon new`, the dashboard's create job for a site made from
// the UI, and nothing at all for a test.
//
// It exists because creating a Laravel project runs composer — and often npm —
// for minutes. Sending those lines to the process's own stdout meant the
// dashboard could only say "Creating…" and hope, with one HTTP request held open
// for the whole build and no way to tell progress from a hang.
//
// A nil *Progress discards everything, so callers never need to check for one.
type Progress struct {
	mu sync.Mutex
	w  io.Writer
}

// NewProgress returns a Progress writing to w. A nil writer discards.
func NewProgress(w io.Writer) *Progress {
	if w == nil {
		return &Progress{}
	}
	return &Progress{w: w}
}

// Discard is a Progress that keeps nothing.
func Discard() *Progress { return &Progress{} }

// Printf writes one progress line. A trailing newline is added when the caller
// did not supply one, so a log made of these lines stays readable.
func (p *Progress) Printf(format string, args ...any) {
	if p == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.w == nil {
		return
	}
	_, _ = io.WriteString(p.w, line)
}

// Out is the writer child processes should use, so their output appears as it
// arrives instead of after they exit.
func (p *Progress) Out() io.Writer {
	if p == nil || p.w == nil {
		return io.Discard
	}
	return lockedWriter{p}
}

// lockedWriter serialises writes from a child's stdout and stderr, which arrive
// on two goroutines.
type lockedWriter struct{ p *Progress }

func (l lockedWriter) Write(b []byte) (int, error) {
	l.p.mu.Lock()
	defer l.p.mu.Unlock()
	return l.p.w.Write(b)
}

// tailLimit bounds what is kept for an error message: a failing composer run can
// print megabytes, and the message only needs the end of it.
const tailLimit = 8 << 10

// tailWriter keeps the last tailLimit bytes written to it.
type tailWriter struct {
	buf   bytes.Buffer
	limit int
}

func (t *tailWriter) Write(b []byte) (int, error) {
	n := len(b)
	if n >= t.limit {
		t.buf.Reset()
		t.buf.Write(b[n-t.limit:])
		return n, nil
	}
	if t.buf.Len()+n > t.limit {
		keep := t.buf.Bytes()[t.buf.Len()+n-t.limit:]
		rest := append([]byte(nil), keep...)
		t.buf.Reset()
		t.buf.Write(rest)
	}
	t.buf.Write(b)
	return n, nil
}

// Run executes a tool in dir, streaming its output to the Progress as it arrives
// and keeping the end of it for the error message.
//
// The binary is resolved with bin/ first, because exec.Command searches the
// CURRENT process's PATH rather than cmd.Env — the difference between a nested
// `php` call finding the interpreter Sabdopalon manages and failing with
// `exec: "php": executable file not found in %PATH%`. The child environment gets
// the same treatment (see childEnv).
func (p *Progress) Run(dir, name string, args ...string) error {
	bin, err := resolveTool(name)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	winproc.Quiet(cmd)
	cmd.Dir = dir
	cmd.Env = childEnv()

	tail := &tailWriter{limit: tailLimit}
	out := io.MultiWriter(p.Out(), tail)
	cmd.Stdout, cmd.Stderr = out, out

	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(tail.buf.String())
		if detail != "" {
			return fmt.Errorf("%s %s failed (%w): %s", name, strings.Join(args, " "), err, detail)
		}
		return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
