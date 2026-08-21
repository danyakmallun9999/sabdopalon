package dashboard

import (
	"embed"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The React UI lives in internal/dashboard/ui (Vite + shadcn/ui). Its build
// output (npm run build) is embedded into the binary so the distributed Go
// executable stays fully self-contained.
//
//go:embed all:ui/dist
var distFS embed.FS

// spaHandler serves the built single-page app: /assets/* straight from the
// embedded filesystem, every other non-API GET path falls back to index.html
// so client-side routing (react-router) works on deep links.
func (s *Server) spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "ui/dist")
	if err != nil {
		panic("dashboard: embedded ui/dist unavailable: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		// Serve real files directly (hashed assets, favicon, ...).
		if p != "" {
			if f, err := sub.Open(p); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		index := "index.html"
		if _, err := fs.ReadFile(sub, index); err != nil {
			// UI not built yet (fresh clone without Node) — placeholder page.
			placeholder, rerr := fs.ReadFile(sub, "index.placeholder.html")
			if rerr != nil {
				http.Error(w, "Dashboard UI is not built.\nRun: cd internal/dashboard/ui && npm install && npm run build", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(placeholder)
			return
		}
		data, _ := fs.ReadFile(sub, index)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
}

var _ = io.Discard // retained for future use
