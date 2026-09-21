// Package netutil provides the loopback listeners shared by the proxy and the
// dashboard.
//
// Sabdopalon answers on "localhost" and "<name>.localhost", so the address
// family it binds is user-visible. Windows resolves those names to ::1 BEFORE
// 127.0.0.1 (AAAA is preferred over A), so a server bound to 127.0.0.1 alone
// refuses the first address every client tries.
package netutil

import (
	"errors"
	"net"
	"net/http"
	"strconv"
)

// Listen returns the listeners a Sabdopalon server should serve on for port.
//
// With lan=false (the default) it binds BOTH loopback families. Two listeners
// are the only correct option here:
//
//   - binding 127.0.0.1 alone makes every ::1 (IPv6) connection fail first,
//     which is what browsers and curl hit for "*.localhost";
//   - binding [::1] is NOT a substitute — measured on Windows, a specific
//     IPv6 loopback socket still refuses IPv4-mapped connections even with
//     IPV6_V6ONLY=0, so IPv4 clients would then break instead;
//   - the wildcard [::] would cover both but exposes the dev server, and the
//     PHP it executes, to the whole LAN.
//
// With lan=true the wildcard is bound, which Go already opens dual-stack.
//
// IPv4 is required: if it cannot bind, that error is returned because the
// configured port genuinely belongs to someone else. IPv6 is best effort, so a
// host with the IPv6 stack disabled still gets a working server rather than a
// startup failure.
func Listen(port int, lan bool) ([]net.Listener, error) {
	if lan {
		ln, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
		if err != nil {
			return nil, err
		}
		return []net.Listener{ln}, nil
	}

	v4, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	lns := []net.Listener{v4}
	if v6, verr := net.Listen("tcp", net.JoinHostPort("::1", strconv.Itoa(port))); verr == nil {
		lns = append(lns, v6)
	}
	return lns, nil
}

// Serve serves srv on every listener for port and blocks until they all stop.
// An orderly close (srv.Close/Shutdown) returns nil; the first real listener
// failure is returned after closing the rest, so a half-dead server is never
// left running silently.
func Serve(srv *http.Server, port int, lan bool) error {
	return serve(srv, port, lan, "", "")
}

// ServeTLS is Serve for the HTTPS listener. certFile/keyFile are read once per
// connection (ServeTLS semantics), so a renewed certificate is picked up.
func ServeTLS(srv *http.Server, port int, lan bool, certFile, keyFile string) error {
	return serve(srv, port, lan, certFile, keyFile)
}

func serve(srv *http.Server, port int, lan bool, certFile, keyFile string) error {
	lns, err := Listen(port, lan)
	if err != nil {
		return err
	}
	errc := make(chan error, len(lns))
	for _, ln := range lns {
		go func(l net.Listener) {
			if certFile != "" {
				errc <- srv.ServeTLS(l, certFile, keyFile)
				return
			}
			errc <- srv.Serve(l)
		}(ln)
	}
	for range lns {
		e := <-errc
		if e != nil && !errors.Is(e, http.ErrServerClosed) {
			_ = srv.Close()
			return e
		}
	}
	return nil
}
