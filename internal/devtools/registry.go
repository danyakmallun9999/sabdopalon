// Package devtools — registry.go: the list of managed dev-tool specs.
//
// Adding a tool = appending one ToolSpec here. See devtools.go for the
// Manager and lifecycle semantics.
package devtools

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ToolSpec describes one managed dev-tool.
type ToolSpec struct {
	Name      string // "vite", "laravel-dev", "npm-dev", etc.
	Label     string // "Vite Dev Server"
	BinName   string // "npx", "php", "node", "composer"
	Args      func(siteDir string, port int) []string
	Port      int    // 0 = no port (one-shot commands)
	ReadyKind string // "http" | "tcp" | "" (no probe)
	ReadyPath string // HTTP probe path (e.g. "/" for Vite)

	// Env returns extra environment for the process. port is the resolved
	// listen port (0 for one-shot tools).
	Env func(siteDir string, port int) []string

	// available reports whether this tool makes sense for the site dir
	// (e.g. Vite needs a vite.config file).
	available func(siteDir string) bool

	// preflight, when set, runs before the process is spawned. It exists to turn
	// a failure that would otherwise happen deep inside a child — and surface
	// only as "did not become ready on :8000" — into something actionable.
	preflight func(siteDir string) error
}

// ready waits until the tool responds on its port within the timeout.
func (s *ToolSpec) ready(port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		switch s.ReadyKind {
		case "http":
			readyPath := s.ReadyPath
			if readyPath == "" {
				readyPath = "/"
			}
			resp, err := http.Get("http://" + addr + readyPath)
			if err == nil {
				resp.Body.Close()
				return true
			}
		default:
			c, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if err == nil {
				c.Close()
				return true
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// allTools returns the full tool registry.
func allTools() []*ToolSpec {
	return toolRegistry
}

// findTool returns the spec for a name, or nil.
func findTool(name string) *ToolSpec {
	for _, t := range toolRegistry {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// hasViteConfig reports whether a site has a Vite config file.
func hasViteConfig(siteDir string) bool {
	for _, name := range []string{"vite.config.js", "vite.config.mjs", "vite.config.ts", "vite.config.mts"} {
		if fileExists(filepath.Join(siteDir, name)) {
			return true
		}
	}
	return false
}

// hasFile reports whether a named file exists in the site dir.
func hasFile(siteDir, name string) bool {
	return fileExists(filepath.Join(siteDir, name))
}

// hasPackageJSON reports whether package.json exists.
func hasPackageJSON(siteDir string) bool {
	return hasFile(siteDir, "package.json")
}

// composerScripts models the "scripts" object of a composer.json file.
// Values may be strings OR arrays of strings (the Laravel skeleton mixes
// both), so RawMessage keeps any shape decodable — only key presence matters.
type composerScripts struct {
	Scripts map[string]json.RawMessage `json:"scripts"`
}

// hasComposerScript reports whether composer.json defines the given script
// (e.g. "dev" in the Laravel 11+ skeleton, which runs serve + queue + vite).
func hasComposerScript(siteDir, script string) bool {
	if !hasFile(siteDir, "composer.json") {
		return false
	}
	data, err := os.ReadFile(filepath.Join(siteDir, "composer.json"))
	if err != nil {
		return false
	}
	var cs composerScripts
	if err := json.Unmarshal(data, &cs); err != nil {
		return false
	}
	_, ok := cs.Scripts[script]
	return ok
}

// viteNotCoveredByComposerDev reports whether a standalone Vite tool is worth
// offering.
//
// A project whose composer "dev" script already starts the whole stack must not
// be offered a second, separate Vite. Laravel's dev runner registers
// `npm run dev` whenever package.json exists (Illuminate\Foundation\DevCommands
// ::registerDefaults), and two Vite servers would both write public/hot — the
// asset URLs then point at whichever wrote it last, which the user experiences
// as assets that break at random.
func viteNotCoveredByComposerDev(siteDir string) bool {
	if !hasViteConfig(siteDir) {
		return false
	}
	return !(hasComposerScript(siteDir, "dev") && hasPackageJSON(siteDir))
}

// nodePackageManager names the package manager a project's lockfile implies,
// defaulting to npm. Laravel's own dev runner asks its NodePackageManager the
// same question, so the install command offered here is the one `composer run
// dev` will actually use.
func nodePackageManager(siteDir string) string {
	for _, pm := range []struct{ lock, bin string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"bun.lock", "bun"},
	} {
		if hasFile(siteDir, pm.lock) {
			return pm.bin
		}
	}
	return "npm"
}

// nodeModulesReady rejects the one condition that makes Vite die deep inside a
// child process: JS dependencies declared but never installed.
//
// This matters most for `composer run dev`, where the failure is invisible:
// artisan serve and the queue worker come up regardless, so the readiness probe
// SUCCEEDS and the UI reports a healthy tool — while every asset 404s and
// nothing explains why.
func nodeModulesReady(siteDir string) error {
	if !hasPackageJSON(siteDir) {
		return nil // nothing JavaScript to install
	}
	if fileExists(filepath.Join(siteDir, "node_modules")) {
		return nil
	}
	pm := nodePackageManager(siteDir)
	return fmt.Errorf("node_modules is missing — run `%s install` first (Dev Tools → %s install)", pm, pm)
}

// nodeInstallSpec builds a one-shot "<pm> install" tool. One per package
// manager, each gated on the lockfile that identifies it, so a project sees
// exactly the install command it should run.
func nodeInstallSpec(name, bin string, available func(string) bool) *ToolSpec {
	return &ToolSpec{
		Name:      name,
		Label:     bin + " install (one-shot)",
		BinName:   bin,
		Args:      func(siteDir string, port int) []string { return []string{"install"} },
		Env:       func(siteDir string, port int) []string { return nil },
		available: available,
	}
}

// usesPackageManager gates an install tool on the project's detected manager.
func usesPackageManager(bin string) func(string) bool {
	return func(siteDir string) bool {
		return hasPackageJSON(siteDir) && nodePackageManager(siteDir) == bin
	}
}

var toolRegistry = []*ToolSpec{
	{
		Name:    "vite",
		Label:   "Vite Dev Server",
		BinName: "npx",
		Args: func(siteDir string, port int) []string {
			return []string{"vite", "--port", fmt.Sprintf("%d", port), "--host", "127.0.0.1"}
		},
		Port:      5173,
		ReadyKind: "http",
		ReadyPath: "/",
		Env: func(siteDir string, port int) []string {
			return []string{"APP_ENV=local"}
		},
		available: viteNotCoveredByComposerDev,
		preflight: nodeModulesReady,
	},
	{
		Name:    "laravel-dev",
		Label:   "Laravel Dev (composer run dev)",
		BinName: "composer",
		Args: func(siteDir string, port int) []string {
			return []string{"run", "dev"}
		},
		Port:      8000,
		ReadyKind: "tcp",
		Env: func(siteDir string, port int) []string {
			// Laravel's `artisan serve` takes its bind address from SERVER_HOST /
			// SERVER_PORT (Illuminate\Foundation\Console\ServeCommand), not from
			// APP_PORT. With APP_PORT the server stayed on :8000 while this
			// manager probed whatever port it had picked, so the readiness check
			// timed out after 15s and killed an otherwise healthy dev tree.
			//
			// Verified against Laravel 13: SERVER_PORT=8127 makes it print
			// "Server running on [http://127.0.0.1:8127]" and :8000 stays free.
			return []string{
				"APP_ENV=local",
				"SERVER_HOST=127.0.0.1",
				fmt.Sprintf("SERVER_PORT=%d", port),
			}
		},
		available: func(siteDir string) bool { return hasComposerScript(siteDir, "dev") },
		preflight: nodeModulesReady,
	},
	{
		Name:    "npm-dev",
		Label:   "npm run dev",
		BinName: "npm",
		Args: func(siteDir string, port int) []string {
			return []string{"run", "dev"}
		},
		Port:      0,
		ReadyKind: "",
		Env:       func(siteDir string, port int) []string { return nil },
		available: func(siteDir string) bool {
			return hasPackageJSON(siteDir) && !hasViteConfig(siteDir)
		},
	},
	{
		Name:    "npm-build",
		Label:   "npm run build (one-shot)",
		BinName: "npm",
		Args: func(siteDir string, port int) []string {
			return []string{"run", "build"}
		},
		Port:      0,
		ReadyKind: "",
		Env:       func(siteDir string, port int) []string { return nil },
		available: hasPackageJSON,
	},
	{
		Name:    "composer-install",
		Label:   "composer install (one-shot)",
		BinName: "composer",
		Args: func(siteDir string, port int) []string {
			return []string{"install", "--no-interaction"}
		},
		Port:      0,
		ReadyKind: "",
		Env:       func(siteDir string, port int) []string { return nil },
		available: func(siteDir string) bool { return hasFile(siteDir, "composer.json") },
	},
	{
		Name:    "composer-update",
		Label:   "composer update (one-shot)",
		BinName: "composer",
		Args: func(siteDir string, port int) []string {
			return []string{"update", "--no-interaction"}
		},
		Port:      0,
		ReadyKind: "",
		Env:       func(siteDir string, port int) []string { return nil },
		available: func(siteDir string) bool { return hasFile(siteDir, "composer.json") },
	},

	// Node dependency installs. Without one of these, a freshly cloned or
	// created project has no node_modules, and every JS-side tool fails with a
	// message about a missing binary rather than a missing install.
	nodeInstallSpec("npm-install", "npm", usesPackageManager("npm")),
	nodeInstallSpec("pnpm-install", "pnpm", usesPackageManager("pnpm")),
	nodeInstallSpec("yarn-install", "yarn", usesPackageManager("yarn")),
	nodeInstallSpec("bun-install", "bun", usesPackageManager("bun")),
}

// Keep the strings import alive (used in hasViteConfig via filepath).
var _ = strings.TrimSpace
