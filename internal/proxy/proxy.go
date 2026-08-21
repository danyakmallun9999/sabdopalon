// Package proxy implements Sabdopalon's multiplexing reverse proxy.
//
// One Go HTTP server listens on a single port (default :8080). It inspects the
// Host header of every request, maps it to a project (site folder), and
// proxies the request to that project's PHP built-in server, which runs on a
// per-project dynamically assigned port.
//
// This replaces Apache/Nginx entirely: no web server binaries to download,
// no vhost config files to maintain, and `.localhost` resolves automatically
// on modern OSes (Linux, macOS, Windows 10+) without editing /etc/hosts.
package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/pkgmgr"
	"github.com/sabdopalon/sabdopalon/internal/siteconfig"
	"github.com/sabdopalon/sabdopalon/internal/vhost"
)

// Server is the multiplexing proxy that routes *.localhost to per-site PHP.
type Server struct {
	cfg      *config.Engine
	mu       sync.Mutex
	sites    map[string]*siteServer // host -> running PHP server for that site
	disabled map[string]bool        // sites stopped from the dashboard (in-memory)
	portNext int
	stopCh   chan struct{}
	aliases  map[string]string // alias hostname -> canonical site name
	Verbose  bool              // print per-site start/stop events

	// Actual bound ports after low-port auto-attempt (may differ from config).
	httpPortActual  int
	httpsPortActual int
	lowPortsBound   bool
}

type siteServer struct {
	host    string
	name    string // site folder name
	dir     string // document root
	port    int
	proxy   *httputil.ReverseProxy
	php     *managedPHP
	logFile *os.File
}

