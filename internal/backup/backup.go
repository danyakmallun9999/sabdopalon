// Package backup handles database backups for Sabdopalon.
//
//	SQLite:            copies the .db file
//	MariaDB/MySQL:     mariadb-dump/mysqldump → .sql.gz (unix socket)
//	PostgreSQL:        pg_dump → .sql.gz (TCP 127.0.0.1)
//
// Every daemon engine can be backed up independently — backups/<engine>-… —
// and each engine's history is pruned separately.
package backup

import (
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Manager handles database backups.
type Manager struct {
	cfg       *config.Engine
	backupDir string
	retention int // number of backups to keep, per engine
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

// Backup performs a backup of ONE engine now. Returns the backup file path.
func (m *Manager) Backup(engine string) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0o755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	var ext string
	switch engine {
	case "sqlite":
		ext = ".db"
	case "mariadb", "mysql", "postgresql":
		ext = ".sql.gz"
	default:
		return "", fmt.Errorf("backup tidak didukung untuk engine: %s", engine)
	}

	backupPath := filepath.Join(m.backupDir, fmt.Sprintf("%s-%s%s", engine, timestamp, ext))

	switch engine {
	case "sqlite":
		return backupPath, m.backupSQLite(backupPath)
	case "mariadb", "mysql":
		return backupPath, m.backupMariaDB(backupPath, engine)
	case "postgresql":
		return backupPath, m.backupPostgreSQL(backupPath)
	}
	return "", fmt.Errorf("engine tidak dikenal: %s", engine)
}

// backupSQLite copies the database file.
func (m *Manager) backupSQLite(dest string) error {
	src := m.cfg.Database.Path
	if !fileExists(src) {
		return fmt.Errorf("file database tidak ditemukan: %s", src)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o644)
}

// backupMariaDB runs mariadb-dump/mysqldump over the daemon's unix socket.
func (m *Manager) backupMariaDB(dest, engine string) error {
	socket := filepath.Join(m.cfg.Data, engine+"-sock", "mysqld.sock")
	if !fileExists(socket) {
		return fmt.Errorf("database tidak berjalan — nyalakan dulu di halaman Database (socket: %s)", socket)
	}
	dumpBin := m.findDumpBinary(engine)
	if dumpBin == "" {
		return fmt.Errorf("binary dump tidak ditemukan (mariadb-dump / mysqldump)")
	}

	dumpCmd := exec.Command(dumpBin, "--socket="+socket, "-u", database.DatabaseRootUser, "--all-databases")
	winproc.Quiet(dumpCmd)
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

// backupPostgreSQL runs pg_dump over TCP (trust auth, no password needed).
func (m *Manager) backupPostgreSQL(dest string) error {
	pgDump := filepath.Join(m.cfg.BinDir(), "postgresql", "bin", "pg_dump"+extSuffix())
	if !fileExists(pgDump) {
		if p, err := exec.LookPath("pg_dump"); err == nil {
			pgDump = p
		} else {
			return fmt.Errorf("pg_dump tidak ditemukan (bundel PostgreSQL belum terpasang)")
		}
	}

	port := fmt.Sprintf("%d", database.EffectivePort(m.cfg, "postgresql"))
	dumpCmd := exec.Command(pgDump,
		"-h", "127.0.0.1", "-p", port, "-U", "sabdopalon",
		"--no-owner", "--no-privileges", "postgres")
	winproc.Quiet(dumpCmd)
	dumpOut, err := dumpCmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := dumpCmd.Start(); err != nil {
		return fmt.Errorf("start pg_dump: %w", err)
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

// Prune removes old backups beyond the retention count FOR one engine
// (files are prefixed "<engine>-").
func (m *Manager) Prune(engine string) (int, error) {
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	var backups []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), engine+"-") {
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

func (m *Manager) findDumpBinary(engine string) string {
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

func extSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
