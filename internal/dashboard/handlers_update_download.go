// Package dashboard — handlers_update_download.go: fetch the asset a check found.
//
// Downloading is still not installing: this proves the bytes are the ones the
// release published, and stops there. Applying an update — stopping the sidecar,
// swapping the file, starting it again — belongs to the desktop shell, which is the
// process that survives that restart.
package dashboard

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/update"
)

// updateJob is the observable state of the one download that can be in flight.
type updateJob struct {
	mu       sync.Mutex
	running  bool
	done     int64
	total    int64
	path     string
	err      string
	started  string
	finished string
}

var updateDownload updateJob

func (j *updateJob) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return map[string]any{
		"running":  j.running,
		"done":     j.done,
		"total":    j.total,
		"path":     j.path,
		"error":    j.err,
		"started":  j.started,
		"finished": j.finished,
	}
}

// handleAPIUpdateDownload serves /api/update/download.
//
//	GET  → the state of the current or last download, never a network call
//	POST → start one, using the asset the last check found
func (s *Server) handleAPIUpdateDownload(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.json(w, updateDownload.snapshot())
		return
	case http.MethodPost:
		// fall through to the start path
	default:
		s.json(w, map[string]string{"error": "method not allowed"})
		return
	}

	updateCache.mu.Lock()
	payload := updateCache.got
	updateCache.mu.Unlock()

	if !payload.Available {
		s.json(w, map[string]string{"error": "There is no update to download — check for one first."})
		return
	}
	if payload.AssetURL == "" || payload.AssetDigest == "" {
		s.json(w, map[string]string{
			"error": "This release published no checksum for its asset, so it cannot be verified.",
		})
		return
	}

	updateDownload.mu.Lock()
	if updateDownload.running {
		updateDownload.mu.Unlock()
		s.json(w, map[string]string{"error": "A download is already running."})
		return
	}
	updateDownload.running = true
	updateDownload.done, updateDownload.total = 0, 0
	updateDownload.path, updateDownload.err = "", ""
	updateDownload.started = time.Now().UTC().Format(time.RFC3339)
	updateDownload.finished = ""
	updateDownload.mu.Unlock()

	url, digest, name := payload.AssetURL, payload.AssetDigest, payload.AssetName
	// Staging inside the data root: the shell swaps the file from here, and nothing
	// is written anywhere the user's projects or the running program live.
	dir := filepath.Join(s.cfg.RootDir, "updates")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		path, err := update.Download(ctx, url, digest, dir, name, func(done, total int64) {
			updateDownload.mu.Lock()
			updateDownload.done, updateDownload.total = done, total
			updateDownload.mu.Unlock()
		})

		updateDownload.mu.Lock()
		updateDownload.running = false
		updateDownload.finished = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			updateDownload.err = err.Error()
		} else {
			updateDownload.path = path
		}
		updateDownload.mu.Unlock()
	}()

	s.json(w, updateDownload.snapshot())
}
