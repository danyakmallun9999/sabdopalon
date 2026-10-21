// Package dashboard — handlers_hosts.go: the two endpoints behind the Settings
// page's "Local DNS" row.
//
//	GET  /api/hosts          → read-only state: what is mapped, what is missing
//	POST /api/hosts/elevate  → one UAC approval, then the CLI's sync runs elevated
//
// They are separate on purpose. Reading the state never asks for rights, so the
// page can show a correct report on a machine where the user has decided against
// approving elevation at all; only the explicit button can raise a prompt.
package dashboard

import (
	"net/http"
)

// hostsStatePayload is the wire form of hostsync.State. Slices are flattened to
// empty arrays (never null) so the page can iterate without a guard, and "ok"
// is computed here rather than in TypeScript so one definition of "in step"
// exists.
type hostsStatePayload struct {
	TLD           string   `json:"tld"`
	Path          string   `json:"path"`
	SelfResolving bool     `json:"self_resolving"`
	Managed       bool     `json:"managed"`
	Wanted        []string `json:"wanted"`
	Current       []string `json:"current"`
	Missing       []string `json:"missing"`
	Stale         []string `json:"stale"`
	OK            bool     `json:"ok"`
	// can_elevate tells the page whether a button is worth showing: false on
	// systems where only a sudo command can do the job.
	CanElevate bool `json:"can_elevate"`
	// error is set when the file could not be read; the page shows it instead
	// of a green report.
	Error string `json:"error,omitempty"`
}

// handleAPIHosts serves GET /api/hosts.
func (s *Server) handleAPIHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	st, err := s.HostsState()
	payload := hostsStatePayload{
		TLD:           st.TLD,
		Path:          st.Path,
		SelfResolving: st.SelfResolving,
		Managed:       st.Managed,
		Wanted:        orEmpty(st.Wanted),
		Current:       orEmpty(st.Current),
		Missing:       orEmpty(st.Missing),
		Stale:         orEmpty(st.Stale),
		OK:            st.OK(),
		CanElevate:    elevationSupported(),
	}
	if err != nil {
		payload.Error = err.Error()
	}
	s.json(w, payload)
}

// handleAPIHostsElevate serves POST /api/hosts/elevate.
//
// The response is an acknowledgement, not a result: the elevated process runs
// outside this one, so the page re-reads /api/hosts once the prompt closes.
// Nothing but a hosts-file sync is ever started this way.
func (s *Server) handleAPIHostsElevate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	started, message := s.RequestHostsElevation()
	s.json(w, map[string]any{
		"ok":      started || message != "", // false only when the spawn failed
		"started": started,
		"message": message,
	})
}

// orEmpty keeps JSON iterable: a nil slice marshals as null, and a page that
// does missing.length would throw on it.
func orEmpty(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
