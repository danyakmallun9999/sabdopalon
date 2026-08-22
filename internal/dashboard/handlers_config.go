package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
	"github.com/sabdopalon/sabdopalon/internal/profiles"
	"github.com/sabdopalon/sabdopalon/internal/services"
)

// handleAPIStatus reports overall system state for the header/status card.
func (s *Server) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	httpPort, httpsPort := s.proxy.Ports()
	dbRunning := false
	if s.db != nil {
		dbRunning = s.db.Ready(s.cfg.Database.Engine)
	} else if s.cfg.Database.Engine == "sqlite" || s.cfg.Database.Engine == "" {
		dbRunning = true // sqlite is always available
	}
	resp := map[string]any{
		"version":     Version,
		"uptime":      time.Since(s.started).Round(time.Second).String(),
		"http_port":   httpPort,
		"https_port":  httpsPort,
		"low_ports":   s.proxy.LowPortsBound(),
		"tld":         s.cfg.TLD,
		"database":    s.cfg.Database.Engine,
		"db_running":  dbRunning,
		"sites_count": len(s.proxy.RunningSites()),
		"services":    s.svcRunning(),
	}
	if s.db != nil {
		resp["db_states"] = s.db.States()
		if errs := s.db.Errors(); len(errs) > 0 {
			resp["db_errors"] = errs
		}
	}
	if s.cfg.PHP.Binary != "" {
		resp["php"] = baseName(s.cfg.PHP.Binary)
		resp["php_version"] = pkgmgr.PHPBinaryVersion(s.cfg.PHP.Binary)
	}
	s.json(w, resp)
}

// handleAPISystemPHP lists PHP CLIs found on the host (outside bin/).
func (s *Server) handleAPISystemPHP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	cands := pkgmgr.SystemPHPCandidates()
	for i := range cands {
		cands[i].Active = samePathLocal(cands[i].Path, s.cfg.PHP.Binary)
	}
	s.json(w, cands)
}

