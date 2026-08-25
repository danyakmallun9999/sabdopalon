package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// jobWriter is the install job's live progress sink: every Write appends to
// job.output under the mutex so the UI's poll of /api/packages/job sees output
// as it arrives instead of only after the install finishes. (A plain
// io.Pipe + ReadAll-after-Download deadlocked and hid all progress until
// completion — the root cause of the UI stuck on "starting…".)
type jobWriter struct{ j *installJob }

func (w jobWriter) Write(p []byte) (int, error) {
	w.j.mu.Lock()
	defer w.j.mu.Unlock()
	return w.j.output.Write(p)
}

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

	// Resolve the PHP actually in use so the UI can mark the matching package
	// card "active". "Installed" below only means a bundled copy exists in bin/;
	// but when the system supplies the PHP Sabdopalon runs on, the card still
	// showed "not installed" while the status header said "php 8.5.8" — a
	// direct contradiction. "active" bridges that: it is true when the package
	// version matches the PHP Sabdopalon is currently using, regardless of
	// whether that PHP is bundled or system-supplied.
	activeShort := ""
	if bin := s.cfg.PHP.Binary; bin != "" {
		if v := pkgmgr.PHPBinaryVersion(bin); v != "" {
			activeShort = majorMinor(v)
		}
	}

	result := []map[string]any{}
	for _, p := range m.List() {
		isPHP := strings.HasPrefix(p.Name, "php")
		entry := map[string]any{
			"name":      p.Name,
			"version":   p.Version,
			"short":     p.ShortVersion(),
			"license":   p.License,
			"installed": m.IsInstalled(p.Name),
			"is_php":    isPHP,
			// "active" is only meaningful for PHP packages: true when this
			// package's version is the one Sabdopalon is currently running.
			"active": isPHP && activeShort != "" && p.ShortVersion() == activeShort,
		}
		result = append(result, entry)
	}
	s.json(w, result)
}

// majorMinor returns the "X.Y" prefix of a dotted version string.
func majorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
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
		// Progress is written straight into job.output (via a jobWriter) so
		// the UI's poll of /api/packages/job sees it live. The previous code
		// used an unbuffered io.Pipe and read it with ReadAll AFTER Download
		// returned — the first progress Write blocked forever (io.Pipe has
		// no internal buffer), deadlocking the install and leaving the UI
		// permanently on "starting…".
		m, merr := pkgmgr.New(cfg)
		var derr error
		if merr != nil {
			derr = merr
		} else {
			m.Out = jobWriter{job}
			// Resolve shorthands like "php84" / "8.4" to registry names.
			name := req.Name
			if resolved, rerr := m.ResolvePackageName(name); rerr == nil {
				name = resolved
				fmt.Fprintf(m.Out, "(%s → package %q)\n", req.Name, name)
			}
			derr = m.Download(name)
		}

		job.mu.Lock()
		defer job.mu.Unlock()
		job.done = true
		job.running = false
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
