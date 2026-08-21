package dashboard

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleAPIBackup creates a database backup (POST).
func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.backup == nil {
		s.json(w, map[string]string{"error": "backup not configured"})
		return
	}
	path, err := s.backup.Backup()
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	pruned, _ := s.backup.Prune()
	s.json(w, map[string]any{
		"backup":  filepath.Base(path),
		"pruned":  pruned,
		"message": "Backup created: " + filepath.Base(path),
	})
}

// handleAPIBackups lists existing backups.
func (s *Server) handleAPIBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	if s.backup == nil {
		s.json(w, []any{})
		return
	}
	list, err := s.backup.List()
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	result := []map[string]any{}
	for _, b := range list {
		result = append(result, map[string]any{
			"name": b.Name,
			"size": b.Size,
			"time": b.ModTime.Format("2006-01-02 15:04:05"),
		})
	}
	s.json(w, result)
}

// handleAPILogs returns the last 100 lines of a site's PHP log
// (or the DB/service log as fallback).
func (s *Server) handleAPILogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/api/logs/")
	if name == "" || strings.Contains(name, "..") {
		s.json(w, map[string]string{"error": "invalid log name"})
		return
	}
	candidates := []string{name + ".php.log", s.cfg.Database.Engine + ".log", "mailpit.log"}
	for _, suffix := range candidates {
		data, err := os.ReadFile(filepath.Join(s.cfg.Logs, suffix))
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 100 {
			lines = lines[len(lines)-100:]
		}
		s.json(w, map[string]any{
			"file":  suffix,
			"lines": lines,
			"count": len(lines),
		})
		return
	}
	s.json(w, map[string]string{"error": "no logs yet for " + name})
}
