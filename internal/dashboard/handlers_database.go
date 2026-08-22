package dashboard

import (
	"net/http"
	"strings"
)

// handleAPIDatabaseControl starts/stops/restarts a database daemon.
//
//	POST /api/database/{engine}/{action}   engine ∈ mariadb|postgresql
//	POST /api/database/{action}            legacy → primary engine
func (s *Server) handleAPIDatabaseControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.db == nil {
		s.json(w, map[string]string{"error": "database manager not available (setup mode)"})
		return
	}

	// Path after the registered prefix: "/api/database/" + rest.
	rest := strings.TrimPrefix(r.URL.Path, "/api/database/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	var engine, action string
	switch {
	case len(parts) == 2:
		engine, action = parts[0], parts[1]
	case len(parts) == 1:
		engine, action = s.cfg.Database.Engine, parts[0]
	default:
		s.json(w, map[string]string{"error": "usage: /api/database/{engine}/{action}"})
		return
	}
	if action != "start" && action != "stop" && action != "restart" {
		s.json(w, map[string]string{"error": "unknown action (start|stop|restart)"})
		return
	}
	if engine == "sqlite" || engine == "" {
		s.json(w, map[string]string{"error": "sqlite needs no daemon"})
		return
	}
	if !s.db.Enabled(engine) {
		s.json(w, map[string]string{"error": engine + " is disabled — enable it on this page first"})
		return
	}

	switch action {
	case "start":
		if err := s.db.Start(engine); err != nil {
			s.dbErr(w, engine, action, err)
			return
		}
	case "stop":
		if err := s.db.Stop(engine); err != nil {
			s.dbErr(w, engine, action, err)
			return
		}
	case "restart":
		if err := s.db.Restart(engine); err != nil {
			s.dbErr(w, engine, action, err)
			return
		}
	}
	s.json(w, map[string]any{
		"ok":        true,
		"engine":    engine,
		"db_states": s.db.States(),
		"message":   engine + " " + action + " OK",
	})
}

func (s *Server) dbErr(w http.ResponseWriter, engine, action string, err error) {
	s.json(w, map[string]any{
		"error":     err.Error(),
		"engine":    engine,
		"action":    action,
		"db_errors": s.db.Errors(),
	})
}
