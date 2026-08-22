// Package dashboard implements Sabdopalon's web control panel: a JSON API
// plus a multi-page HTML UI (embedded via go:embed) for managing sites,
// packages, SSL, settings, backups, logs and services — everything the CLI
// can do, from the browser.
//
// Layout of this package:
//
//	server.go            routing + shared helpers
//	handlers_sites.go    site CRUD / start-stop-restart
//	handlers_packages.go package installs with progress jobs
//	handlers_ssl.go      local CA wizard + trust status
//	handlers_config.go   engine settings + profiles
//	handlers_backup.go   database backups
//	ui.go                template loading (go:embed)
//	web/                 HTML/CSS/JS sources
package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/backup"
	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
	"github.com/sabdopalon/sabdopalon/internal/services"
)

// Version is shown in the UI header; set by the app package on startup.
var Version = "dev"

// Server is the dashboard HTTP server.
type Server struct {
	cfg     *config.Engine
	proxy   *proxy.Server
	backup  *backup.Manager
	svc     *services.Manager // nil when no optional service is enabled
	mux     *http.ServeMux
	started time.Time
}

// New creates a dashboard Server. svc may be nil.
func New(cfg *config.Engine, px *proxy.Server, bk *backup.Manager, svc *services.Manager) *Server {
	s := &Server{
		cfg:     cfg,
		proxy:   px,
		backup:  bk,
		svc:     svc,
		mux:     http.NewServeMux(),
		started: time.Now(),
	}
	s.routes()
	return s
}

// Start launches the dashboard. Blocks.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.cfg.Dashboard.Port)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	// JSON API (used by the React SPA and handy for scripting).
	s.mux.HandleFunc("/api/status", s.handleAPIStatus)

	// API: sites
	s.mux.HandleFunc("/api/sites", s.handleAPISites)
	s.mux.HandleFunc("/api/sites/", s.handleAPISiteAction)

	// API: packages
	s.mux.HandleFunc("/api/packages", s.handleAPIPackages)
	s.mux.HandleFunc("/api/packages/install", s.handleAPIPackageInstall)
	s.mux.HandleFunc("/api/packages/job", s.handleAPIPackageJob)

	// API: ssl
	s.mux.HandleFunc("/api/ssl", s.handleAPISSLStatus)
	s.mux.HandleFunc("/api/ssl/ca", s.handleAPISSLCA)
	s.mux.HandleFunc("/api/ssl/wildcard", s.handleAPISSLWildcard)
	s.mux.HandleFunc("/api/ssl/trust", s.handleAPISSLTrust)

	// API: config & profiles
	s.mux.HandleFunc("/api/config", s.handleAPIConfig)
	s.mux.HandleFunc("/api/profiles", s.handleAPIProfiles)
	s.mux.HandleFunc("/api/profiles/apply", s.handleAPIProfileApply)

	// API: system PHP discovery
	s.mux.HandleFunc("/api/php/system", s.handleAPISystemPHP)

	// API: services
	s.mux.HandleFunc("/api/services", s.handleAPIServices)
	s.mux.HandleFunc("/api/services/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/toggle"), "/api/services/")
		if name == "" || strings.Contains(name, "/") {
			http.NotFound(w, r)
			return
		}
		s.handleAPIServiceToggle(w, name, r)
	})

	// API: backups
	s.mux.HandleFunc("/api/backup", s.handleAPIBackup)
	s.mux.HandleFunc("/api/backups", s.handleAPIBackups)

	// API: logs
	s.mux.HandleFunc("/api/logs/", s.handleAPILogs)

	// Unknown API path → JSON 404 (registered last, /api/* falls through).
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"unknown api endpoint"}`))
	})

	// Everything else → React SPA (client-side routing).
	uiHandler := s.spaHandler()
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		uiHandler.ServeHTTP(w, r)
	})
}

// siteURLs builds friendly URLs for a site using the actually-bound ports.
func (s *Server) siteURLs(name string) (string, string) {
	httpPort, httpsPort := s.proxy.Ports()
	host := name + "." + s.cfg.TLD
	if httpPort == 80 {
		httpPort = 0
	}
	if httpsPort == 443 {
		httpsPort = 0
	}
	u := fmt.Sprintf("http://%s", host)
	if httpPort != 0 {
		u = fmt.Sprintf("%s:%d", u, httpPort)
	}
	h := fmt.Sprintf("https://%s", host)
	if httpsPort != 0 {
		h = fmt.Sprintf("%s:%d", h, httpsPort)
	}
	return u + "/", h + "/"
}

// --- helpers ---

func (s *Server) json(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(v)
}

func (s *Server) methodNotAllowed(w http.ResponseWriter, want string) {
	w.Header().Set("Allow", want)
	s.json(w, map[string]string{"error": "method not allowed, use " + want})
}

// baseName returns the final path element.
func baseName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[i+1:]
		}
	}
	return p
}

// svcRunning reports whether any optional managed service is running.
func (s *Server) svcRunning() bool { return s.svc != nil && s.svc.AnyRunning() }

// readBody reads and limits a request body (kept for future endpoints).
func readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r.Body, 1<<20))
}
