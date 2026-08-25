// Package proxy — viteproxy.go: reverses Vite dev server paths so a site
// served at name.localhost can use Vite HMR without the user manually running
// `composer run dev` in a terminal.
//
// When the dev-tools Manager starts Vite for a site, the proxy registers a
// ViteProxy on the siteServer. Vite-specific paths (/@vite/, /node_modules/.vite/,
// and HMR client scripts) are intercepted and forwarded to the Vite dev server
// running on 127.0.0.1:<port>. Everything else goes to PHP as normal.
package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// vitePaths are URL path prefixes that the Vite dev server owns.
var vitePaths = []string{
	"/@vite/",
	"/node_modules/.vite/",
}

// ViteProxy reverses Vite dev server requests to the running Vite instance.
type ViteProxy struct {
	port int
	rp   *httputil.ReverseProxy
}

// NewViteProxy creates a ViteProxy pointing at 127.0.0.1:port.
func NewViteProxy(port int) *ViteProxy {
	target, _ := url.Parse("http://127.0.0.1:" + itoa(port))
	rp := httputil.NewSingleHostReverseProxy(target)
	return &ViteProxy{port: port, rp: rp}
}

// Port returns the Vite dev server port.
func (vp *ViteProxy) Port() int { return vp.port }

// ShouldIntercept reports whether a request should go to Vite instead of PHP.
func (vp *ViteProxy) ShouldIntercept(r *http.Request) bool {
	p := r.URL.Path
	for _, prefix := range vitePaths {
		if strings.HasPrefix(p, prefix) {
			return true
		}
	}
	// HMR websocket upgrade path (Vite uses /, but the upgrade request for
	// the client may include query params). The vite/client.js script and
	// hot-module entries also match here.
	if strings.Contains(p, "/@vite") {
		return true
	}
	// Vite serves its own client entry at the root or with a .js extension
	// when the app references it. The most common HMR paths:
	if p == "/@vite/client" || strings.HasSuffix(p, ".vite/deps/") {
		return true
	}
	return false
}

// ServeHTTP forwards the request to the Vite dev server.
func (vp *ViteProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Vite expects requests at the root, so we don't strip any path prefix.
	vp.rp.ServeHTTP(w, r)
}

// itoa is a tiny strconv.Itoa replacement to avoid an extra import in the
// hot path (the url.Parse above already handles the string).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
