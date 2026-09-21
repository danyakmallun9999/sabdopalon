package dashboard

import (
	"crypto/subtle"
	"net/http"
	"os"
	"sync"
	"time"
)

// shutdownFn is registered by the app package so this endpoint can run the
// exact same graceful stop as Ctrl+C.
var (
	shutdownMu sync.RWMutex
	shutdownFn func()
)

// SetShutdown registers the callback POST /api/shutdown invokes.
func SetShutdown(fn func()) {
	shutdownMu.Lock()
	shutdownFn = fn
	shutdownMu.Unlock()
}

// handleAPIShutdown lets the desktop shell stop the sidecar cleanly.
//
// The bundled sidecar is built with -H windowsgui, so it owns no console and
// Windows therefore has no CTRL_C/CTRL_BREAK to deliver to it — os.Interrupt
// never arrives. Without this endpoint the Tauri shell's only option was
// `taskkill /T /F`, which bypassed the graceful path entirely: MariaDB and
// PostgreSQL were terminated mid-write (crash recovery on every Quit) and
// services could be orphaned on their ports.
//
// Authorisation: the desktop shell passes a per-run token in the environment
// (SABDOPALON_SHUTDOWN_TOKEN). A CLI run sets no token, so the endpoint does
// not exist for it at all. The custom header also keeps a web page out: a
// cross-origin POST with a non-simple header requires a CORS preflight that
// this server never answers.
func (s *Server) handleAPIShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tok := os.Getenv("SABDOPALON_SHUTDOWN_TOKEN")
	if tok == "" {
		http.Error(w, "shutdown endpoint disabled", http.StatusNotFound)
		return
	}
	got := r.Header.Get("X-Sabdopalon-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(tok)) != 1 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	shutdownMu.RLock()
	fn := shutdownFn
	shutdownMu.RUnlock()
	if fn == nil {
		http.Error(w, "shutdown unavailable", http.StatusServiceUnavailable)
		return
	}

	// Acknowledge first, THEN stop: the caller waits on this response while
	// deciding whether to escalate to a hard kill.
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"stopping":true}`))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	go func() {
		// Give the response time to actually leave the socket.
		time.Sleep(150 * time.Millisecond)
		fn()
	}()
}
