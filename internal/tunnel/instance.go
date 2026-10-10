package tunnel

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"
)

// Status is the public state of one site's tunnel.
type Status string

const (
	StatusOff      Status = "off"
	StatusStarting Status = "starting"
	StatusOnline   Status = "online"
	StatusError    Status = "error"
)

// State is what the API and the SSE snapshot expose for one site.
type State struct {
	Enabled     bool     `json:"enabled"`
	Status      Status   `json:"status"`
	URL         string   `json:"url"`
	AllowedMail []string `json:"allowed_mail"`
	Error       string   `json:"error"`
	StartedAt   string   `json:"started_at"`
}

// instance is one site's cloudflared process plus the state the UI reads.
//
// Every field is behind the mutex because three parties touch it: the HTTP
// handler asking for a snapshot, the goroutine reading cloudflared's output, and
// the shutdown path.
type instance struct {
	site string
	mail []string

	mu       sync.Mutex
	cancel   context.CancelFunc
	url      string
	status   Status
	errMsg   string
	started  time.Time
	stopping bool

	logPath string
}

func newInstance(site string, mail []string, logPath string) *instance {
	return &instance{
		site:    site,
		mail:    mail,
		status:  StatusStarting,
		started: time.Now(),
		logPath: logPath,
	}
}

func (i *instance) snapshot() State {
	i.mu.Lock()
	defer i.mu.Unlock()
	started := ""
	if !i.started.IsZero() && i.status != StatusOff {
		started = i.started.UTC().Format(time.RFC3339)
	}
	mail := make([]string, len(i.mail))
	copy(mail, i.mail)
	return State{
		Enabled:     i.status != StatusOff,
		Status:      i.status,
		URL:         i.url,
		AllowedMail: mail,
		Error:       i.errMsg,
		StartedAt:   started,
	}
}

// setURL is called from the output reader the moment cloudflared prints the
// public hostname.
func (i *instance) setURL(u string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.url = u
	i.status = StatusOnline
	i.errMsg = ""
}

// setStarting is used when a dropped tunnel is being brought back up: the URL
// from the previous run is gone, so it must not be shown as live.
func (i *instance) setStarting() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.url = ""
	i.status = StatusStarting
	i.errMsg = ""
}

func (i *instance) setError(msg string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.url = ""
	i.status = StatusError
	i.errMsg = msg
}

func (i *instance) setOff() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.url = ""
	i.status = StatusOff
	i.errMsg = ""
}

func (i *instance) hasURL() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.url != ""
}

func (i *instance) stopped() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.stopping
}

func (i *instance) stop() {
	i.mu.Lock()
	i.stopping = true
	cancel := i.cancel
	i.mu.Unlock()
	// Cancelling the context is what kills cloudflared (see execRunner). There is
	// no graceful drain worth waiting for: a quick tunnel carries no state, and on
	// Windows there is no SIGTERM to send in the first place.
	if cancel != nil {
		cancel()
	}
}

// setCancel stores the current run's cancel func so stop() can reach it.
func (i *instance) setCancel(cancel context.CancelFunc) {
	i.mu.Lock()
	i.cancel = cancel
	i.mu.Unlock()
}

// tailBuffer keeps the last few kilobytes of output so an error can quote the
// reason. cloudflared's failures are explained in its log, not in the exit code.
type tailBuffer struct {
	max  int
	data []byte
}

func newTailBuffer(max int) *tailBuffer { return &tailBuffer{max: max} }

func (t *tailBuffer) add(p []byte) {
	t.data = append(t.data, p...)
	if len(t.data) > t.max {
		t.data = t.data[len(t.data)-t.max:]
	}
}

func (t *tailBuffer) String() string { return string(t.data) }

// lineWriter forwards complete lines to onLine and mirrors everything to the
// log file. cloudflared's URL only exists as a log line, so the reader is on the
// hot path of starting a tunnel.
//
// It is an io.Writer rather than a scanner over an io.Pipe so exec hands us both
// stdout and stderr directly: with a single writer there is no chance of the
// banner being split across two buffers, and no pipe to leak when cloudflared
// dies mid-run.
type lineWriter struct {
	mu       sync.Mutex
	partial  []byte
	onLine   func(string)
	file     *os.File
	onOutput func([]byte)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_, _ = w.file.Write(p)
	}
	if w.onOutput != nil {
		w.onOutput(p)
	}
	w.partial = append(w.partial, p...)
	for {
		idx := indexNewline(w.partial)
		if idx < 0 {
			break
		}
		line := string(w.partial[:idx])
		w.partial = w.partial[idx+1:]
		if w.onLine != nil {
			w.onLine(line)
		}
	}
	// Guard against a single line with no newline growing without bound.
	if len(w.partial) > 64*1024 {
		w.partial = w.partial[len(w.partial)-64*1024:]
	}
	return len(p), nil
}

func indexNewline(b []byte) int {
	for i := 0; i < len(b); i++ {
		if b[i] == '\n' {
			return i
		}
	}
	return -1
}

func hostFromURL(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	u = strings.TrimSuffix(u, "/")
	return strings.ToLower(strings.TrimSpace(u))
}
