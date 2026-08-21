package dashboard

import (
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/sabdopalon/sabdopalon/internal/ssl"
	"github.com/sabdopalon/sabdopalon/internal/trust"
)

// handleAPISSLStatus reports the local PKI + trust state for the SSL page.
func (s *Server) handleAPISSLStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	st := trust.CheckStatus(s.cfg)
	_, httpsPort := s.proxy.Ports()
	resp := map[string]any{
		"ca_exists":         st.CAExists,
		"wildcard_cert":     st.WildcardCert,
		"installed":         st.Installed,
		"fingerprint_match": st.FingerMatch,
		"source":            st.Source,
		"detail":            st.Detail,
		"https_port":        httpsPort,
		"tld":               s.cfg.TLD,
	}
	s.json(w, resp)
}

// handleAPISSLCA generates the root CA (POST).
func (s *Server) handleAPISSLCA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	m := ssl.New(s.cfg)
	if _, err := m.EnsureCA(); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	s.json(w, map[string]any{"ok": true, "message": "Root CA ready."})
}

// handleAPISSLWildcard issues the *.<tld> wildcard certificate (POST).
// The HTTPS listener picks it up after a restart.
func (s *Server) handleAPISSLWildcard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	m := ssl.New(s.cfg)
	wildcard := "*." + s.cfg.TLD
	if err := m.IssueSite(wildcard); err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	cert, _ := m.CertPaths(wildcard)
	s.json(w, map[string]any{
		"ok":      true,
		"message": "Wildcard certificate issued. Restart Sabdopalon to enable HTTPS.",
		"cert":    filepath.Base(cert),
	})
}

// handleAPISSLTrust installs the root CA (POST). Prefers system-wide, falls
// back to per-user stores needing no admin; on failure returns exact manual
// instructions for a copyable dialog.
func (s *Server) handleAPISSLTrust(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	ok, installErr := trust.InstallCAQuiet(s.cfg)
	if !ok {
		msg := "Automatic installation failed. Run this in a terminal:"
		if installErr != nil {
			msg = installErr.Error()
		}
		s.json(w, map[string]any{
			"ok":       false,
			"needs_su": true,
			"message":  msg,
			"command":  trust.ManualCommand(s.cfg),
			"note":     firefoxNote(),
		})
		return
	}
	where := ""
	if st := trust.CheckStatus(s.cfg); st.Source == "user" {
		where = " (per-user store — no admin rights needed)"
	}
	s.json(w, map[string]any{
		"ok":      true,
		"message": "Root CA trusted" + where + " — restart your browser and reload.",
		"note":    firefoxNote(),
	})
}

func firefoxNote() string {
	if runtime.GOOS == "linux" {
		return "Firefox uses its own certificate store: Settings → Privacy → Certificates → View Certificates → Authorities → Import certs/sabdopalon-rootCA.crt."
	}
	return ""
}
