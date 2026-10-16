// Package dashboard — logsimplify.go: turning a build's output into something a
// person can read.
//
// Creating a Laravel project prints, measured on a real run: 23.7 KB of output,
// 1736 terminal escape sequences, 270 carriage returns, about two hundred lines
// that each name a single package, and ninety dots per migration line. All of it
// went into the dialog verbatim, which is how a log ends up unreadable — and how
// a long run of ANSI codes, having no space to wrap at, stretched the dialog past
// the window edge.
//
// This keeps the shape of what happened (which step, how many packages, which
// migrations) and drops the terminal theatre. The raw bytes are kept alongside,
// so nothing is lost: the dialog can show them, and they are what a bug report
// should quote.
package dashboard

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

const (
	// maxLogLines bounds the simplified view. A long npm install prints thousands
	// of lines and the dialog only ever shows the end of them.
	maxLogLines = 400
	// maxLogLineLen keeps one pathological line from filling the view.
	maxLogLineLen = 400
	// rawLogKeep bounds the raw copy kept for diagnostics.
	rawLogKeep = 32 << 10
)

var (
	// Artisan pads a migration line with dots until the timing is known.
	logDotsRe = regexp.MustCompile(`\s*\.{3,}\s*`)
	logGapRe  = regexp.MustCompile(`[ \t]{2,}`)
	// Composer names one package per line while it locks or installs.
	logPkgRe = regexp.MustCompile(`^\s*-\s+(Locking|Installing)\s+(\S+)(?:\s+\(([^)]*)\))?`)
)

// createLog is the log of one site creation in both forms.
type createLog struct {
	mu      sync.Mutex
	partial []byte // the line being written
	pending bool   // a carriage return is waiting for the byte that explains it
	esc     byte   // escape state carried between writes: 0, '[', or ']'
	lines   []string
	raw     tailBuffer

	// A run of composer package lines collapses into one line that counts them,
	// so a hundred "Installing …" lines become one that keeps updating.
	run      string
	runCount int
	runAt    int
}

// Write implements io.Writer.
//
// State is carried between calls because a pipe delivers buffers, not lines: an
// escape sequence, a carriage return and a half-finished line all arrive split
// wherever the buffer happened to end.
func (l *createLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.raw.Write(p)

	for i := 0; i < len(p); i++ {
		b := p[i]

		// Inside an escape sequence, skip until it ends — in this write or a later
		// one, which is what the carried state is for.
		if l.esc != 0 {
			switch l.esc {
			case '[':
				if b >= 0x40 && b <= 0x7e {
					l.esc = 0
				}
			case ']':
				if b == 0x07 {
					l.esc = 0
				} else if b == 0x1b {
					l.esc = '\\' // maybe the two-byte string terminator ESC \
				}
			case '\\':
				l.esc = 0 // ESC \ closed an OSC; one byte is enough
			case '?':
				// A lone ESC: the byte after it decides whether this was the start of
				// a CSI/OSC (split across writes) or a two-byte sequence.
				if b == '[' || b == ']' {
					l.esc = b
				} else {
					l.esc = 0
				}
			}
			continue
		}
		if b == 0x1b {
			if i+1 < len(p) {
				switch p[i+1] {
				case '[', ']':
					l.esc = p[i+1]
					i++
					continue
				}
			}
			// A lone escape, or one whose introducer has not arrived yet.
			l.esc = '?'
			continue
		}

		if l.pending {
			l.pending = false
			if b == '\n' {
				l.emit()
				continue
			}
			// A carriage return with data after it rewrites the line: a progress
			// bar drawn ten times is one line, and only its last state counts.
			l.partial = l.partial[:0]
		}
		switch {
		case b == '\r':
			l.pending = true
		case b == '\n':
			l.emit()
		case b == '\b':
			if n := len(l.partial); n > 0 {
				l.partial = l.partial[:n-1]
			}
		case b == '\t':
			l.partial = append(l.partial, ' ')
		case b < 0x20:
			// Any other control byte means nothing in a log line.
		default:
			if len(l.partial) < 4*maxLogLineLen {
				l.partial = append(l.partial, b)
			}
		}
	}
	return len(p), nil
}

// Simplified is the readable form: what the dialog shows.
func (l *createLog) Simplified() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := strings.Join(l.lines, "\n")
	// A line still being written is shown as it stands rather than held back, so
	// the view keeps moving during a long step. It is not emitted: the same line
	// will be completed by later writes.
	if tail := cleanLogLine(string(l.partial)); tail != "" {
		if out != "" {
			out += "\n"
		}
		out += tail
	}
	return out
}

// Raw is the untouched output, to the last rawLogKeep bytes.
func (l *createLog) Raw() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.raw.String()
}

func (l *createLog) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.partial = nil
	l.pending = false
	l.lines = nil
	l.raw.reset()
	l.run, l.runCount, l.runAt = "", 0, 0
}

// emit turns the finished line into output.
func (l *createLog) emit() {
	text := string(l.partial)
	l.partial = l.partial[:0]

	if kind, label, ok := packageLine(text); ok {
		l.collapsePackages(kind, label)
		return
	}
	// Any other line ends a package run.
	l.run = ""

	line := cleanLogLine(text)
	if line == "" {
		return
	}
	if n := len(l.lines); n > 0 && l.lines[n-1] == line {
		return // composer repeats itself; the reader does not need it twice
	}
	l.append(line)
}

// append adds a line and keeps the view bounded.
func (l *createLog) append(line string) {
	l.lines = append(l.lines, line)
	if len(l.lines) > maxLogLines {
		drop := len(l.lines) - maxLogLines
		l.lines = append([]string(nil), l.lines[drop:]...)
		l.runAt -= drop
	}
}

// collapsePackages folds one package line into the run's single counter line.
func (l *createLog) collapsePackages(kind, label string) {
	if l.run != kind || l.runAt < 0 || l.runAt >= len(l.lines) {
		l.run, l.runCount = kind, 0
		l.append("")
		l.runAt = len(l.lines) - 1
	}
	l.runCount++
	l.lines[l.runAt] = fmt.Sprintf("%s packages… %d (last: %s)", kind, l.runCount, label)
}

// packageLine recognises composer's per-package lines and returns a label for the
// counter, e.g. "laravel/framework v13.37.0".
func packageLine(line string) (kind, label string, ok bool) {
	m := logPkgRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	label = m[2]
	if m[3] != "" {
		label += " " + m[3]
	}
	return m[1], label, true
}

// cleanLogLine does the per-line tidy-up: no escape codes survive to here, but
// the padding, the dots and the progress bars do.
func cleanLogLine(line string) string {
	t := strings.TrimSpace(line)
	if t == "" {
		return ""
	}
	t = logDotsRe.ReplaceAllString(t, " ")
	t = logGapRe.ReplaceAllString(t, " ")
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	// A line with no letters is a progress bar, a rule, or punctuation: something
	// drawn for a terminal, not said to a person.
	if !hasLetter(t) {
		return ""
	}
	if len(t) > maxLogLineLen {
		t = strings.TrimSpace(t[:maxLogLineLen]) + "…"
	}
	return t
}

func hasLetter(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			return true
		}
	}
	return false
}

// tailBuffer keeps the last limit bytes written to it, starting on a line
// boundary so the raw view never opens with half a line.
type tailBuffer struct {
	buf   []byte
	limit int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	if t.limit == 0 {
		t.limit = rawLogKeep
	}
	n := len(p)
	if n >= t.limit {
		t.buf = append(t.buf[:0], p[n-t.limit:]...)
	} else {
		t.buf = append(t.buf, p...)
		if len(t.buf) > t.limit {
			t.buf = append(t.buf[:0], t.buf[len(t.buf)-t.limit:]...)
		}
	}
	return n, nil
}

func (t *tailBuffer) String() string {
	b := t.buf
	if len(b) == 0 {
		return ""
	}
	// Do not begin mid-character or mid-line after a cut.
	for len(b) > 0 && b[0]&0xc0 == 0x80 {
		b = b[1:]
	}
	if len(t.buf) >= t.limit {
		if i := indexByte(b, '\n'); i >= 0 && i+1 < len(b) {
			b = b[i+1:]
		}
	}
	return string(b)
}

func (t *tailBuffer) reset() { t.buf = nil }

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}