// samePathLocal compares two paths after symlink/clean resolution.
func samePathLocal(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, e1 := filepath.EvalSymlinks(a)
	rb, e2 := filepath.EvalSymlinks(b)
	if e1 != nil || e2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

type configPayload struct {
	TLD       string `json:"tld,omitempty"`
	HTTPPort  *int   `json:"http_port,omitempty"`
	HTTPSPort *int   `json:"https_port,omitempty"`
	DBEngine  string `json:"db_engine,omitempty"`
	DBPort    *int   `json:"db_port,omitempty"`
	// Multi-daemon: per-engine switches + live state snapshots.
	DBMariadbEnabled *bool             `json:"db_mariadb_enabled,omitempty"`
	DBMariadbPort    *int              `json:"db_mariadb_port,omitempty"`
	DBPgEnabled      *bool             `json:"db_pg_enabled,omitempty"`
	DBPgPort         *int              `json:"db_pg_port,omitempty"`
	DBInstalled      map[string]bool   `json:"db_installed,omitempty"`
	DBRunning        bool              `json:"db_running"`
	DBStates         map[string]bool   `json:"db_states,omitempty"`
	DBErrors         map[string]string `json:"db_errors,omitempty"`
	DashEnabled      *bool             `json:"dashboard_enabled,omitempty"`
	DashPort         *int              `json:"dashboard_port,omitempty"`
	AutoOpen         *bool             `json:"auto_open,omitempty"`
	Mailpit          *bool             `json:"mailpit_enabled,omitempty"`
}

// handleAPIConfig serves GET/PUT /api/config — the Settings page.
// PUT persists to config/engine.toml; structural changes require a restart.
func (s *Server) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		httpPort, httpsPort := s.proxy.Ports()
		dbRunning := false
		if s.cfg.Database.Engine == "sqlite" || s.cfg.Database.Engine == "" {
			dbRunning = true // sqlite is always available
		} else if s.db != nil {
			dbRunning = s.db.Ready(s.cfg.Database.Engine)
		}
		payload := configPayload{
			TLD:              s.cfg.TLD,
			HTTPPort:         intPtr(orActual(httpPort, s.cfg.Proxy.HTTPPort)),
			HTTPSPort:        intPtr(orActual(httpsPort, s.cfg.Proxy.HTTPSPort)),
			DBEngine:         s.cfg.Database.Engine,
			DBPort:           intPtr(s.cfg.Database.Port),
			DBMariadbEnabled: boolPtr(s.cfg.Database.MariaDBEnabled),
			DBMariadbPort:    intPtr(s.cfg.Database.MariaDBPort),
			DBPgEnabled:      boolPtr(s.cfg.Database.PGEnabled),
			DBPgPort:         intPtr(s.cfg.Database.PGPort),
			DashEnabled:      boolPtr(s.cfg.Dashboard.Enabled),
			DashPort:         intPtr(s.cfg.Dashboard.Port),
			AutoOpen:         boolPtr(s.cfg.Dashboard.AutoOpen),
			Mailpit:          boolPtr(s.cfg.Services.Mailpit),
			DBRunning:        dbRunning,
		}
		inst := map[string]bool{}
		for _, eng := range []string{"sqlite", "mariadb", "postgresql"} {
			inst[eng] = database.Installed(s.cfg, eng)
		}
		payload.DBInstalled = inst
		if s.db != nil {
			payload.DBStates = s.db.States()
			payload.DBErrors = s.db.Errors()
		}
		s.json(w, payload)

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
		// Multi-daemon: enable/disable applies LIVE (daemon starts/stops
		// immediately); a port change needs a daemon restart.
		if p.DBMariadbEnabled != nil && *p.DBMariadbEnabled != s.cfg.Database.MariaDBEnabled {
			s.cfg.Database.MariaDBEnabled = *p.DBMariadbEnabled
			if s.db != nil {
				if *p.DBMariadbEnabled {
					go func() { _ = s.db.Start("mariadb") }()
				} else {
					_ = s.db.Stop("mariadb")
				}
			}
		}
		if p.DBMariadbPort != nil && validPort(*p.DBMariadbPort) && *p.DBMariadbPort != s.cfg.Database.MariaDBPort {
			s.cfg.Database.MariaDBPort = *p.DBMariadbPort
			if s.db != nil && s.db.Ready("mariadb") {
				restart = true
			}
		}
		if p.DBPgEnabled != nil && *p.DBPgEnabled != s.cfg.Database.PGEnabled {
			s.cfg.Database.PGEnabled = *p.DBPgEnabled
			if s.db != nil {
				if *p.DBPgEnabled {
					go func() { _ = s.db.Start("postgresql") }()
				} else {
					_ = s.db.Stop("postgresql")
				}
			}
		}
		if p.DBPgPort != nil && validPort(*p.DBPgPort) && *p.DBPgPort != s.cfg.Database.PGPort {
			s.cfg.Database.PGPort = *p.DBPgPort
			if s.db != nil && s.db.Ready("postgresql") {
				restart = true
			}
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

// handleAPIServices lists every optional managed service with live state.
func (s *Server) handleAPIServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	// Always report the full registry, even when no service is enabled yet
	// (svcMgr is nil then) — the dashboard shows every service with its state.
	mgr := s.svc
	if mgr == nil {
		mgr = services.New(s.cfg)
	}
	s.json(w, map[string]any{"services": mgr.All()})
}

// handleAPIServiceToggle enables/disables a service {enabled}. Persists to
// engine.toml and applies immediately when the service manager is wired.
// Route: POST /api/services/<name>/toggle
func (s *Server) handleAPIServiceToggle(w http.ResponseWriter, name string, r *http.Request) {
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
	if !services.SetEnabled(s.cfg, name, req.Enabled) {
		s.json(w, map[string]string{"error": "unknown service: " + name})
		return
	}

	var msg string
	switch {
	case req.Enabled && s.svc == nil:
		msg = fmt.Sprintf("%s enabled in config — it will start after restarting Sabdopalon.", name)
	case req.Enabled:
		if err := s.cfg.Save(); err != nil {
			s.json(w, map[string]string{"error": err.Error()})
			return
		}
		if err := s.svc.Start(name); err != nil {
			msg = "Enabled in config, but could not start now: " + err.Error()
		} else if st := s.svc.Status(name); st.UI != "" {
			msg = name + " started → " + st.UI
		} else {
			msg = name + " started."
		}
	default:
		_ = s.svc.Stop(name)
		msg = name + " stopped and disabled."
	}

	if err := s.cfg.Save(); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{"ok": true, "message": msg}
	if s.svc != nil {
		resp["status"] = s.svc.Status(name)
	}
	s.json(w, resp)
}

// handleAPIServiceStart starts a service NOW without touching its config
// enable flag (runtime-only). Route: POST /api/services/<name>/start
func (s *Server) handleAPIServiceStart(w http.ResponseWriter, name string, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.svc == nil {
		s.json(w, map[string]string{"error": "service manager not available (no service enabled in config)"})
		return
	}
	if err := s.svc.Start(name); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{"ok": true, "message": name + " started.", "status": s.svc.Status(name)}
	if st := s.svc.Status(name); st.UI != "" {
		resp["message"] = name + " started → " + st.UI
	}
	s.json(w, resp)
}

// handleAPIServiceStop stops a running service NOW (runtime-only, no config
// change). Route: POST /api/services/<name>/stop
func (s *Server) handleAPIServiceStop(w http.ResponseWriter, name string, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.svc == nil {
		s.json(w, map[string]string{"error": "service manager not available"})
		return
	}
	_ = s.svc.Stop(name)
	s.json(w, map[string]any{"ok": true, "message": name + " stopped.", "status": s.svc.Status(name)})
}

func intPtr(n int) *int    { return &n }
func boolPtr(b bool) *bool { return &b }
