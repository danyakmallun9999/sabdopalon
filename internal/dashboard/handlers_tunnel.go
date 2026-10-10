package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/sabdopalon/sabdopalon/internal/tunnel"
)

// handleAPISiteTunnelStart shares a site publicly through a Cloudflare Quick
// Tunnel.
//
// POST /api/sites/{name}/tunnel — body { "allowed_mail": ["a@b.com"] } optional.
// Returns immediately: the public URL is produced a moment later and reaches the
// UI through the SSE status snapshot, which every page already subscribes to.
func (s *Server) handleAPISiteTunnelStart(w http.ResponseWriter, name string, r *http.Request) {
	if s.tn == nil {
		s.json(w, map[string]string{"error": "tunnels are unavailable in this mode"})
		return
	}
	var body struct {
		AllowedMail []string `json:"allowed_mail"`
	}
	if r.Body != nil {
		// No body (or an unparseable one) simply means "no email restriction".
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	// A shared site has to be up: the tunnel points at the proxy, and the proxy
	// answers 503 for a site the user stopped. StartSite re-enables it and is
	// idempotent, so this is a no-op for a site that is already running.
	if _, err := s.proxy.StartSite(name); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}

	st, err := s.tn.Start(name, body.AllowedMail)
	if err != nil {
		s.json(w, map[string]any{
			"error": err.Error(),
			// The install command gets its own field: it is the only thing the
			// user has to do, and it differs per platform.
			"hint":  tunnel.InstallHint(),
			"state": st,
		})
		return
	}
	s.json(w, st)
}

// handleAPISiteTunnelState reports one site's tunnel.
//
// GET /api/sites/{name}/tunnel
func (s *Server) handleAPISiteTunnelState(w http.ResponseWriter, name string) {
	if s.tn == nil {
		s.json(w, tunnel.State{Status: tunnel.StatusOff})
		return
	}
	st := s.tn.State(name)
	if len(st.AllowedMail) == 0 {
		// Pre-fill with what the user typed the last time they shared this site.
		st.AllowedMail = s.tn.SavedMail(name)
	}
	s.json(w, st)
}

// handleAPISiteTunnelStop closes a site's tunnel.
//
// DELETE /api/sites/{name}/tunnel
//
// This has to be claimed inside handleAPISiteAction before its DELETE branch:
// that branch moves a site to .trash, so an unclaimed "tunnel" DELETE would
// delete the site instead of unsharing it.
func (s *Server) handleAPISiteTunnelStop(w http.ResponseWriter, name string) {
	if s.tn == nil {
		s.json(w, tunnel.State{Status: tunnel.StatusOff})
		return
	}
	s.json(w, s.tn.Stop(name))
}
