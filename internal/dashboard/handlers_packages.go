package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
)

// installJob tracks a single package installation started from the UI.
type installJob struct {
	mu      sync.Mutex
	name    string
	running bool
	done    bool
	err     string
	output  bytes.Buffer
}

var job = &installJob{}

// handleAPIPackages lists registry packages with install status.
func (s *Server) handleAPIPackages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	m, err := pkgmgr.New(s.cfg)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, "%s", err.Error())
		return
	}
	result := []map[string]any{}
	for _, p := range m.List() {
		result = append(result, map[string]any{
			"name":      p.Name,
			"version":   p.Version,
			"short":     p.ShortVersion(),
			"license":   p.License,
			"installed": m.IsInstalled(p.Name),
			"is_php":    strings.HasPrefix(p.Name, "php"),
		})
	}
	s.json(w, result)
}

// handleAPIPackageInstall starts an async install of {"name": "<package>"}.
func (s *Server) handleAPIPackageInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		s.fail(w, http.StatusBadRequest, `body must be {"name": "<package>"}`)
		return
	}

	job.mu.Lock()
	if job.running {
		job.mu.Unlock()
		s.fail(w, http.StatusConflict, "another install is already running")
		return
	}
	job.name = req.Name
	job.running = true
	job.done = false
	job.err = ""
	job.output.Reset()
	job.mu.Unlock()

	cfg := s.cfg
	go func() {
		pr, pw := io.Pipe()
		m, merr := pkgmgr.New(cfg)
		var derr error
		if merr != nil {
			derr = merr
		} else {
			m.Out = pw
			// Resolve shorthands like "php84" / "8.4" to registry names.
			name := req.Name
			if resolved, rerr := m.ResolvePackageName(name); rerr == nil {
				name = resolved
				fmt.Fprintf(pw, "(%s → package %q)\n", req.Name, name)
			}
			derr = m.Download(name)
		}
		pw.CloseWithError(derr)
		out, _ := io.ReadAll(pr)

		job.mu.Lock()
		defer job.mu.Unlock()
		job.done = true
		job.running = false
		job.output.Write(out)
		if derr != nil {
			job.err = derr.Error()
		}
	}()

	s.json(w, map[string]any{"ok": true, "message": "installing " + req.Name})
}

// handleAPIPackageJob reports current/last install progress.
func (s *Server) handleAPIPackageJob(w http.ResponseWriter, r *http.Request) {
	job.mu.Lock()
	defer job.mu.Unlock()
	resp := map[string]any{
		"name":    job.name,
		"running": job.running,
		"done":    job.done,
		"output":  job.output.String(),
	}
	if job.err != "" {
		resp["error"] = job.err
	}
	s.json(w, resp)
}
