// Package dashboard — handlers_sitedetail.go: per-site detail aggregate API,
// multi-log tailing, and dev-tools start/stop control.
package dashboard

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/devtools"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
	"github.com/sabdopalon/sabdopalon/internal/siteconfig"
)

// SiteDetail is the aggregate response for GET /api/sites/<name>.
type SiteDetail struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	HTTPS     string            `json:"https"`
	Dir       string            `json:"dir"`
	Running   bool              `json:"running"`
	Port      int               `json:"port,omitempty"`
	Framework string            `json:"framework"`
	PHP       SiteDetailPHP     `json:"php"`
	Config    siteConfigPayload `json:"config"`
	DevTools  []devtools.Status `json:"devtools"`
	Logs      []string          `json:"logs"`
}

// SiteDetailPHP describes the PHP binary/version active for a site.
type SiteDetailPHP struct {
	Binary  string `json:"binary"`
	Version string `json:"version,omitempty"`
}

// handleAPISiteDetail returns the full per-site aggregate: framework, PHP
// binary, config, dev-tools status, and available logs. GET /api/sites/<name>.
func (s *Server) handleAPISiteDetail(w http.ResponseWriter, name string) {
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}

	u, h := s.siteURLs(name)
	framework := proxy.DetectFramework(siteDir)

	// Resolve the active PHP binary path (per-site override or global).
	phpBin := ""
	if s.cfg.PHP.Binary != "" {
		phpBin = s.cfg.PHP.Binary
	}
	sc, _ := loadSiteConfig(s.cfg.Root, name)
	if sc != nil && sc.PHP != "" {
		// We don't re-resolve here (needs pkgmgr); the list endpoint already
		// reports the configured string. Keep it simple.
		phpBin = sc.PHP
	}

	// Available dev-tools for this site dir.
	var tools []devtools.Status
	if s.dt != nil {
		tools = s.dt.Status(name)
		// Annotate availability from the site dir.
		avail := devtools.AvailableTools(siteDir)
		availMap := map[string]bool{}
		for _, a := range avail {
			availMap[a.Tool] = true
		}
		for i := range tools {
			if !availMap[tools[i].Tool] {
				tools[i].LastError = "not applicable for this project"
			}
		}
	}

	// Available log files.
	logs := s.siteLogFiles(name)

	detail := SiteDetail{
		Name:      name,
		URL:       u,
		HTTPS:     h,
		Dir:       siteDir,
		Running:   s.proxy.IsRunning(name),
		Framework: framework.String(),
		PHP:       SiteDetailPHP{Binary: phpBin},
		DevTools:  tools,
		Logs:      logs,
	}
	if sc != nil {
		detail.Config = siteConfigPayload{
			PHP:     sc.PHP,
			PHPIni:  sc.PHPIni,
			Docroot: sc.Docroot,
			Aliases: sc.Aliases,
			Env:     sc.Env,
			Running: detail.Running,
		}
	}
	s.json(w, detail)
}

// handleAPISiteLogs tails a specific log for a site.
// GET /api/sites/<name>/logs?log=<source>  (source: php, vite, artisan, …)
func (s *Server) handleAPISiteLogs(w http.ResponseWriter, name string, r *http.Request) {
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}
	source := r.URL.Query().Get("log")
	if source == "" {
		source = "php"
	}
	// Map source to a log filename: "php" → <name>.php.log, anything else →
	// <name>.<source>.log. Guard against path traversal.
	if strings.ContainsAny(source, "/\\..") {
		s.json(w, map[string]string{"error": "invalid log source"})
		return
	}
	fileName := name + ".php.log"
	if source != "php" {
		fileName = name + "." + source + ".log"
	}
	data, err := readTail(filepath.Join(s.cfg.Logs, fileName), 256*1024)
	if err != nil {
		s.json(w, map[string]any{
			"file":  fileName,
			"lines": []string{},
			"count": 0,
			"error": "no logs yet",
		})
		return
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) > 300 {
		lines = lines[len(lines)-300:]
	}
	s.json(w, map[string]any{
		"file":  fileName,
		"lines": lines,
		"count": len(lines),
	})
}

