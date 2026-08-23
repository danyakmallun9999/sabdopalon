// Package deploy installs web apps (phpMyAdmin) into the sites tree with
// pre-wired Sabdopalon configuration. Shared by the CLI (`add phpmyadmin`)
// and the GUI setup wizard.
//
// Deployment is verified and atomic: the tree is staged into a sibling
// directory, checked against canary files, and only then renamed into its
// final location. A half-copied phpMyAdmin (the classic cause of the
// "failed opening required .../libraries/constants.php" fatal) can no longer
// end up served as a site.
package deploy

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// phpMyAdminCanaries are the minimal files any complete phpMyAdmin tree must
// contain. libraries/constants.php is exactly the file whose absence causes
// the well-known require_once fatal error in public/index.php — it is the
// earliest visible symptom of a truncated copy/extraction.
var phpMyAdminCanaries = []string{
	"index.php",
	"libraries/constants.php",
}

// treeComplete reports whether dir contains all canary files.
func treeComplete(dir string) bool {
	for _, rel := range phpMyAdminCanaries {
		st, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// PHPMyAdminComplete reports whether a deployed phpMyAdmin docroot is
// complete enough to serve (used by bootstrap to decide on re-deployment).
func PHPMyAdminComplete(public string) bool {
	return treeComplete(public)
}

// PHPMyAdmin copies the downloaded phpMyAdmin tree from bin/phpmyadmin into
// sites/phpmyadmin/public and writes a config.inc.php pre-wired to the local
// MariaDB (root, no password, cookie auth).
func PHPMyAdmin(cfg *config.Engine) error {
	return PHPMyAdminFrom(filepath.Join(cfg.RootDir, "bin", "phpmyadmin"), cfg)
}

// PHPMyAdminFrom deploys phpMyAdmin from an explicit source dir (writable
// bin/ or read-only resource dir in desktop mode).
func PHPMyAdminFrom(src string, cfg *config.Engine) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("phpMyAdmin not found (run 'sabdopalon add phpmyadmin' first)")
	}
	if !treeComplete(src) {
		return fmt.Errorf(
			"phpMyAdmin source at %s is incomplete (missing %s) — reinstall it with 'sabdopalon add phpmyadmin'",
			src, phpMyAdminCanaries[1])
	}
	dest := filepath.Join(cfg.Root, "phpmyadmin", "public")
	staging := dest + ".staging"

	if err := os.RemoveAll(staging); err != nil {
		return fmt.Errorf("clear staging dir: %w", err)
	}
	if err := copyTree(src, staging); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("copy phpMyAdmin: %w", err)
	}
	blowfish := randomHex(32)
	config := fmt.Sprintf(`<?php
// Sabdopalon-generated phpMyAdmin config — local dev only (127.0.0.1).
$cfg['blowfish_secret'] = '%s';
$i = 0;
$i++;
$cfg['Servers'][$i]['host'] = '127.0.0.1';
$cfg['Servers'][$i]['port'] = %d;
$cfg['Servers'][$i]['user'] = 'root';
$cfg['Servers'][$i]['password'] = '';
$cfg['Servers'][$i]['auth_type'] = 'cookie';
$cfg['Servers'][$i]['AllowNoPassword'] = true;
$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
`, blowfish, cfg.Database.Port)
	if err := os.WriteFile(filepath.Join(staging, "config.inc.php"), []byte(config), 0o644); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("write phpMyAdmin config: %w", err)
	}
	// Verify BEFORE promoting — the live docroot only ever appears complete.
	if !treeComplete(staging) {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("copied phpMyAdmin failed verification (%s missing) — source may be corrupted", phpMyAdminCanaries[1])
	}
	// Move the previous deploy aside; only delete it once the replacement is
	// in place, so a failed promotion never leaves the site missing.
	backup := dest + ".old"
	if err := os.RemoveAll(backup); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("clear old backup %s: %w", backup, err)
	}
	hadOld := false
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			_ = os.RemoveAll(staging)
			return fmt.Errorf("back up previous deploy: %w", err)
		}
		hadOld = true
	}
	if err := os.Rename(staging, dest); err != nil {
		if hadOld {
			_ = os.Rename(backup, dest)
		}
		return fmt.Errorf("promote phpMyAdmin deploy: %w", err)
	}
	if hadOld {
		_ = os.RemoveAll(backup)
	}
	return nil
}

// copyTree recursively copies a directory tree.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			dest, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(dest, target)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

// randomHex returns n random bytes hex-encoded (crypto-grade).
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
