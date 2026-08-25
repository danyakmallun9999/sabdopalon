package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/sabdopalon/sabdopalon/internal/sysinstall"
)

// sysToolJob tracks a single system-tool installation started from the UI.
// Mirrors the bundled-package installJob in handlers_packages.go.
type sysToolJob struct {
	mu      sync.Mutex
	name    string
	running bool
	done    bool
	err     string
	output  bytes.Buffer
}

var stJob = &sysToolJob{}

// sysToolProgress implements sysinstall.Progress, streaming install output
// into the job buffer for the UI's poll of /api/sys-tools/job.
type sysToolProgress struct{ j *sysToolJob }

func (w sysToolProgress) Write(p []byte) (int, error) {
	w.j.mu.Lock()
	defer w.j.mu.Unlock()
	return w.j.output.Write(p)
}

func (w sysToolProgress) Printf(format string, a ...any) {
	w.j.mu.Lock()
	defer w.j.mu.Unlock()
	fmt.Fprintf(&w.j.output, format, a...)
}

// handleAPISysTools lists system tools (Node.js, Composer) with detection status.
func (s *Server) handleAPISysTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	result := []map[string]any{}
	for _, t := range sysinstall.List() {
		installed := sysinstall.IsInstalled(t.Name)
		ver := ""
		if installed {
			ver = sysinstall.Version(t.Name)
		}
		entry := map[string]any{
			"name":      t.Name,
			"label":     t.Label,
			"installed": installed,
			"version":   ver,
		}
		result = append(result, entry)
	}
	s.json(w, result)
}

// handleAPISysToolInstall starts an async install of {"name": "<tool>"}.
func (s *Server) handleAPISysToolInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		s.fail(w, http.StatusBadRequest, `body must be {"name": "<tool>"}`)
		return
	}

	stJob.mu.Lock()
	if stJob.running {
		stJob.mu.Unlock()
		s.fail(w, http.StatusConflict, "another install is already running")
		return
	}
	stJob.name = req.Name
	stJob.running = true
	stJob.done = false
	stJob.err = ""
	stJob.output.Reset()
	stJob.mu.Unlock()

	go func() {
		p := sysToolProgress{stJob}
		_, err := sysinstall.Install(req.Name, p)
		stJob.mu.Lock()
		defer stJob.mu.Unlock()
		stJob.done = true
		stJob.running = false
		if err != nil {
			stJob.err = err.Error()
		}
		// PATH wiring + "open a new terminal" hint is handled inside
		// sysinstall.Install (ensurePath), so no extra message here.
	}()

	s.json(w, map[string]any{"ok": true, "message": "installing " + req.Name})
}

// handleAPISysToolJob reports current/last system-tool install progress.
func (s *Server) handleAPISysToolJob(w http.ResponseWriter, r *http.Request) {
	stJob.mu.Lock()
	defer stJob.mu.Unlock()
	resp := map[string]any{
		"name":    stJob.name,
		"running": stJob.running,
		"done":    stJob.done,
		"output":  stJob.output.String(),
	}
	if stJob.err != "" {
		resp["error"] = stJob.err
	}
	s.json(w, resp)
}
