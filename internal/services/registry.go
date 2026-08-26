// Package services — registry.go: the list of optional managed services.
//
// Adding a service = appending one spec here (plus a packages/packages.toml
// entry for its binary). See Spec in services.go for field semantics.
package services

import (
	"fmt"
	"runtime"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Enabled reads the [services] toggle for a name from engine config.
func enabledFromCfg(cfg *config.Engine, name string) bool {
	switch name {
	case "mailpit":
		return cfg.Services.Mailpit
	case "redis":
		return cfg.Services.Redis
	case "minio":
		return cfg.Services.MinIO
	case "meilisearch":
		return cfg.Services.Meilisearch
	default:
		return false
	}
}

// SetEnabled flips the [services] toggle for a name (caller persists config).
func SetEnabled(cfg *config.Engine, name string, v bool) bool {
	switch name {
	case "mailpit":
		cfg.Services.Mailpit = v
	case "redis":
		cfg.Services.Redis = v
	case "minio":
		cfg.Services.MinIO = v
	case "meilisearch":
		cfg.Services.Meilisearch = v
	default:
		return false
	}
	return true
}

func registry() []*Spec {
	return []*Spec{
		{
			Name:     "mailpit",
			Label:    "Mailpit — local e-mail catcher",
			Package:  "mailpit",
			BinNames: binNames("mailpit"),
			Ports:    []int{1025, 8025},
			Args: func(_ *config.Engine, _ string) []string {
				return []string{"--smtp", "127.0.0.1:1025", "--listen", "127.0.0.1:8025"}
			},
			ReadyKind:   "http",
			ReadyPort:   8025, // Ports[0]=1025 is SMTP; probe the HTTP UI instead
			ConsolePort: 8025,
			UIPath:      "/",
			Label2:      "Web UI",
			Hint:        "install via 'sabdopalon add mailpit'",
			PHPEnv: func(_ *config.Engine) []string {
				return []string{
					"SABDOPALON_MAIL_SMTP=127.0.0.1:1025",
					"SABDOPALON_MAIL_UI=http://localhost:8025",
				}
			},
		},

		{
			Name:         "redis",
			Label:        "Redis — cache & queue",
			Package:      "redis",
			BinNames:     binNames("redis-server"),
			PathFallback: "redis-server",
			Hint:         "Windows: bundled via 'sabdopalon add redis' · Linux/macOS: install redis-server (apt/brew) or run 'sabdopalon add redis'",
			Ports:        []int{6379},
			DataSub:      "redis",
			Args: func(_ *config.Engine, dataDir string) []string {
				// Dev-grade hardening: loopback only, no persistence surprises.
				return []string{
					"--bind", "127.0.0.1",
					"--port", "6379",
					"--dir", dataDir,
					"--save", "",
					"--appendonly", "no",
				}
			},
			ReadyKind:   "tcp",
			ConsolePort: 0,
			PHPEnv: func(_ *config.Engine) []string {
				return []string{
					"SABDOPALON_REDIS_HOST=127.0.0.1",
					"SABDOPALON_REDIS_PORT=6379",
				}
			},
		},

		{
			Name:     "minio",
			Label:    "MinIO — S3-compatible storage",
			Package:  "minio",
			BinNames: binNames("minio"),
			Ports:    []int{9000, 9001},
			DataSub:  "minio",
			Args: func(_ *config.Engine, dataDir string) []string {
				return []string{
					"server", dataDir,
					"--address", "127.0.0.1:9000",
					"--console-address", "127.0.0.1:9001",
				}
			},
			ReadyKind:   "http",
			ReadyPath:   "/minio/health/live",
			ConsolePort: 9001,
			UIPath:      "/",
			Label2:      "Console",
			Hint:        "install via 'sabdopalon add minio'",
			PHPEnv: func(_ *config.Engine) []string {
				return []string{
					"SABDOPALON_S3_ENDPOINT=http://127.0.0.1:9000",
					"SABDOPALON_S3_KEY=sabdopalon",
					"SABDOPALON_S3_SECRET=sabdopalon-secret",
					"SABDOPALON_S3_BUCKET=sabdopalon-bucket",
					fmt.Sprintf("MINIO_ROOT_USER=%s", "sabdopalon"),
					fmt.Sprintf("MINIO_ROOT_PASSWORD=%s", "sabdopalon-secret"),
				}
			},
		},

		{
			Name:     "meilisearch",
			Label:    "Meilisearch — instant search",
			Package:  "meilisearch",
			BinNames: binNames("meilisearch"),
			Ports:    []int{7700},
			DataSub:  "meilisearch",
			Args: func(_ *config.Engine, dataDir string) []string {
				return []string{
					"--http-addr", "127.0.0.1:7700",
					"--db-path", dataDir,
					"--env", "development", // no master key required on loopback
				}
			},
			ReadyKind:   "http",
			ReadyPath:   "/health",
			ConsolePort: 7700,
			UIPath:      "/",
			Label2:      "Mini-dashboard",
			Hint:        "install via 'sabdopalon add meilisearch'",
			PHPEnv: func(_ *config.Engine) []string {
				return []string{
					"SABDOPALON_MEILI_HOST=http://127.0.0.1:7700",
				}
			},
		},
	}
}

// binNames maps a base binary name to platform-specific candidates.
func binNames(base string) []string {
	if runtime.GOOS == "windows" {
		return []string{base + ".exe"}
	}
	return []string{base}
}
