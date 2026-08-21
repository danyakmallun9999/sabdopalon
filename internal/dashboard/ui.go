package dashboard

import (
	"embed"
	"html/template"
	"io"
	"net/http"
	"path"
	"strings"
)

//go:embed web
var webFS embed.FS

// pageSet holds one parsed template set per UI page.
type pageSet map[string]*template.Template

var pages = []string{"sites", "database", "packages", "ssl", "settings", "logs"}

// loadTemplates parses layouts/base.html together with each page template.
func loadTemplates() pageSet {
	set := pageSet{}
	base, err := webFS.ReadFile("web/layouts/base.html")
	if err != nil {
		return set
	}
	for _, p := range pages {
		body, err := webFS.ReadFile("web/pages/" + p + ".html")
		if err != nil {
			continue
		}
		t := template.Must(template.New("base").Parse(string(base)))
		template.Must(t.New("content").Parse(string(body)))
		set[p] = t
	}
	return set
}

type pageData struct {
	Version  string
	Page     string
	TLD      string
	DashPort int
}

// renderPage serves one embedded HTML page.
func (s *Server) renderPage(w http.ResponseWriter, name string, r *http.Request) {
	t, ok := s.tmpl[name]
	if !ok {
		http.Error(w, "page not available", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.ExecuteTemplate(w, "base", pageData{
		Version:  Version,
		Page:     name,
		TLD:      s.cfg.TLD,
		DashPort: s.cfg.Dashboard.Port,
	})
}

// handleStatic serves CSS/JS/images from the embedded filesystem.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/static/")
	if strings.Contains(name, "..") {
		http.NotFound(w, r)
		return
	}
	f, err := webFS.Open(path.Join("web", name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(name, ".svg"):
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	_, _ = w.Write(data)
}
