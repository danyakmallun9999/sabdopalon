// Package vhost scans the sites/ directory and can generate reference
// Apache/Nginx virtual host configs. Sabdopalon's proxy handles routing
// natively, so these configs are provided only for users who prefer to run
// a standalone Apache/Nginx instead of the built-in proxy.
package vhost

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Site describes one discovered project.
type Site struct {
	Name    string
	DocRoot string
}

// Scan walks the configured root and returns site names (folder names).
func Scan(cfg *config.Engine) ([]string, error) {
	entries, err := os.ReadDir(cfg.Root)
	if err != nil {
		return nil, fmt.Errorf("read sites dir %s: %w", cfg.Root, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ScanSites returns full Site structs with resolved docroots.
func ScanSites(cfg *config.Engine) ([]Site, error) {
	names, err := Scan(cfg)
	if err != nil {
		return nil, err
	}
	var sites []Site
	for _, n := range names {
		docroot := filepath.Join(cfg.Root, n, "public")
		if !dirExists(docroot) {
			docroot = filepath.Join(cfg.Root, n)
		}
		sites = append(sites, Site{Name: n, DocRoot: docroot})
	}
	return sites, nil
}

// GenerateApache writes Apache vhost configs for each site into config/vhosts/.
func GenerateApache(cfg *config.Engine, sites []Site) (int, error) {
	outDir := filepath.Join(cfg.RootDir, "config", "vhosts")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return 0, err
	}
	n := 0
	for _, s := range sites {
		host := fmt.Sprintf("%s.%s", s.Name, cfg.TLD)
		conf := apacheVhost(host, s.DocRoot, cfg.Proxy.HTTPPort, cfg.Proxy.HTTPSPort)
		path := filepath.Join(outDir, s.Name+".conf")
		if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
			return n, err
		}
		n++
		fmt.Printf("  •  vhost: %s -> %s\n", host, s.DocRoot)
	}
	return n, nil
}

func apacheVhost(host, docroot string, httpPort, httpsPort int) string {
	return fmt.Sprintf(`# Reference Apache vhost — Sabdopalon's proxy does not require this.
<VirtualHost *:%d>
    ServerName %s
    DocumentRoot "%s"
    <Directory "%s">
        AllowOverride All
        Require all granted
    </Directory>
    ErrorLog  "logs/%s.error.log"
    CustomLog "logs/%s.access.log" combined
</VirtualHost>
`, httpPort, host, docroot, docroot, host, host)
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
