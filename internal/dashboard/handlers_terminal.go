package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/sabdopalon/sabdopalon/internal/terminal"
)

// terminalMessage is the client→server frame: input bytes or a resize.
type terminalMessage struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// --- named sessions: survive disconnects, support reattach + replay -------

const (
	termBufMax      = 256 << 10        // replay ring cap per session
	termMaxSessions = 12               // LRU cap across the dashboard
	termIdleTTL     = 30 * time.Minute // detached sessions die after this
)

// termSession wraps a PTY with a replay buffer and at most one attached
// client ("sink"). The shell outlives the websocket: navigating away, a
// reload or a laptop nap only detaches the sink — output keeps buffering —
// and reconnecting with the same session key replays it into the live shell.
//
// Exactly ONE permanent pump goroutine reads the PTY (created with the
// session); clients never touch sess.Read. That makes kick/reattach trivial:
// swapping the sink cannot steal bytes or interleave writers.
type termSession struct {
	sess *terminal.Session

	mu      sync.Mutex
	buf     []byte          // recent output, trimmed to termBufMax
	sink    *websocket.Conn // current client (nil = detached)
	sinkCtx context.Context // canceled with the client's request
	clients int
	lastUse time.Time
}

var (
	termReg     = map[string]*termSession{}
	termMu      sync.Mutex
	termJanitor sync.Once
)

func (t *termSession) setSink(ctx context.Context, c *websocket.Conn) {
	t.mu.Lock()
	t.sink, t.sinkCtx = c, ctx
	t.clients++
	t.lastUse = time.Now()
	t.mu.Unlock()
}

func (t *termSession) clearSink(c *websocket.Conn) {
	t.mu.Lock()
	if t.sink == c {
		t.sink, t.sinkCtx = nil, nil
	}
	t.clients--
	t.lastUse = time.Now()
	t.mu.Unlock()
}

// writeSink delivers one chunk to the attached client, if any and still
// connected. Called only from the pump.
func (t *termSession) writeSink(data []byte) {
	t.mu.Lock()
	c, ctx := t.sink, t.sinkCtx
	t.mu.Unlock()
	if c == nil || ctx.Err() != nil {
		return
	}
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.clearSink(c)
	}
}

// record appends output to the replay ring (called from the pump only).
func (t *termSession) record(chunk []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, chunk...)
	if len(t.buf) > termBufMax {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-termBufMax:]...)
	}
}

func (t *termSession) replay() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return []byte(strings.ToValidUTF8(string(t.buf), "\uFFFD"))
}

func (t *termSession) destroy() { _ = t.sess.Close() }

// startPump is the session's single PTY reader: it buffers everything and
// forwards to the attached sink, holding back partial UTF-8 runes across
// read boundaries. Exits only when the PTY dies.
func (t *termSession) startPump() {
	go func() {
		buf := make([]byte, 4096)
		var pend []byte
		for {
			n, err := t.sess.Read(buf)
			if n > 0 {
				chunk := append(pend, buf[:n]...)
				cut := utf8PrefixLen(chunk)
				if cut > 0 {
					out := append([]byte(nil), chunk[:cut]...)
					t.record(out)
					t.writeSink(out)
					pend = append(pend[:0], chunk[cut:]...)
				} else {
					// No valid boundary yet — hold. If a pathological binary
					// flood never yields one, flush it through with U+FFFD
					// placeholders instead of dropping or growing forever.
					pend = append(pend[:0], chunk...)
					if len(pend) > 8192 {
						out := []byte(strings.ToValidUTF8(string(pend), "\uFFFD"))
						t.record(out)
						t.writeSink(out)
						pend = pend[:0]
					}
				}
			}
			if err != nil {
				return // PTY gone; janitor/destroy handles the rest
			}
		}
	}()
}

// startTermJanitor reaps detached sessions so abandoned shells (closed tab,
// forgotten dock) never accumulate forever.
func startTermJanitor() {
	termJanitor.Do(func() {
		go func() {
			for range time.Tick(5 * time.Minute) {
				termMu.Lock()
				for k, t := range termReg {
					t.mu.Lock()
					idle := time.Since(t.lastUse)
					busy := t.clients > 0
					t.mu.Unlock()
					if !busy && idle > termIdleTTL {
						t.destroy()
						delete(termReg, k)
					}
				}
				termMu.Unlock()
			}
		}()
	})
}

