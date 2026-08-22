package dashboard

import (
	"net/http"
)

// handleAPIStatsTraffic reports proxy request counters (per-minute window)
// for the dashboard traffic chart.
func (s *Server) handleAPIStatsTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	if s.proxy == nil {
		s.json(w, map[string]any{"total": 0, "per_minute": []any{}})
		return
	}
	s.json(w, s.proxy.Stats())
}
