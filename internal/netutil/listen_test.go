package netutil

import (
	"fmt"
	"net"
	"testing"
	"time"
)

// freePort grabs an ephemeral port, releases it, and returns it. The tiny
// reuse window only matters if another process steals it in between.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func dialOK(host string, port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func hasIPv6(lns []net.Listener) bool {
	for _, ln := range lns {
		if addr, ok := ln.Addr().(*net.TCPAddr); ok && addr.IP.To4() == nil {
			return true
		}
	}
	return false
}

// Windows resolves "*.localhost" to ::1 before 127.0.0.1. A loopback server
// that only binds IPv4 refuses that first attempt — the browser has to fall
// back (curl measured ~0.44s instead of ~0.01s) and any client that does not
// fall back cannot reach the site or the dashboard at all.
func TestListenServesBothLoopbackFamilies(t *testing.T) {
	port := freePort(t)
	lns, err := Listen(port, false)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	}()

	if !dialOK("127.0.0.1", port) {
		t.Error("IPv4 loopback is not served — the dashboard and every site would be unreachable")
	}
	if !hasIPv6(lns) {
		t.Skip("IPv6 loopback is unavailable on this host")
	}
	if !dialOK("::1", port) {
		t.Error("IPv6 loopback is not served — clients resolving *.localhost to ::1 are refused")
	}
}

// The default must never widen exposure: PHP executes on these ports, so a
// localhost-only server has to stay on loopback even though it now binds two
// addresses.
func TestListenLocalhostStaysOnLoopback(t *testing.T) {
	port := freePort(t)
	lns, err := Listen(port, false)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	}()

	for _, ln := range lns {
		addr, ok := ln.Addr().(*net.TCPAddr)
		if !ok {
			t.Fatalf("unexpected listener address type %T", ln.Addr())
		}
		if addr.IP == nil || addr.IP.IsUnspecified() || !addr.IP.IsLoopback() {
			t.Errorf("listener %s is not loopback — sites would be exposed to the LAN", ln.Addr())
		}
	}
}

// lan=true keeps the documented opt-in: one wildcard listener, dual-stack.
func TestListenLANUsesWildcard(t *testing.T) {
	port := freePort(t)
	lns, err := Listen(port, true)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer func() {
		for _, ln := range lns {
			_ = ln.Close()
		}
	}()

	if len(lns) != 1 {
		t.Fatalf("LAN mode should bind exactly one wildcard listener, got %d", len(lns))
	}
	addr, ok := lns[0].Addr().(*net.TCPAddr)
	if !ok || !addr.IP.IsUnspecified() {
		t.Fatalf("LAN mode should bind the wildcard, got %s", lns[0].Addr())
	}
}
