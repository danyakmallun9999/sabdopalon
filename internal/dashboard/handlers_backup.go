package dashboard

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleAPIBackup creates a database backup (POST /api/backup?engine=X).
// engine defaults to the primary engine; every daemon engine is supported.
func (s *Server) handleAPIBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.methodNotAllowed(w, "POST")
		return
	}
	if s.backup == nil {
		s.json(w, map[string]string{"error": "backup not configured"})
		return
	}
	engine := r.URL.Query().Get("engine")
	if engine == "" || engine == "mysql" {
		engine = s.cfg.Database.Engine
	}
	path, err := s.backup.Backup(engine)
	if err != nil {
		s.json(w, map[string]string{"error": err.Error()})
		return
	}
	pruned, _ := s.backup.Prune(engine)
	s.json(w, map[string]any{
		"backup":  filepath.Base(path),
		"pruned":  pruned,
		"message": "Backup dibuat: " + filepath.Base(path),
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
	candidates := []string{name + ".php.log",
		s.cfg.Database.Engine + ".log", "mariadb.log", "postgresql.log", "mailpit.log"}
	for _, suffix := range candidates {
		data, err := readTail(filepath.Join(s.cfg.Logs, suffix), 256*1024)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) > 300 {
			lines = lines[len(lines)-300:]
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

// readTail returns at most maxBytes from the END of the file — log files can
// grow unbounded and the UI only shows recent lines anyway.
func readTail(path string, maxBytes int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	if size > maxBytes {
		if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
			return nil, err
		}
		data := make([]byte, maxBytes)
		n, _ := io.ReadFull(f, data)
		// Drop the first (likely partial) line.
		if i := strings.IndexByte(string(data[:n]), '\n'); i >= 0 && i < n-1 {
			return data[i+1 : n], nil
		}
		return data[:n], nil
	}
	return io.ReadAll(f)
}
