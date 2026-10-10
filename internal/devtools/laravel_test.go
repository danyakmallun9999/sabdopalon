package devtools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Laravel's `artisan serve` takes its bind address from SERVER_HOST/SERVER_PORT
// (Illuminate\Foundation\Console\ServeCommand), NOT from APP_PORT. With APP_PORT
// the server stayed on :8000 while this manager probed whatever port it had
// picked, so the readiness check timed out after 15s and killed an otherwise
// healthy dev tree — the exact failure this pins.
//
// Verified against Laravel 13: SERVER_PORT=8127 prints
// "Server running on [http://127.0.0.1:8127]" and leaves :8000 free.
func TestLaravelDevUsesServerPortEnv(t *testing.T) {
	spec := findTool("laravel-dev")
	if spec == nil {
		t.Fatal("laravel-dev is not registered")
	}

	// Windows cannot run Laravel's own script at all (see laravelDevTool): it
	// ends with `artisan pail`, which needs the POSIX-only pcntl extension. There
	// is no artisan serve to point anywhere either, because Sabdopalon serves the
	// site through its proxy — so what is checked here is that the Windows shape
	// starts the two things that genuinely need supervising and nothing else.
	if runtime.GOOS == "windows" {
		args := strings.Join(spec.Args(t.TempDir(), 0), " ")
		if !strings.Contains(args, "queue:listen") {
			t.Errorf("laravel-dev on Windows must run the queue worker, got %q", args)
		}
		if !strings.Contains(args, "npm run dev") {
			t.Errorf("laravel-dev on Windows must run Vite, got %q", args)
		}
		if strings.Contains(args, "pail") {
			t.Errorf("laravel-dev must not run pail — pcntl does not exist on Windows: %q", args)
		}
		if strings.Contains(args, "artisan serve") {
			t.Errorf("laravel-dev must not run artisan serve — the proxy already serves the site: %q", args)
		}
		if spec.BinName != "npx" {
			t.Errorf("laravel-dev on Windows runs concurrently through npx, got %q", spec.BinName)
		}
		// No readiness probe: Vite's port comes from the project's own config, and
		// the manager kills whatever fails to become ready on a port we guessed.
		if spec.Port != 0 || spec.ReadyKind != "" {
			t.Errorf("laravel-dev on Windows must not probe a fixed port (port=%d kind=%q)", spec.Port, spec.ReadyKind)
		}
		return
	}

	env := spec.Env(t.TempDir(), 8127)
	joined := strings.Join(env, " ")

	if !strings.Contains(joined, "SERVER_PORT=8127") {
		t.Errorf("laravel-dev env must point artisan serve at the probed port, got %v", env)
	}
	if !strings.Contains(joined, "SERVER_HOST=127.0.0.1") {
		t.Errorf("laravel-dev env must set SERVER_HOST, got %v", env)
	}
	for _, e := range env {
		if strings.HasPrefix(e, "APP_PORT=") {
			t.Errorf("APP_PORT is not read by Laravel and gave a false sense of control: %q", e)
		}
	}
}

// A project whose composer "dev" script already starts Vite must not also be
// offered a standalone Vite: two servers both write public/hot, and the asset
// URLs then follow whichever wrote last.
func TestViteSuppressedWhenComposerDevRunsIt(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "vite.config.js"), []byte("{}"), 0o644)

	// Vite config alone: the standalone tool is the only way to run it.
	if !hasViteConfig(dir) || !viteNotCoveredByComposerDev(dir) {
		t.Error("with only vite.config.js, the standalone Vite tool must be offered")
	}

	// Laravel 13 shape: composer dev exists and package.json exists, so
	// `artisan dev` starts `npm run dev` itself.
	_ = os.WriteFile(filepath.Join(dir, "composer.json"),
		[]byte(`{"scripts":{"dev":"Composer\\Config::disableProcessTimeout"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"dev":"vite"}}`), 0o644)
	if viteNotCoveredByComposerDev(dir) {
		t.Error("composer dev + package.json already runs Vite — the standalone tool must step aside")
	}

	// ...but a composer dev script without JS is not an excuse to hide Vite.
	_ = os.Remove(filepath.Join(dir, "package.json"))
	if !viteNotCoveredByComposerDev(dir) {
		t.Error("without package.json, composer dev cannot be starting Vite")
	}
}

// Without an install tool a fresh project has no node_modules, and every JS-side
// tool fails with a message about a missing binary instead of a missing install.
func TestNodeInstallToolMatchesLockfile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	// No lockfile → npm, and only npm.
	if got := nodePackageManager(dir); got != "npm" {
		t.Errorf("nodePackageManager with no lockfile = %q, want npm", got)
	}
	if names := installTools(dir); len(names) != 1 || names[0] != "npm-install" {
		t.Errorf("install tools = %v, want exactly npm-install", names)
	}

	// A pnpm lockfile switches the tool, and never leaves two of them.
	_ = os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: 9"), 0o644)
	if got := nodePackageManager(dir); got != "pnpm" {
		t.Errorf("nodePackageManager with pnpm-lock.yaml = %q, want pnpm", got)
	}
	if names := installTools(dir); len(names) != 1 || names[0] != "pnpm-install" {
		t.Errorf("install tools = %v, want exactly pnpm-install", names)
	}

	// No JavaScript at all → no install tool.
	_ = os.Remove(filepath.Join(dir, "package.json"))
	if names := installTools(dir); len(names) != 0 {
		t.Errorf("install tools = %v, want none without package.json", names)
	}
}

// The message has to name the command and where to click, because for
// `composer run dev` the failure is otherwise invisible: serve and queue come
// up, the probe succeeds, and the UI reports a healthy tool.
func TestNodeModulesReadyMessage(t *testing.T) {
	dir := t.TempDir()
	if err := nodeModulesReady(dir); err != nil {
		t.Errorf("no package.json means nothing to install, got %v", err)
	}

	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)
	err := nodeModulesReady(dir)
	if err == nil {
		t.Fatal("package.json without node_modules must be rejected")
	}
	for _, want := range []string{"node_modules", "npm install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}

	_ = os.MkdirAll(filepath.Join(dir, "node_modules"), 0o755)
	if err := nodeModulesReady(dir); err != nil {
		t.Errorf("with node_modules present it must pass, got %v", err)
	}
}

// Start must run the preflight BEFORE resolving a binary, so the user gets the
// actionable message instead of "npx not found".
func TestStartRunsPreflightAndReportsIt(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "vite.config.js"), []byte("{}"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	m := New(nil)
	_, err := m.Start("site", dir, "vite")
	if err == nil {
		t.Fatal("starting vite without node_modules must fail")
	}
	if !strings.Contains(err.Error(), "node_modules") {
		t.Errorf("error = %q, want it to name the missing node_modules", err)
	}

	// The same message must reach the Dev Tools tab, not just the API caller.
	found := false
	for _, st := range m.Status("site") {
		if st.Tool == "vite" && strings.Contains(st.LastError, "node_modules") {
			found = true
		}
	}
	if !found {
		t.Error("the preflight message did not reach Status(), so the UI would show nothing")
	}
}

// installTools lists the registered node install tools applicable to dir.
func installTools(dir string) []string {
	var out []string
	for _, spec := range allTools() {
		if !strings.HasSuffix(spec.Name, "-install") || strings.HasPrefix(spec.Name, "composer") {
			continue
		}
		if spec.available(dir) {
			out = append(out, spec.Name)
		}
	}
	return out
}
