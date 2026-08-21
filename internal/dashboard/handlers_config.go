package dashboard

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/profiles"
)

// handleAPIStatus reports overall system state for the header/status card.
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	httpPort, httpsPort := s.proxy.Ports()
	resp := map[string]any{
		"version":     Version,
		"uptime":      time.Since(s.started).Round(time.Second).String(),
		"http_port":   httpPort,
		"https_port":  httpsPort,
		"low_ports":   s.proxy.LowPortsBound(),
		"tld":         s.cfg.TLD,
		"database":    s.cfg.Database.Engine,
		"sites_count": len(s.proxy.RunningSites()),
		"mailpit":     s.svcRunning(),
	}
	if s.cfg.PHP.Binary != "" {
		resp["php"] = baseName(s.cfg.PHP.Binary)
	}
	s.json(w, resp)
}

type configPayload struct {
	TLD         string `json:"tld,omitempty"`
	HTTPPort    *int   `json:"http_port,omitempty"`
	HTTPSPort   *int   `json:"https_port,omitempty"`
	DBEngine    string `json:"db_engine,omitempty"`
	DBPort      *int   `json:"db_port,omitempty"`
	DashEnabled *bool  `json:"dashboard_enabled,omitempty"`
	DashPort    *int   `json:"dashboard_port,omitempty"`
	AutoOpen    *bool  `json:"auto_open,omitempty"`
	Mailpit     *bool  `json:"mailpit_enabled,omitempty"`
}

// handleAPIConfig serves GET/PUT /api/config — the Settings page.
// PUT persists to config/engine.toml; structural changes require a restart.
func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		httpPort, httpsPort := s.proxy.Ports()
		s.json(w, configPayload{
			TLD:         s.cfg.TLD,
			HTTPPort:    intPtr(orActual(httpPort, s.cfg.Proxy.HTTPPort)),
			HTTPSPort:   intPtr(orActual(httpsPort, s.cfg.Proxy.HTTPSPort)),
			DBEngine:    s.cfg.Database.Engine,
			DBPort:      intPtr(s.cfg.Database.Port),
			DashEnabled: boolPtr(s.cfg.Dashboard.Enabled),
			DashPort:    intPtr(s.cfg.Dashboard.Port),
			AutoOpen:    boolPtr(s.cfg.Dashboard.AutoOpen),
			Mailpit:     boolPtr(s.cfg.Services.Mailpit),
		})

	case http.MethodPut:
		var p configPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			s.json(w, map[string]string{"error": "invalid JSON"})
			return
		}
		restart := false
		validPort := func(n int) bool { return n > 0 && n <= 65535 }
		if p.TLD != "" && p.TLD != s.cfg.TLD {
			s.cfg.TLD = p.TLD
			restart = true
		}
		if p.HTTPPort != nil && validPort(*p.HTTPPort) && *p.HTTPPort != s.cfg.Proxy.HTTPPort {
			s.cfg.Proxy.HTTPPort = *p.HTTPPort
			restart = true
		}
		if p.HTTPSPort != nil && validPort(*p.HTTPSPort) && *p.HTTPSPort != s.cfg.Proxy.HTTPSPort {
			s.cfg.Proxy.HTTPSPort = *p.HTTPSPort
			restart = true
		}
		if p.DBEngine != "" && p.DBEngine != s.cfg.Database.Engine {
			s.cfg.Database.Engine = p.DBEngine
			restart = true
		}
		if p.DBPort != nil && validPort(*p.DBPort) {
			s.cfg.Database.Port = *p.DBPort
		}
		if p.DashEnabled != nil {
			s.cfg.Dashboard.Enabled = *p.DashEnabled
		}
		if p.DashPort != nil && validPort(*p.DashPort) {
			s.cfg.Dashboard.Port = *p.DashPort
		}
		if p.AutoOpen != nil {
			s.cfg.Dashboard.AutoOpen = *p.AutoOpen
		}
		if p.Mailpit != nil {
			s.cfg.Services.Mailpit = *p.Mailpit
		}

		if err := s.cfg.Save(); err != nil {
			s.json(w, map[string]string{"error": "save failed: " + err.Error()})
			return
		}
		msg := "Settings saved."
		if restart {
			msg += " Some changes apply after restarting Sabdopalon."
		}
		s.json(w, map[string]any{"ok": true, "message": msg, "restart_required": restart})

	default:
		w.Header().Set("Allow", "GET, PUT")
		s.json(w, map[string]string{"error": "method not allowed"})
	}
}

func orActual(actual, cfg int) int {
	if actual != 0 && actual != cfg {
		return actual
	}
	return cfg
}

// handleAPIProfiles lists saved profiles.
func (s *Server) handleAPIProfiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	m := profiles.New(s.cfg)
	list, err := m.List()
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, list)
}

// handleAPIProfileApply overlays a profile onto the live config {name} and
// stops all sites so they restart with the new PHP binary. DB engine changes
// still need a full restart.
func (s *Server) handleAPIProfileApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		s.json(w, map[string]string{"error": `body must be {"name": "<profile>"}`})
		return
	}
	cfg, err := profiles.Apply(s.cfg, req.Name)
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	dbChanged := cfg.Database.Engine != s.cfg.Database.Engine
	*s.cfg = *cfg
	stopped := s.proxy.StopAll()
	msg := "Profile applied"
	if stopped > 0 {
		msg += ", sites restarted on next visit"
	}
	if dbChanged {
		msg += ". Database engine changed — restart Sabdopalon."
	}
	s.json(w, map[string]any{"ok": true, "message": msg})
}

// handleAPIServices reports optional service status (Mailpit).
func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	resp := map[string]any{
		"mailpit_enabled": s.cfg.Services.Mailpit,
	}
	if s.svc != nil {
		resp["mailpit"] = s.svc.Status()
	} else {
		resp["mailpit"] = map[string]any{"installed": false, "running": false}
	}
	s.json(w, resp)
}

// handleAPIMailpitToggle enables/disables Mailpit {enabled}. Applies
// immediately when the service manager is available, and persists to config.
func (s *Server) handleAPIMailpitToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, map[string]string{"error": "invalid JSON"})
		return
	}
	s.cfg.Services.Mailpit = req.Enabled

	var msg string
	switch {
	case req.Enabled && s.svc == nil:
		// Service manager not wired (mailpit disabled at startup) — needs restart.
		msg = "Mailpit enabled in config — it will start after restarting Sabdopalon."
	case req.Enabled:
		if err := s.svc.Start(); err != nil {
			if err := s.cfg.Save(); err != nil {
				s.json(w, map[string]string{"error": err.Error()})
				return
			}
			msg = "Enabled in config, but could not start now: " + err.Error()
		} else {
			msg = "Mailpit started → " + s.svc.Status().UI
		}
	default:
		_ = s.svc.Stop()
		msg = "Mailpit stopped and disabled."
	}

	if err := s.cfg.Save(); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{"ok": true, "message": msg}
	if s.svc != nil {
		resp["mailpit"] = s.svc.Status()
	}
	s.json(w, resp)
}

func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }
