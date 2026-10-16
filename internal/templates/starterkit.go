// Package templates — starterkit.go: Laravel starter kits, installed the way
// `laravel new` installs them.
//
// Reverse-engineered from laravel/installer's NewCommand:
//
//   - the kits are ordinary Composer packages (laravel/react-starter-kit and
//     friends), installed with `composer create-project <kit> <dir>
//     --stability=dev` because they publish no stable tags;
//   - the "blank" variants are the same stacks without authentication
//     (laravel/blank-react-starter-kit, …);
//   - a kit performs its own setup through composer.json's
//     extra.laravel.installer["post-create-project"] hook, which is why nothing
//     here imitates what a kit does internally: `composer create-project` runs
//     the scripts, and the kit migrates itself.
//
// What Sabdopalon adds is the one thing `laravel new` cannot know: the site is
// served at <name>.localhost, so APP_URL points there instead of at the
// `artisan serve` default of localhost:8000.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// First-party starter kit packages, verified present on Packagist.
const (
	kitReact    = "laravel/react-starter-kit"
	kitVue      = "laravel/vue-starter-kit"
	kitSvelte   = "laravel/svelte-starter-kit"
	kitLivewire = "laravel/livewire-starter-kit"

	blankReact    = "laravel/blank-react-starter-kit"
	blankVue      = "laravel/blank-vue-starter-kit"
	blankSvelte   = "laravel/blank-svelte-starter-kit"
	blankLivewire = "laravel/blank-livewire-starter-kit"
)

// starterKitArgs is the composer invocation for one starter kit. Split out so it
// can be asserted without touching the network.
func starterKitArgs(kit, name string) []string {
	return []string{"create-project", kit, name, "--stability=dev"}
}

// setupStarterKit returns a Setup function that installs one Laravel starter kit.
func setupStarterKit(kit string) func(sitesDir, name string, p *Progress) error {
	return func(sitesDir, name string, p *Progress) error {
		if err := requireComposer(); err != nil {
			return err
		}
		dir := filepath.Join(sitesDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		p.Printf("  ⏳  Running composer create-project %s ...", kit)
		if err := p.Run(sitesDir, "composer", starterKitArgs(kit, name)...); err != nil {
			// Remove the partial scaffold so a retry is not blocked by it.
			_ = os.RemoveAll(dir)
			return fmt.Errorf("composer create-project %s failed: %w", kit, err)
		}
		return setAppURL(dir, name)
	}
}

// setAppURL points the application at the URL Sabdopalon actually serves it on.
//
// A fresh kit is configured for `artisan serve` (APP_URL=http://localhost:8000),
// which is the wrong host behind the proxy: absolute asset URLs and redirects
// then name a port nothing listens on. Both files are patched, because
// .env.example is what a later `cp .env.example .env` would restore.
func setAppURL(dir, name string) error {
	want := "APP_URL=http://" + name + ".localhost"
	for _, file := range []string{".env", ".env.example"} {
		path := filepath.Join(dir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // the plain skeleton has both; a kit may ship only .env
		}
		out := replaceAppURL(string(data), want)
		if out == string(data) {
			continue
		}
		if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", file, err)
		}
	}
	return nil
}

// replaceAppURL rewrites the APP_URL line in a dotenv file, appending one when
// the file has none. Only the first occurrence is replaced, so a later
// commented example is left as the kit wrote it.
func replaceAppURL(content, want string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "APP_URL=") {
			lines[i] = want
			return strings.Join(lines, "\n")
		}
	}
	return want + "\n" + content
}