// handleAPITerminalWS upgrades a WebSocket connection to one terminal.
//
// Without a "session" query param the session lives exactly as long as the
// connection (legacy behaviour). With ?session=<key> the shell is named and
// persistent: disconnect keeps it alive (buffering output), reconnecting
// with the same key replays the buffer into the live shell, and
// ?fresh=1&session=<key> replaces it with a brand-new shell.
func (s *Server) handleAPITerminalWS(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	key := strings.TrimSpace(r.URL.Query().Get("session"))
	fresh := r.URL.Query().Get("fresh") == "1"

	// kill=1: destroy the named session WITHOUT creating a replacement —
	// used when the UI closes a terminal tab.
	if key != "" && r.URL.Query().Get("kill") == "1" {
		termMu.Lock()
		if old := termReg[key]; old != nil {
			old.destroy()
			delete(termReg, key)
		}
		termMu.Unlock()
		w.WriteHeader(http.StatusOK)
		return
	}

	// Running optional services contribute their PHP-style env so CLI tools
	// inside the shell see the same variables as sites do.
	var extraEnv []string
	if s.svc != nil {
		extraEnv = s.svc.EnvVars()
	}

	// Resolve or create the session (named) before touching the socket.
	var ts *termSession
	var created bool
	if key != "" {
		termMu.Lock()
		if fresh {
			if old := termReg[key]; old != nil {
				old.destroy()
				delete(termReg, key)
			}
		}
		ts = termReg[key]
		if ts == nil {
			sess, err := terminal.New(s.cfg, dir, extraEnv)
			if err != nil {
				termMu.Unlock()
				http.Error(w, "terminal: "+err.Error(), http.StatusInternalServerError)
				return
			}
			ts = &termSession{sess: sess, lastUse: time.Now()}
			ts.startPump()
			termReg[key] = ts
			created = true
			// LRU: evict the oldest idle session over the cap.
			if len(termReg) > termMaxSessions {
				var oldestKey string
				var oldest time.Time
				for k, t := range termReg {
					t.mu.Lock()
					at := t.lastUse
					busy := t.clients > 0
					t.mu.Unlock()
					if busy {
						continue
					}
					if oldestKey == "" || at.Before(oldest) {
						oldestKey, oldest = k, at
					}
				}
				if oldestKey != "" {
					termReg[oldestKey].destroy()
					delete(termReg, oldestKey)
				}
			}
			startTermJanitor()
		}
		termMu.Unlock()
	} else {
		sess, err := terminal.New(s.cfg, dir, extraEnv)
		if err != nil {
			http.Error(w, "terminal: "+err.Error(), http.StatusInternalServerError)
			return
		}
		ts = &termSession{sess: sess, lastUse: time.Now()}
		ts.startPump()
		defer ts.destroy()
	}

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost", "127.0.0.1"},
	})
	if err != nil {
		if created {
			// The socket never came up; don't leave a brand-new empty
			// session behind to occupy an LRU slot.
			termMu.Lock()
			if termReg[key] == ts {
				ts.destroy()
				delete(termReg, key)
			}
			termMu.Unlock()
		}
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Attach as the session's sink (kicks any previous client), then replay
	// the buffered output so the terminal looks exactly as it was left.
	ts.setSink(ctx, c)
	if key != "" {
		if rep := ts.replay(); len(rep) > 0 {
			if werr := c.Write(ctx, websocket.MessageText, rep); werr != nil {
				ts.clearSink(c)
				return
			}
		}
	}
	defer ts.clearSink(c)

	// Client frames → shell input / resize.
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg terminalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			_, _ = ts.sess.Write([]byte(msg.Data))
		case "resize":
			if msg.Rows > 0 && msg.Cols > 0 {
				_ = ts.sess.Resize(msg.Rows, msg.Cols)
			}
		}
	}
}

// utf8PrefixLen returns the length of the longest valid UTF-8 prefix of b,
// holding back an incomplete trailing rune until more bytes arrive.
func utf8PrefixLen(b []byte) int {
	for i := len(b) - 1; i >= len(b)-utf8.UTFMax && i >= 0; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.Valid(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}