// handleAPISiteDevTools controls dev-tools for a site.
//
//	GET  /api/sites/<name>/devtools           → status of all tools
//	POST /api/sites/<name>/devtools           → {tool, action} start/stop
func (s *Server) handleAPISiteDevTools(w http.ResponseWriter, name string, r *http.Request) {
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}
	if s.dt == nil {
		s.json(w, map[string]string{"error": "dev-tools not available"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		tools := s.dt.Status(name)
		avail := devtools.AvailableTools(siteDir)
		availMap := map[string]bool{}
		for _, a := range avail {
			availMap[a.Tool] = true
		}
		for i := range tools {
			if !availMap[tools[i].Tool] {
				tools[i].LastError = "not applicable for this project"
			}
		}
		s.json(w, map[string]any{"tools": tools})

	case http.MethodPost:
		var req struct {
			Tool   string `json:"tool"`
			Action string `json:"action"` // "start" | "stop"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.json(w, map[string]string{"error": "invalid JSON"})
			return
		}
		req.Tool = strings.TrimSpace(req.Tool)
		req.Action = strings.TrimSpace(req.Action)
		if req.Tool == "" || req.Action == "" {
			s.json(w, map[string]string{"error": "tool and action are required"})
			return
		}
		switch req.Action {
		case "start":
			port, err := s.dt.Start(name, siteDir, req.Tool)
			if err != nil {
				s.json(w, map[string]string{"error": err.Error()})
				return
			}
			// If Vite started, register the reverse-proxy so HMR/asset paths
			// are served from the Vite dev server immediately.
			if req.Tool == "vite" && port > 0 {
				s.proxy.RegisterViteProxy(name, port)
			}
			s.json(w, map[string]any{"ok": true, "port": port})
		case "stop":
			if err := s.dt.Stop(name, req.Tool); err != nil {
				s.json(w, map[string]string{"error": err.Error()})
				return
			}
			if req.Tool == "vite" {
				s.proxy.UnregisterViteProxy(name)
			}
			s.json(w, map[string]any{"ok": true})
		default:
			s.json(w, map[string]string{"error": "unknown action (start|stop)"})
		}

	default:
		w.Header().Set("Allow", "GET, POST")
		s.json(w, map[string]string{"error": "method not allowed"})
	}
}

// siteLogFiles returns the names of log files that exist for a site
// (e.g. "php", "vite", "laravel-dev"). The UI uses these to populate the
// log-source tabs.
func (s *Server) siteLogFiles(name string) []string {
	entries, err := os.ReadDir(s.cfg.Logs)
	if err != nil {
		return nil
	}
	prefix := name + "."
	var out []string
	for _, e := range entries {
		fn := e.Name()
		if !strings.HasPrefix(fn, prefix) || !strings.HasSuffix(fn, ".log") {
			continue
		}
		mid := strings.TrimSuffix(strings.TrimPrefix(fn, prefix), ".log")
		if mid == "php" {
			out = append([]string{"php"}, out...) // php first
		} else {
			out = append(out, mid)
		}
	}
	return out
}

// loadSiteConfig is a thin wrapper that returns nil on error (used by the
// detail handler so a missing .sabdopalon.yml doesn't fail the whole response).
func loadSiteConfig(sitesDir, name string) (*siteConfigPayload, error) {
	sc, err := siteconfig.Load(sitesDir, name)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return nil, nil
	}
	return &siteConfigPayload{
		PHP:     sc.PHP,
		PHPIni:  sc.PHPIni,
		Docroot: sc.Docroot,
		Aliases: sc.Aliases,
		Env:     sc.Env,
	}, nil
}
