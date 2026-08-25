package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/siteconfig"
	"github.com/sabdopalon/sabdopalon/internal/templates"
	"github.com/sabdopalon/sabdopalon/internal/vhost"
)

// handleAPISites handles GET (list) and POST (create) on /api/sites.
func (s *Server) handleAPISites(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listSites(w)
	case http.MethodPost:
		s.createSite(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		s.json(w, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) listSites(w http.ResponseWriter) {
	names, err := vhost.Scan(s.cfg)
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	result := []map[string]any{}
	for _, name := range names {
		u, h := s.siteURLs(name)
		sc, _ := siteconfig.Load(s.cfg.Root, name)
		item := map[string]any{
			"name":    name,
			"url":     u,
			"https":   h,
			"dir":     filepath.Join(s.cfg.Root, name),
			"running": s.proxy.IsRunning(name),
			"php":     "",
			"docroot": "",
			"aliases": []string{},
		}
		if sc != nil {
			item["php"] = sc.PHP
			item["docroot"] = sc.Docroot
			if len(sc.Aliases) > 0 {
				item["aliases"] = sc.Aliases
			}
		}
		result = append(result, item)
	}
	s.json(w, result)
}

type createSiteReq struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

// createSite scaffolds a new project from a template {name, template}.
func (s *Server) createSite(w http.ResponseWriter, r *http.Request) {
	var req createSiteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, map[string]string{"error": "invalid JSON body"})
		return
	}
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "" {
		s.json(w, map[string]string{"error": "site name is required"})
		return
	}
	if req.Template == "" {
		req.Template = "blank"
	}
	if err := templates.Create(s.cfg.Root, req.Template, req.Name); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	u, _ := s.siteURLs(req.Name)
	s.json(w, map[string]any{"ok": true, "name": req.Name, "url": u})
}

// handleAPISiteAction dispatches POST/DELETE actions on one site:
//
//	POST   /api/sites/<name>/start|stop|restart
//	DELETE /api/sites/<name>          (moves the folder to .trash/)
func (s *Server) handleAPISiteAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sites/"), "/")
	name := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}
	if name == "" {
		s.json(w, map[string]string{"error": "missing site name"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		siteDir := filepath.Join(s.cfg.Root, name)
		if _, err := os.Stat(siteDir); err != nil {
			s.json(w, map[string]string{"error": "site not found: " + name})
			return
		}
		switch action {
		case "start":
			info, err := s.proxy.StartSite(name)
			if err != nil {
				s.json(w, map[string]string{"error": err.Error()})
				return
			}
			s.json(w, map[string]any{"ok": true, "port": info.Port})
		case "stop":
			stopped := s.proxy.StopSite(name)
			s.json(w, map[string]any{"ok": true, "was_running": stopped})
		case "restart":
			if err := s.proxy.RestartSite(name); err != nil {
				s.json(w, map[string]string{"error": err.Error()})
				return
			}
			s.json(w, map[string]any{"ok": true})
		case "devtools":
			s.handleAPISiteDevTools(w, name, r)
		default:
			s.json(w, map[string]string{"error": "unknown action (start|stop|restart)"})
		}

	case http.MethodGet:
		switch action {
		case "config":
			s.getSiteConfig(w, name)
		case "logs":
			s.handleAPISiteLogs(w, name, r)
		case "devtools":
			s.handleAPISiteDevTools(w, name, r)
		case "phpini":
			s.handleAPISitePhpIni(w, name, r)
		case "":
			s.handleAPISiteDetail(w, name)
		default:
			w.Header().Set("Allow", "POST")
			w.WriteHeader(http.StatusMethodNotAllowed)
			s.json(w, map[string]string{"error": "method not allowed, use POST"})
		}

	case http.MethodPut:
		if action == "config" {
			s.putSiteConfig(w, name, r)
			return
		}
		if action == "phpini" {
			s.handleAPISitePhpIni(w, name, r)
			return
		}
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		s.json(w, map[string]string{"error": "method not allowed"})

	case http.MethodDelete:
		if err := trashSite(s.cfg.Root, name); err != nil {
			s.json(w, map[string]string{"error": err.Error()})
			return
		}
		s.proxy.StopSite(name)
		s.proxy.Enable(name)
		s.json(w, map[string]any{"ok": true, "message": name + " moved to .trash/"})

	default:
		w.Header().Set("Allow", "POST, DELETE")
		s.json(w, map[string]string{"error": "method not allowed"})
	}
}

// trashSite moves sites/<name> into <sites>/.trash/<name>-<timestamp>.
func trashSite(rootDir, name string) error {
	if strings.HasPrefix(name, ".") || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid site name")
	}
	src := filepath.Join(rootDir, name)
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("site not found: %s", name)
	}
	trashDir := filepath.Join(rootDir, ".trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(trashDir, fmt.Sprintf("%s-%d", name, time.Now().Unix()))
	return os.Rename(src, dst)
}

// siteConfigPayload is the editable per-site configuration (.sabdopalon.yml).
type siteConfigPayload struct {
	PHP     string            `json:"php"`
	PHPIni  string            `json:"php_ini"`
	Docroot string            `json:"docroot"`
	Aliases []string          `json:"aliases"`
	Env     map[string]string `json:"env"`
	Running bool              `json:"running"`
}

func (s *Server) getSiteConfig(w http.ResponseWriter, name string) {
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}
	sc, err := siteconfig.Load(s.cfg.Root, name)
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	if sc == nil {
		sc = &siteconfig.SiteConfig{Env: map[string]string{}}
	}
	s.json(w, siteConfigPayload{
		PHP:     sc.PHP,
		PHPIni:  sc.PHPIni,
		Docroot: sc.Docroot,
		Aliases: sc.Aliases,
		Env:     sc.Env,
		Running: s.proxy.IsRunning(name),
	})
}

