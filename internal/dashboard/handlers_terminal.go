package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/coder/websocket"

	"github.com/sabdopalon/sabdopalon/internal/terminal"
)

// terminalMessage is the client→server frame: input bytes or a resize.
type terminalMessage struct {
	Type string `json:"type"` // "input" | "resize"
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

// handleAPITerminalWS upgrades a WebSocket connection to one terminal
// session. The session lives exactly as long as the connection: on
// disconnect the shell is terminated.
func (s *Server) handleAPITerminalWS(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	// Running optional services contribute their PHP-style env so CLI tools
	// inside the shell see the same variables as sites do.
	var extraEnv []string
	if s.svc != nil {
		extraEnv = s.svc.EnvVars()
	}
	sess, err := terminal.New(s.cfg, dir, extraEnv)
	if err != nil {
		http.Error(w, "terminal: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer sess.Close()

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost", "127.0.0.1"},
	})
	if err != nil {
		return
	}
	defer c.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Shell output → websocket text frames. A PTY read can split a multibyte
	// UTF-8 character across the buffer boundary; writing that raw would
	// corrupt the terminal. Hold back the incomplete tail until the rest
	// arrives.
	go func() {
		buf := make([]byte, 4096)
		var pend []byte
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				chunk := append(pend, buf[:n]...)
				cut := utf8PrefixLen(chunk)
				if cut > 0 {
					if werr := c.Write(ctx, websocket.MessageText, chunk[:cut]); werr != nil {
						cancel()
						return
					}
					pend = append(pend[:0], chunk[cut:]...)
				} else if len(chunk) < utf8.UTFMax*2 {
					pend = append(pend[:0], chunk...)
				}
			}
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// Client frames → shell input / resize.
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var msg terminalMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "input":
			_, _ = sess.Write([]byte(msg.Data))
		case "resize":
			if msg.Rows > 0 && msg.Cols > 0 {
				_ = sess.Resize(msg.Rows, msg.Cols)
			}
		}
	}
}

// utf8PrefixLen returns the length of the longest valid UTF-8 prefix of b,
// holding back an incomplete trailing rune until more bytes arrive.
func utf8PrefixLen(b []byte) int {
	for i := len(b) - 1; i >= len(b)-utf8.UTFMax && i >= 0; i-- {
		if utf8.RuneStart(b[i]) {
			if utf8.Valid(b[i:]) {
				return len(b)
			}
			return i
		}
	}
	return len(b)
}
