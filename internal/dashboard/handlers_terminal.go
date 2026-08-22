package dashboard

import (
	"context"
	"encoding/json"
	"net/http"

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
	sess, err := terminal.New(s.cfg, dir)
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

	// Shell output → websocket text frames.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				_ = c.Write(ctx, websocket.MessageText, buf[:n])
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
