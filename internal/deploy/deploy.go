// Package deploy installs web apps (phpMyAdmin) into the sites tree with
// pre-wired Sabdopalon configuration. Shared by the CLI (`add phpmyadmin`)
// and the GUI setup wizard.
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
	dest := filepath.Join(cfg.Root, "phpmyadmin", "public")
	if err := copyTree(src, dest); err != nil {
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
	if err := os.WriteFile(filepath.Join(dest, "config.inc.php"), []byte(config), 0o644); err != nil {
		return fmt.Errorf("write phpMyAdmin config: %w", err)
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