// putSiteConfig validates and writes .sabdopalon.yml, refreshes alias routing
// immediately, and restarts the site when it is running so the new PHP
// binary/docroot take effect right away.
func (s *Server) putSiteConfig(w http.ResponseWriter, name string, r *http.Request) {
	var p siteConfigPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		s.json(w, map[string]string{"error": "invalid JSON"})
		return
	}
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}
	wasRunning := s.proxy.IsRunning(name)

	clean := &siteconfig.SiteConfig{
		PHP:     strings.TrimSpace(p.PHP),
		PHPIni:  strings.TrimSpace(p.PHPIni),
		Docroot: filepath.ToSlash(filepath.Clean("/" + strings.TrimSpace(p.Docroot)))[1:],
		Aliases: make([]string, 0, len(p.Aliases)),
		Env:     map[string]string{},
	}
	for _, a := range p.Aliases {
		a = strings.ToLower(strings.TrimSpace(a))
		if a != "" {
			clean.Aliases = append(clean.Aliases, a)
		}
	}
	for k, v := range p.Env {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && !strings.ContainsAny(k, " \t=") {
			clean.Env[k] = v
		}
	}

	if err := siteconfig.Save(s.cfg.Root, name, clean); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}

	// Apply immediately.
	s.proxy.RebuildAliases()
	restarted := false
	if wasRunning {
		if err := s.proxy.RestartSite(name); err == nil {
			restarted = true
		}
	}

	msg := "Configuration saved."
	if restarted {
		msg += " Site restarted with the new settings."
	} else if wasRunning {
		msg += " Saved — but the restart failed; check the Logs page."
	}
	u, _ := s.siteURLs(name)
	s.json(w, map[string]any{"ok": true, "message": msg, "restarted": restarted, "url": u})
}

// phpIniFile returns the absolute path to the per-site php.ini and whether it
// exists yet. The file always lives at sites/<name>/php.ini — a fixed,
// predictable location so the dashboard can edit its contents directly
// instead of asking the user for a path.
func (s *Server) phpIniFile(name string) (string, bool) {
	p := filepath.Join(s.cfg.Root, name, "php.ini")
	_, err := os.Stat(p)
	return p, err == nil
}

// handleAPISitePhpIni serves GET/PUT /api/sites/<name>/phpini — an inline editor
// for the per-site php.ini contents.
//
//	GET  → { "content": "<file or default>", "exists": bool }
//	PUT  { "content": "…" }  → writes sites/<name>/php.ini, auto-links it in
//	      .sabdopalon.yml (sets php_ini when empty), and restarts the site if
//	      it was running so the new directives take effect immediately.
func (s *Server) handleAPISitePhpIni(w http.ResponseWriter, name string, r *http.Request) {
	if strings.ContainsAny(name, "/\\") || strings.HasPrefix(name, ".") {
		s.json(w, map[string]string{"error": "invalid site name"})
		return
	}
	siteDir := filepath.Join(s.cfg.Root, name)
	if _, err := os.Stat(siteDir); err != nil {
		s.json(w, map[string]string{"error": "site not found: " + name})
		return
	}
	iniPath, exists := s.phpIniFile(name)

	switch r.Method {
	case http.MethodGet:
		if !exists {
			// Return the default global ini as a starting point so the editor
			// is never empty — the user sees what they would get otherwise.
			s.json(w, map[string]any{"content": defaultPHPIniForEditor, "exists": false})
			return
		}
		data, err := os.ReadFile(iniPath)
		if err != nil {
			s.json(w, map[string]string{"error": "read failed: " + err.Error()})
			return
		}
		s.json(w, map[string]any{"content": string(data), "exists": true})

	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.json(w, map[string]string{"error": "invalid JSON"})
			return
		}
		if err := os.WriteFile(iniPath, []byte(req.Content), 0o644); err != nil {
			s.json(w, map[string]string{"error": "write failed: " + err.Error()})
			return
		}
		// Auto-link: if .sabdopalon.yml has no php_ini override yet, set it to
		// "php.ini" (relative) so the proxy picks this file up via PHPRC.
		sc, _ := siteconfig.Load(s.cfg.Root, name)
		if sc != nil && sc.PHPIni == "" {
			sc.PHPIni = "php.ini"
			_ = siteconfig.Save(s.cfg.Root, name, sc)
		}
		// Restart the site if running so the new directives apply.
		wasRunning := s.proxy.IsRunning(name)
		restarted := false
		if wasRunning {
			if err := s.proxy.RestartSite(name); err == nil {
				restarted = true
			}
		}
		msg := "php.ini saved."
		if restarted {
			msg += " Site restarted to apply the changes."
		} else if wasRunning {
			msg += " Restart the site to apply the changes."
		}
		s.json(w, map[string]any{"ok": true, "message": msg, "restarted": restarted})

	default:
		w.Header().Set("Allow", "GET, PUT")
		s.json(w, map[string]string{"error": "method not allowed"})
	}
}

// defaultPHPIniForEditor is the default content shown in the php.ini editor
// when no per-site file exists yet (mirrors the global default).
const defaultPHPIniForEditor = `; Per-site PHP configuration for this project.
; This file is loaded via PHPRC and overrides the global config/php.ini.
; Edit freely, then save to apply (the site restarts automatically if running).

memory_limit = 256M
upload_max_filesize = 64M
post_max_size = 64M
max_execution_time = 120
date.timezone = UTC

; Optional: surface errors during development.
; display_errors = On
; error_reporting = E_ALL
`
