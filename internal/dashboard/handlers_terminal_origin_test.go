package dashboard

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// The terminal WebSocket must accept the dashboard's own origin and refuse
// every other one.
//
// This guards a real regression: the options used to carry
// OriginPatterns: []string{"localhost", "127.0.0.1"}, which looks like a
// localhost allow-list but cannot match anything — path.Match compares against
// the Origin host including its port, so "localhost" never matches
// "localhost:9900". The risk in "fixing" it later is that the obvious repair
// ("localhost:*") would let ANY other local dev server on any port open a
// shell. So the assertion is two-sided: same-origin connects, a different
// localhost port is refused.
func TestTerminalWebSocketOriginPolicy(t *testing.T) {
	if got := terminalAcceptOptions().OriginPatterns; len(got) != 0 {
		t.Errorf("terminalAcceptOptions().OriginPatterns = %v, want empty: patterns are "+
			"matched against host:port and any non-empty list here either never matches "+
			"(dead) or widens access to other local servers", got)
	}

	// The real handshake, through the real options.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, terminalAcceptOptions())
		if err != nil {
			return // Accept already wrote the status; the client sees the failure.
		}
		_ = c.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()
	wsURL := "ws" + srv.URL[len("http"):]

	dial := func(t *testing.T, origin string) error {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h := http.Header{}
		h.Set("Origin", origin)
		c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: h})
		if err == nil {
			_ = c.Close(websocket.StatusNormalClosure, "")
		}
		return err
	}

	// Same origin (host matches the request host) — this is what the dashboard
	// does, and it must keep working.
	if err := dial(t, srv.URL); err != nil {
		t.Errorf("same-origin handshake failed: %v", err)
	}

	// A different localhost port is a DIFFERENT origin and must be refused:
	// otherwise any local page could drive the user's shell.
	crossOrigin := "http://localhost:12345"
	if crossOrigin == srv.URL {
		t.Fatalf("test bug: cross-origin %q equals the server URL", crossOrigin)
	}
	if err := dial(t, crossOrigin); err == nil {
		t.Errorf("cross-origin handshake from %q succeeded, want 403 — any local "+
			"dev server could otherwise open a terminal here", crossOrigin)
	}
}
