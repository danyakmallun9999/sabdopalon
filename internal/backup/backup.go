// Package backup handles automatic database backups for Sabdopalon.
//
// For SQLite: copies the .db file.
// For MariaDB/MySQL: runs mariadb-dump/mysqldump to a .sql.gz file.
//
// Backups are stored in backups/ with timestamped filenames. A configurable
// number of recent backups are retained; older ones are pruned.
package backup

import (
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
)

// Manager handles database backups.
type Manager struct {
	cfg       *config.Engine
	backupDir string
	retention int // number of backups to keep
}

// New creates a backup Manager. retention = how many recent backups to keep.
func New(cfg *config.Engine, retention int) *Manager {
	if retention <= 0 {
		retention = 5
	}
	return &Manager{
		cfg:       cfg,
		backupDir: filepath.Join(cfg.RootDir, "backups"),
		retention: retention,
	}
}

// Backup performs a database backup now. Returns the backup file path.
func (m *Manager) Backup() (string, error) {
	if err := os.MkdirAll(m.backupDir, 0o755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	engine := m.cfg.Database.Engine

	var ext string
	switch engine {
	case "sqlite":
		ext = ".db"
	case "mariadb", "mysql":
		ext = ".sql.gz"
	default:
		ext = ".bak"
	}

	filename := fmt.Sprintf("%s-%s%s", engine, timestamp, ext)
	backupPath := filepath.Join(m.backupDir, filename)

	switch engine {
	case "sqlite":
		return backupPath, m.backupSQLite(backupPath)
	case "mariadb", "mysql":
		return backupPath, m.backupMariaDB(backupPath)
	default:
		return "", fmt.Errorf("backup not supported for engine: %s", engine)
	}
}

// backupSQLite copies the database file.
func (m *Manager) backupSQLite(dest string) error {
	src := m.cfg.Database.Path
	if !fileExists(src) {
		return fmt.Errorf("database file not found: %s", src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

// backupMariaDB runs mariadb-dump and gzips the output.
func (m *Manager) backupMariaDB(dest string) error {
	socket := filepath.Join(m.cfg.Data, m.cfg.Database.Engine+"-sock", "mysqld.sock")
	if !fileExists(socket) {
		return fmt.Errorf("database not running — start sabdopalon serve first (socket not found: %s)", socket)
	}
	dumpBin := m.findDumpBinary()
	if dumpBin == "" {
		return fmt.Errorf("dump binary not found (mariadb-dump or mysqldump)")
	}

	dumpCmd := exec.Command(dumpBin, "--socket="+socket, "-u", database.DatabaseRootUser, "--all-databases")
	dumpOut, err := dumpCmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := dumpCmd.Start(); err != nil {
		return fmt.Errorf("start dump: %w", err)
	}

	outFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	if _, err := copyReader(dumpOut, gz); err != nil {
		return err
	}
	return dumpCmd.Wait()
}

// Prune removes old backups beyond the retention count.
func (m *Manager) Prune() (int, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var backups []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), m.cfg.Database.Engine+"-") {
			backups = append(backups, e)
		}
	}

	if len(backups) <= m.retention {
		return 0, nil
	}

	// Sort by name (timestamp-based, oldest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	removed := 0
	for _, b := range backups[:len(backups)-m.retention] {
		path := filepath.Join(m.backupDir, b.Name())
		if err := os.Remove(path); err == nil {
			removed++
		}
	}
	return removed, nil
}

// List returns existing backup files, newest first.
func (m *Manager) List() ([]BackupInfo, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, err
	}
	var infos []BackupInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, _ := e.Info()
		infos = append(infos, BackupInfo{
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			Path:    filepath.Join(m.backupDir, e.Name()),
		})
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].ModTime.After(infos[j].ModTime)
	})
	return infos, nil
}

// BackupInfo describes a backup file.
type BackupInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	Path    string
}

func (m *Manager) findDumpBinary() string {
	engine := m.cfg.Database.Engine
	binRoot := m.cfg.BinDir()
	candidates := []string{
		filepath.Join(binRoot, engine, "bin", "mariadb-dump"),
		filepath.Join(binRoot, engine, "bin", "mysqldump"),
	}
	for _, c := range candidates {
		if fileExists(c) {
			return c
		}
	}
	if p, err := exec.LookPath("mariadb-dump"); err == nil {
		return p
	}
	if p, err := exec.LookPath("mysqldump"); err == nil {
		return p
	}
	return ""
}

func copyReader(r interface{ Read([]byte) (int, error) }, w interface{ Write([]byte) (int, error) }) (int64, error) {
	buf := make([]byte, 32*1024)
	var total int64
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
		}
		if err != nil {
			if err.Error() == "EOF" {
				return total, nil
			}
			return total, err
		}
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
