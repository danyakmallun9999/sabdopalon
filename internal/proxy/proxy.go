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
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Server is the multiplexing proxy that routes *.localhost to per-site PHP.
type Server struct {
	cfg      *config.Engine
	mu       sync.Mutex
	sites    map[string]*siteServer // host -> running PHP server for that site
	portNext int
	stopCh   chan struct{}
}

type siteServer struct {
	host    string
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
		portNext: 9001,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the HTTP and (optionally) HTTPS proxy. Blocks until stopped.
func (s *Server) Start() error {
	// Ensure the router script exists alongside each site.
	s.ensureRouters()

	addr := fmt.Sprintf(":%d", s.cfg.Proxy.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	fmt.Printf("  ✦  Sabdopalon proxy on http://localhost%s  (.*.%s → PHP)\n", addr, s.cfg.TLD)
	fmt.Printf("  ✦  Dashboard: http://localhost:%d/\n", s.cfg.Proxy.HTTPPort)

	errCh := make(chan error, 2)

	// Optional HTTPS listener if a wildcard cert exists.
	if s.startHTTPS(errCh) {
		fmt.Printf("  🔒  HTTPS on https://localhost:%d\n", s.cfg.Proxy.HTTPSPort)
	}

	fmt.Printf("  ⏹  Press Ctrl+C to stop.\n\n")

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

// startHTTPS launches an HTTPS listener if a wildcard cert for *.<tld> exists.
// Returns true if HTTPS was started.
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
			return false
		}
	}
	_ = caCert // referenced for clarity; trust store install is separate
	httpsAddr := fmt.Sprintf(":%d", s.cfg.Proxy.HTTPSPort)
	httpsSrv := &http.Server{
		Addr:         httpsAddr,
		Handler:      s,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	go func() {
		errCh <- httpsSrv.ListenAndServeTLS(certPath, keyPath)
	}()
	return true
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
	ss, err := s.ensureSite(siteName)
	if err != nil {
		http.Error(w, fmt.Sprintf("sabdopalon: cannot start PHP for %s: %v", siteName, err), http.StatusBadGateway)
		return
	}
	ss.proxy.ServeHTTP(w, r)
}

// hostToSite maps "example-app.localhost" -> "example-app".
func (s *Server) hostToSite(host string) (string, bool) {
	tld := s.cfg.TLD
	// bare localhost / 127.0.0.1 → dashboard
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return "", false
	}
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

// ensureSite lazily starts a PHP built-in server for the given site.
func (s *Server) ensureSite(name string) (*siteServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host := name + "." + s.cfg.TLD
	if ss, ok := s.sites[host]; ok {
		return ss, nil
	}
	docroot := filepath.Join(s.cfg.Root, name, "public")
	if _, err := os.Stat(docroot); err != nil {
		docroot = filepath.Join(s.cfg.Root, name)
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

	router := filepath.Join(s.cfg.Root, name, ".sabdopalon-router.php")
	php, err := startPHP(s.cfg.PHP.Binary, port, docroot, lf, s.cfg.Database.Engine, s.cfg.Database.Path)
	_ = router // startPHP checks for router existence internally
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
		dir:     docroot,
		port:    port,
		proxy:   rp,
		php:     php,
		logFile: lf,
	}
	s.sites[host] = ss
	fmt.Printf("  ▶  %s → php :%d  (%s)\n", host, port, docroot)
	return ss, nil
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
		fmt.Printf("  ◾  stopped %s (php :%d)\n", host, ss.port)
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

// dashboardFallback shows a friendly index when hitting bare localhost.
func (s *Server) dashboardFallback(w http.ResponseWriter, r *http.Request, requestedHost string) {
	sites, _ := discoverSites(s.cfg)
	running := s.RunningSites()
	runningSet := map[string]bool{}
	for _, ri := range running {
		runningSet[ri.Host] = true
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Sabdopalon</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:system-ui,-apple-system,sans-serif;background:#0f1117;color:#e0e0e0;padding:2rem 1rem}
.wrap{max-width:720px;margin:0 auto}
h1{color:#58a6ff;margin-bottom:.3rem}
.meta{color:#8b949e;font-size:.9rem;margin-bottom:1.5rem}
ul{list-style:none}
li{display:flex;align-items:center;justify-content:space-between;padding:.8rem 1rem;border:1px solid #30363d;border-radius:10px;margin:.5rem 0;text-decoration:none;color:#e0e0e0;transition:border-color .2s}
li:hover{border-color:#58a6ff}
a{text-decoration:none;color:inherit;display:flex;align-items:center;gap:.5rem;flex:1}
.badge{font-size:.7rem;padding:.15rem .5rem;border-radius:99px;background:#238636;color:#fff}
.badge.off{background:#30363d;color:#8b949e}
.port{color:#6e7681;font-family:monospace;font-size:.85rem}
.empty{padding:1.2rem;color:#8b949e;text-align:center;border:1px dashed #30363d;border-radius:10px}
.hint{margin-top:1.5rem;padding:.8rem;background:#161b22;border-radius:8px;font-family:monospace;font-size:.85rem;color:#8b949e}
</style></head><body><div class="wrap">
<h1>🐫 Sabdopalon</h1>
<p class="meta">Port %d · TLD .%s · PHP %s · DB %s</p>
<h3 style="margin-bottom:.5rem">Sites</h3>
<ul>`, s.cfg.Proxy.HTTPPort, s.cfg.TLD, shortPHP(s.cfg.PHP.Binary), s.cfg.Database.Engine)

	if len(sites) == 0 {
		fmt.Fprintf(w, `<li class="empty">No sites yet. Run: <code>mkdir -p sites/myapp/public</code></li>`)
	}
	for _, name := range sites {
		host := name + "." + s.cfg.TLD
		on := runningSet[host]
		badge := `<span class="badge off">stopped</span>`
		if on {
			badge = `<span class="badge">running</span>`
		}
		fmt.Fprintf(w, `<li><a href="http://%s:%d/">%s</a>%s</li>`, host, s.cfg.Proxy.HTTPPort, host, badge)
	}
	fmt.Fprintf(w, `</ul>
<div class="hint">$ mkdir -p sites/myapp/public &amp;&amp; echo '&lt;?php echo "Hello Sabdopalon";' > sites/myapp/public/index.php</div>
</div></body></html>`)
}

func shortPHP(p string) string {
	if p == "" {
		return "(not found)"
	}
	return filepath.Base(filepath.Dir(p)) + "/" + filepath.Base(p)
}

// discoverSites lists site folder names.
func discoverSites(cfg *config.Engine) ([]string, error) {
	entries, err := os.ReadDir(cfg.Root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
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