// New creates a proxy Server.
func New(cfg *config.Engine) *Server {
	return &Server{
		cfg:      cfg,
		sites:    map[string]*siteServer{},
		disabled: map[string]bool{},
		aliases:  map[string]string{},
		portNext: 9001,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the HTTP and (optionally) HTTPS proxy. Blocks until stopped.
func (s *Server) Start() error {
	// Ensure the router script exists alongside each site.
	s.ensureRouters()
	s.buildAliases()

	// Try low ports (:80/:443) first for clean URLs; fall back to the
	// configured ports if not permitted (needs root or setcap).
	httpPort := s.cfg.Proxy.HTTPPort
	if canBind(80) {
		httpPort = 80
		s.lowPortsBound = true
	}
	s.httpPortActual = httpPort

	errCh := make(chan error, 4)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", httpPort),
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	if s.Verbose {
		fmt.Printf("  ✦  Sabdopalon proxy on http://localhost:%d  (.*.%s → PHP)\n", httpPort, s.cfg.TLD)
	}

	// Optional HTTPS listener (low port preferred as well).
	if s.startHTTPS(errCh) && s.Verbose {
		fmt.Printf("  🔒  HTTPS on https://localhost:%d\n", s.httpsPortActual)
	}

	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-s.stopCh:
		_ = srv.Close()
		return nil
	}
}

// Ports returns the actually-bound HTTP and HTTPS ports.
func (s *Server) Ports() (int, int) {
	return s.httpPortActual, s.httpsPortActual
}

// LowPortsBound reports whether :80/:443 were successfully bound.
func (s *Server) LowPortsBound() bool { return s.lowPortsBound }

// startHTTPS launches an HTTPS listener if a wildcard cert for *.<tld> exists.
// Returns true if HTTPS was started. TLS handshake rejections from browsers
// that don't trust the local CA yet are logged once instead of per-request.
func (s *Server) startHTTPS(errCh chan<- error) bool {
	caCert := filepath.Join(s.cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	// Look for a wildcard or single cert covering the TLD.
	wildcard := "*." + s.cfg.TLD
	certPath := filepath.Join(s.cfg.RootDir, "certs", wildcard+".crt")
	keyPath := filepath.Join(s.cfg.RootDir, "certs", wildcard+".key")
	if !fileExists(certPath) || !fileExists(keyPath) {
		// fall back to localhost cert
		certPath = filepath.Join(s.cfg.RootDir, "certs", "localhost.crt")
		keyPath = filepath.Join(s.cfg.RootDir, "certs", "localhost.key")
		if !fileExists(certPath) || !fileExists(keyPath) {
			s.httpsPortActual = s.cfg.Proxy.HTTPSPort
			return false
		}
	}
	_ = caCert // referenced for clarity; trust store install is separate

	httpsPort := s.cfg.Proxy.HTTPSPort
	if s.lowPortsBound && canBind(443) {
		httpsPort = 443
	}
	s.httpsPortActual = httpsPort

	httpsAddr := fmt.Sprintf(":%d", httpsPort)
	quietLog := log.New(&handshakeFilter{next: os.Stderr}, "", log.LstdFlags)
	httpsSrv := &http.Server{
		Addr:         httpsAddr,
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		ErrorLog:     quietLog,
	}
	go func() {
		errCh <- httpsSrv.ListenAndServeTLS(certPath, keyPath)
	}()
	return true
}

// handshakeFilter suppresses repetitive client-side TLS handshake errors
// (e.g. "remote error: tls: unknown certificate authority") which occur on
// every request until the user trusts the local CA. Other errors pass through.
type handshakeFilter struct {
	next  io.Writer
	shown map[string]bool
	mu    sync.Mutex
}

func (f *handshakeFilter) Write(p []byte) (int, error) {
	msg := string(p)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.shown == nil {
		f.shown = map[string]bool{}
	}
	if strings.Contains(msg, "TLS handshake error") {
		if f.shown["tls"] {
			return len(p), nil // already warned once — swallow
		}
		f.shown["tls"] = true
		hint := msg
		if strings.Contains(msg, "unknown certificate authority") {
			hint += "  ⚠  Browser does not trust the Sabdopalon CA yet — open the dashboard → SSL → Trust CA.\n"
		}
		return f.next.Write([]byte(hint))
	}
	return f.next.Write(p)
}

// Stop signals the proxy to shut down and kills all PHP servers.
func (s *Server) Stop() {
	s.StopAll()
	close(s.stopCh)
}

// ServeHTTP routes by Host header to the matching site's PHP server.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := normalizeHost(r.Host)
	siteName, ok := s.hostToSite(host)
	if !ok {
		s.dashboardFallback(w, r, host)
		return
	}
	// Sites stopped from the dashboard stay down: no lazy restart until the
	// user explicitly starts them again.
	if s.isStopped(siteName) {
		s.serveStoppedPage(w, siteName)
		return
	}
	ss, err := s.ensureSite(siteName)
	if err != nil {
		http.Error(w, fmt.Sprintf("sabdopalon: cannot start PHP for %s: %v", siteName, err), http.StatusBadGateway)
		return
	}
	ss.proxy.ServeHTTP(w, r)
}

// isStopped reports whether the site was stopped via the dashboard.
func (s *Server) isStopped(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.disabled[name]
}

// serveStoppedPage renders a friendly 503 so visitors understand why the site
// is down (and that it is intentional, not broken).
func (s *Server) serveStoppedPage(w http.ResponseWriter, name string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusServiceUnavailable)
	dash := fmt.Sprintf("http://localhost:%d/sites", s.cfg.Dashboard.Port)
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>%s — stopped</title>
<style>body{font-family:system-ui,-apple-system,sans-serif;background:#0d1117;color:#e6edf3;display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0}
.c{max-width:460px;text-align:center;padding:2.5rem;border:1px solid #30363d;border-radius:14px}
h1{font-size:1.3rem;margin:0 0 .5rem}p{color:#8b949e;line-height:1.6;margin:.5rem 0}
a{color:#58a6ff;text-decoration:none}a:hover{text-decoration:underline}</style></head>
<body><div class="c"><h1>🛑 %s is stopped</h1>
<p>This site was stopped from the Sabdopalon dashboard and stays down until you start it again.</p>
<p><a href="%s">Start it in the dashboard →</a></p></div></body></html>`, name, name, dash)
}

// hostToSite maps "example-app.localhost" -> "example-app".
func (s *Server) hostToSite(host string) (string, bool) {
	tld := s.cfg.TLD
	// bare localhost / 127.0.0.1 → dashboard
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "", false
	}
	// per-site aliases (.sabdopalon.yml) take precedence
	s.mu.Lock()
	if name, ok := s.aliases[strings.ToLower(host)]; ok {
		s.mu.Unlock()
		return name, true
	}
	s.mu.Unlock()
	suffix := "." + tld
	if !strings.HasSuffix(host, suffix) {
		return "", false
	}
	name := strings.TrimSuffix(host, suffix)
	if name == "" {
		return "", false
	}
	// verify site folder exists
	docroot := filepath.Join(s.cfg.Root, name, "public")
	if _, err := os.Stat(docroot); err != nil {
		docroot = filepath.Join(s.cfg.Root, name)
		if _, err := os.Stat(docroot); err != nil {
			return "", false
		}
	}
	return name, true
}

// buildAliases pre-scans all sites for .sabdopalon.yml aliases so extra
// domains can route to a project without editing /etc/hosts.
func (s *Server) buildAliases() {
	names, err := discoverSites(s.cfg)
	if err != nil {
		return
	}
	m := map[string]string{}
	for _, n := range names {
		sc, err := siteconfig.Load(s.cfg.Root, n)
		if err != nil || len(sc.Aliases) == 0 {
			continue
		}
		for _, a := range sc.Aliases {
			a = strings.ToLower(strings.TrimSpace(a))
			if a == "" {
				continue
			}
			if !strings.Contains(a, ".") { // bare word → append default TLD
				a = a + "." + s.cfg.TLD
			}
			m[a] = n
		}
	}
	s.mu.Lock()
	s.aliases = m
	s.mu.Unlock()
}

// ensureSite lazily starts a PHP built-in server for the given site.
func (s *Server) ensureSite(name string) (*siteServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host := name + "." + s.cfg.TLD
	if ss, ok := s.sites[host]; ok {
		return ss, nil
	}

	// Per-site overrides from sites/<name>/.sabdopalon.yml (optional).
	sc, scErr := siteconfig.Load(s.cfg.Root, name)
	docroot := filepath.Join(s.cfg.Root, name, "public")
	if _, err := os.Stat(docroot); err != nil {
		docroot = filepath.Join(s.cfg.Root, name)
	}
	if scErr == nil && sc.Docroot != "" {
		overridden := filepath.Join(s.cfg.Root, name, filepath.FromSlash(sc.Docroot))
		if _, err := os.Stat(overridden); err == nil {
			docroot = overridden
		} else {
			fmt.Printf("  ⚠  %s: docroot override %q not found — using default\n", name, sc.Docroot)
		}
	}

	// Resolve the PHP binary: per-site version/binary wins over global default.
	phpBin := s.cfg.PHP.Binary
	if scErr == nil && sc.PHP != "" {
		resolved, err := pkgmgr.ResolvePHP(filepath.Join(s.cfg.RootDir, "bin"), sc.PHP)
		if err != nil {
			return nil, fmt.Errorf("%s wants PHP %s: %w", name, sc.PHP, err)
		}
		phpBin = resolved
	}

	// Ensure the router script exists for this site (handles sites created
	// while the proxy is already running).
	routerPath := filepath.Join(s.cfg.Root, name, ".sabdopalon-router.php")
	if !fileExists(routerPath) {
		_ = os.WriteFile(routerPath, []byte(defaultRouter), 0o644)
	}

	port := s.portNext
	for !isPortFree(port) {
		port++
	}
	s.portNext = port + 1

	if err := os.MkdirAll(s.cfg.Logs, 0o755); err != nil {
		return nil, err
	}
	logPath := filepath.Join(s.cfg.Logs, name+".php.log")
	lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}

	extraEnv := []string{}
	if scErr == nil {
		for k, v := range sc.Env {
			extraEnv = append(extraEnv, fmt.Sprintf("%s=%s", k, v))
		}
	}
	sort.Strings(extraEnv)

	php, err := startPHP(phpBin, port, docroot, lf, s.cfg.Database.Engine, s.cfg.Database.Path, extraEnv)
	if err != nil {
		lf.Close()
		return nil, err
	}
	if !waitForPort(port, 3*time.Second) {
		lf.Close()
		_ = php.stop()
		return nil, fmt.Errorf("php did not start on :%d (see %s)", port, logPath)
	}

	target := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", port)}
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Director = func(r *http.Request) {
		r.URL.Scheme = target.Scheme
		r.URL.Host = target.Host
		r.Host = host
	}

	ss := &siteServer{
		host:    host,
		name:    name,
		dir:     docroot,
		port:    port,
		proxy:   rp,
		php:     php,
		logFile: lf,
	}
	s.sites[host] = ss
	if s.Verbose {
		fmt.Printf("  ▶  %s → php :%d  (%s)\n", host, port, docroot)
	}
	return ss, nil
}

// StartSite pre-warms (starts) the PHP server for a named site.
// Used by the dashboard so users can start sites without waiting for a request.
func (s *Server) StartSite(name string) (*SiteInfo, error) {
	s.mu.Lock()
	delete(s.disabled, name) // an explicit start always re-enables the site
	s.mu.Unlock()
	ss, err := s.ensureSite(name)
	if err != nil {
		return nil, err
	}
	return &SiteInfo{Host: ss.host, Dir: ss.dir, Port: ss.port}, nil
}

// StopSite stops one site's PHP server and keeps it down (requests receive a
// friendly 503 instead of lazily restarting PHP). Returns true if it was running.
func (s *Server) StopSite(name string) bool {
	host := name + "." + s.cfg.TLD
	s.mu.Lock()
	ss, ok := s.sites[host]
	if ok {
		_ = ss.php.stop()
		if ss.logFile != nil {
			_ = ss.logFile.Close()
		}
		delete(s.sites, host)
	}
	s.disabled[name] = true
	verbose := s.Verbose
	port := 0
	if ok {
		port = ss.port
	}
	s.mu.Unlock()
	if verbose {
		fmt.Printf("  ◾  stopped %s (php :%d)\n", host, port)
	}
	return ok
}

// RestartSite restarts one site's PHP server (picks up code-level changes
// that require a fresh process).
func (s *Server) RestartSite(name string) error {
	s.StopSite(name)
	_, err := s.StartSite(name)
	return err
}

// Enable clears the stopped flag for a site without starting PHP (used when
// a site is deleted so a future site with the same name starts fresh).
func (s *Server) Enable(name string) {
	s.mu.Lock()
	delete(s.disabled, name)
	s.mu.Unlock()
}

// IsRunning reports whether a site's PHP server is currently up.
func (s *Server) IsRunning(name string) bool {
	host := name + "." + s.cfg.TLD
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.sites[host]
	return ok
}

// StopAll terminates all per-site PHP servers.
func (s *Server) StopAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for host, ss := range s.sites {
		_ = ss.php.stop()
		if ss.logFile != nil {
			_ = ss.logFile.Close()
		}
		if s.Verbose {
			fmt.Printf("  ◾  stopped %s (php :%d)\n", host, ss.port)
		}
		delete(s.sites, host)
		n++
	}
	return n
}

// RunningSites returns info about currently running per-site PHP servers.
func (s *Server) RunningSites() []SiteInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SiteInfo, 0, len(s.sites))
	for _, ss := range s.sites {
		out = append(out, SiteInfo{Host: ss.host, Dir: ss.dir, Port: ss.port})
	}
	return out
}

// SiteInfo describes a running site.
type SiteInfo struct {
	Host string
	Dir  string
	Port int
}

// ensureRouters creates a default PHP router script in each site folder if
// none exists. The router serves existing static files directly and falls back
// to index.php (front-controller pattern) for everything else.
func (s *Server) ensureRouters() {
	entries, err := os.ReadDir(s.cfg.Root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		routerPath := filepath.Join(s.cfg.Root, e.Name(), ".sabdopalon-router.php")
		if !fileExists(routerPath) {
			_ = os.WriteFile(routerPath, []byte(defaultRouter), 0o644)
		}
	}
}

const defaultRouter = `<?php
// Sabdopalon default PHP router — serves static files, falls back to index.php.
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$docroot = $_SERVER['DOCUMENT_ROOT'];
$file = $docroot . $uri;
if ($uri !== '/' && is_file($file)) {
    return false; // let PHP's built-in server serve the static file
}
$index = $docroot . '/index.php';
if (is_file($index)) {
    require $index;
    return true;
}
http_response_code(404);
echo "404 Not Found — create index.php or the requested file in " . $docroot;
return true;
`

// dashboardFallback routes bare localhost traffic to the real dashboard.
// When the dashboard is disabled a minimal site index is rendered instead.
func (s *Server) dashboardFallback(w http.ResponseWriter, r *http.Request, requestedHost string) {
	if s.cfg.Dashboard.Enabled {
		http.Redirect(w, r, fmt.Sprintf("http://localhost:%d/", s.cfg.Dashboard.Port), http.StatusFound)
		return
	}
	sites, _ := vhost.Scan(s.cfg)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8"><title>Sabdopalon</title></head>
<body style="font-family:system-ui;padding:2rem;background:#0d1117;color:#e6edf3">
<h1 style="color:#58a6ff">🐫 Sabdopalon</h1><p style="color:#8b949e">Dashboard is disabled in config.</p><ul>`)
	for _, name := range sites {
		fmt.Fprintf(w, `<li><a href="http://%s.%s:%d/">%s.%s</a></li>`,
			name, s.cfg.TLD, s.httpPortActual, name, s.cfg.TLD)
	}
	fmt.Fprintf(w, "</ul></body></html>")
}

// discoverSites lists site folder names via the shared scanner.
func discoverSites(cfg *config.Engine) ([]string, error) {
	return vhost.Scan(cfg)
}

func normalizeHost(h string) string {
	if h, _, err := net.SplitHostPort(h); err == nil {
		return h
	}
	return h
}

func isPortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// canBind reports whether the process is allowed to bind a privileged
// (low) TCP port on all interfaces.
func canBind(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

func waitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
