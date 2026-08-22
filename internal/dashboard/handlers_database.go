package dashboard

import (
	"net/http"
)

// handleAPIDatabaseControl starts/stops/restarts the database daemon.
// Route: POST /api/database/<action> where action ∈ start|stop|restart.
// SQLite is a no-op (always "ready").
func (s *Server) handleAPIDatabaseControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.db == nil {
		s.json(w, map[string]string{"error": "database manager not available (setup mode)"})
		return
	}
	action := r.PathValue("action")
	switch action {
	case "start":
		if err := s.db.Start(); err != nil {
			s.json(w, map[string]string{"error": err.Error()})
			return
		}
	case "stop":
		if err := s.db.Stop(); err != nil {
			s.json(w, map[string]string{"error": err.Error()})
			return
		}
	case "restart":
		if err := s.db.Restart(); err != nil {
			s.json(w, map[string]string{"error": err.Error()})
			return
		}
	default:
		s.json(w, map[string]string{"error": "unknown action (start|stop|restart)"})
		return
	}
	s.json(w, map[string]any{
		"ok":        true,
		"engine":    s.cfg.Database.Engine,
		"db_running": s.db.Ready(),
		"message":   "database " + action + " OK",
	})
}
