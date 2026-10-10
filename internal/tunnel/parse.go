// Package tunnel gives a site a public URL through a Cloudflare Quick Tunnel.
//
// A Quick Tunnel is `cloudflared tunnel --url http://127.0.0.1:<port>`: no
// Cloudflare account, no domain, an HTTPS URL on the Cloudflare edge, and it
// dies with the process. That is exactly the lifetime Sabdopalon wants for a
// "share this for a moment" action, so it is what v1 implements.
//
// The tunnel targets the Sabdopalon proxy, never PHP directly: the proxy is what
// knows a site's document root, its per-site env and its PHP version. Because
// the public hostname is random and only known once cloudflared has started, the
// proxy learns it at runtime (see Aliaser) — the origin therefore sees the real
// public Host, so frameworks generate links that actually work for the visitor.
// Forcing the internal hostname with cloudflared's httpHostHeader would produce
// `http://myapp.localhost/...` URLs that are dead for everyone outside this
// machine.
package tunnel

import (
	"regexp"
	"strings"
)

// quickTunnelURL matches the public hostname cloudflared prints when a Quick
// Tunnel is up.
//
// The URL is scraped from the log on purpose. cloudflared has no
// machine-readable "here is your URL" output: it draws it inside a box-drawn
// banner, and with --output json inside a log line. Matching the hostname
// itself survives both, and survives the banner being reworded — parsing the
// sentence around it is how these integrations break.
var quickTunnelURL = regexp.MustCompile(`https://[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.trycloudflare\.com`)

// ExtractURL returns the first Quick Tunnel URL found in s, or "".
func ExtractURL(s string) string {
	return quickTunnelURL.FindString(strings.ToLower(s))
}

// IsQuickTunnelHost reports whether host is a trycloudflare.com name with no
// port and no path — the only hostnames this package is allowed to point the
// proxy at.
func IsQuickTunnelHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if host == "" || strings.ContainsAny(host, "/\\ ") {
		return false
	}
	// Require a label before the suffix: "trycloudflare.com" alone is not a
	// tunnel of ours.
	return strings.HasSuffix(host, ".trycloudflare.com") && len(host) > len(".trycloudflare.com")
}

// UnknownFlag reports whether cloudflared rejected one of the flags we passed.
// Older builds do not know --no-autoupdate (or a future one drops it), and the
// useful reaction is to start again without it rather than to report a flag
// error the user cannot act on.
func UnknownFlag(out string) bool {
	l := strings.ToLower(out)
	for _, s := range []string{
		"flag provided but not defined",
		"unknown flag",
		"unknown shorthand flag",
	} {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// EdgeUnreachable reports whether cloudflared failed to reach the Cloudflare
// edge. Quick Tunnels use QUIC on UDP 7844 by default; on a network that blocks
// it, one retry over http/2 is usually all that is needed.
func EdgeUnreachable(out string) bool {
	l := strings.ToLower(out)
	for _, s := range []string{
		"failed to dial to edge",
		"failed to establish connection",
		"no such host",
		"i/o timeout",
		"connection timed out",
		"connection refused",
		"register tunnel error",
	} {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// RateLimited reports whether Cloudflare refused to hand out a tunnel, which it
// does under load. The message is worth translating: the raw text reads like a
// bug in Sabdopalon.
func RateLimited(out string) bool {
	l := strings.ToLower(out)
	return strings.Contains(l, "429") || strings.Contains(l, "too many requests") ||
		strings.Contains(l, "rate limit")
}
