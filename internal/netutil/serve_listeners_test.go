package netutil

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// ServeListeners exists so a caller can DECIDE about a port by binding it (see
// the low-port preference in the proxy). It has to serve on exactly the
// listeners it is handed, and has to come back once they are closed.
func TestServeListenersServesOnProvidedListeners(t *testing.T) {
	// Reserve a free port, then release it: Listen(0) would give the IPv4 and
	// IPv6 listeners two DIFFERENT random ports.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	lns, err := Listen(port, false)
	if err != nil {
		t.Fatalf("Listen(%d): %v", port, err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("served"))
	})}

	done := make(chan error, 1)
	go func() { done <- ServeListeners(srv, lns, "", "") }()

	deadline := time.Now().Add(3 * time.Second)
	var body string
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://127.0.0.1:" + itoa(port))
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if body != "served" {
		t.Fatalf("handler output = %q, want %q", body, "served")
	}

	_ = srv.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("ServeListeners returned %v after an orderly close, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("ServeListeners did not return after Close")
	}
}

func TestServeListenersRejectsNoListeners(t *testing.T) {
	if err := ServeListeners(&http.Server{}, nil, "", ""); err == nil {
		t.Error("no listeners must be an error, not a silent no-op server")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
