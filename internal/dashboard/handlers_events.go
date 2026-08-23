package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleAPIEvents streams the status snapshot over Server-Sent Events.
//
// GET /api/events — THE single live feed every dashboard page subscribes to,
// replacing per-page status polling. Snapshots are pushed only when the
// JSON actually changes (2 s check), with SSE comments as keepalives so
// proxies don't close idle streams. EventSource reconnects automatically.
func (s *Server) handleAPIEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no")

	// Immediate first snapshot so pages render instantly on subscribe.
	lastJSON := ""
	send := func() bool {
		snap, err := json.Marshal(s.statusSnapshot())
		if err != nil {
			return true // keep the stream alive even if a snapshot fails
		}
		if string(snap) == lastJSON {
			return true // unchanged — nothing to push
		}
		if _, err := fmt.Fprintf(w, "event: status\ndata: %s\n\n", snap); err != nil {
			return false // client gone
		}
		fl.Flush()
		lastJSON = string(snap)
		return true
	}

	if !send() {
		return
	}

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	beat := time.NewTicker(15 * time.Second)
	defer beat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			if !send() {
				return
			}
		case <-beat.C:
			// Comment line = keepalive; never triggers EventSource events.
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
