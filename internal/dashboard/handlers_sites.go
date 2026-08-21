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
	case http.MethodGet:
		if action == "config" {
			s.getSiteConfig(w, name)
			return
		}
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
		default:
			s.json(w, map[string]string{"error": "unknown action (start|stop|restart)"})
		}

	case http.MethodPut:
		if action == "config" {
			s.putSiteConfig(w, name, r)
			return
		}
		w.Header().Set("Allow", "POST, DELETE")
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
