// Package templates — testing.go: the Pest or PHPUnit choice.
//
// The command sequence is laravel/installer's installPest(), in its own order:
// remove PHPUnit, require Pest and its Laravel plugin without updating, then a
// single update; initialise Pest; use the drift plugin to convert the skeleton's
// PHPUnit tests; drop that one-shot plugin again. PHPUnit needs nothing — it is
// what every skeleton and starter kit already ships.
package templates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Testing frameworks a new site can be set up with.
const (
	TestingPest    = "pest"
	TestingPHPUnit = "phpunit"
)

// NormalizeTesting maps input onto the two choices, defaulting to Pest exactly
// as `laravel new` does (`if (! $input->getOption('phpunit')) { ... pest ... }`).
func NormalizeTesting(framework string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(framework)) {
	case "", TestingPest:
		return TestingPest, nil
	case TestingPHPUnit:
		return TestingPHPUnit, nil
	}
	return "", fmt.Errorf("unsupported testing framework %q — use %s (default) or %s", framework, TestingPest, TestingPHPUnit)
}

// configureTesting installs Pest when it was chosen, and does nothing for
// PHPUnit.
func configureTesting(dir, framework string, logf func(string, ...any)) error {
	framework, err := NormalizeTesting(framework)
	if err != nil {
		return err
	}
	if framework == TestingPHPUnit {
		return nil
	}

	say := func(format string, args ...any) {
		if logf != nil {
			logf(format, args...)
		}
	}
	say("installing Pest")

	steps := []struct {
		bin  string
		args []string
	}{
		{"composer", []string{"remove", "phpunit/phpunit", "--dev", "--no-update"}},
		{"composer", []string{"require", "pestphp/pest", "pestphp/pest-plugin-laravel", "--no-update", "--dev"}},
		{"composer", []string{"update"}},
		{"php", []string{"./vendor/bin/pest", "--init", "--no-interaction"}},
	}
	for _, step := range steps {
		if err := runIn(dir, step.bin, step.args...); err != nil {
			return err
		}
	}

	// Drift rewrites the skeleton's PHPUnit-style tests into Pest style. It is
	// the one cosmetic step in the sequence, and it can fail on its own — measured
	// as a DirectoryNotFoundException whose trace named the cause:
	//
	//	Pest\Drift\Finder\Finder::__construct("--no-interaction")
	//
	// The plugin takes a positional path and treats that flag as one, so no flag
	// is passed to it. Failure is still not fatal: a project whose Pest
	// installation is complete must not be thrown away over a style conversion.
	if err := runIn(dir, "composer", "require", "pestphp/pest-plugin-drift", "--dev"); err != nil {
		say("skipping test drift: %v", err)
	} else {
		if err := runIn(dir, "php", "./vendor/bin/pest", "--drift"); err != nil {
			say("test drift did not convert anything: %v", err)
		}
		if err := runIn(dir, "composer", "remove", "pestphp/pest-plugin-drift", "--dev"); err != nil {
			say("could not remove the drift plugin: %v", err)
		}
	}
	uncommentRefreshDatabase(dir)
	return nil
}

// uncommentRefreshDatabase applies the same edit laravel/installer makes to a
// starter kit: tests/Pest.php ships the trait commented out, and a project whose
// tests hit the database wants it on.
func uncommentRefreshDatabase(dir string) {
	path := filepath.Join(dir, "tests", "Pest.php")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	out := strings.Replace(string(data),
		" // ->use(RefreshDatabase::class)",
		"    ->use(RefreshDatabase::class)",
		1)
	if out != string(data) {
		_ = os.WriteFile(path, []byte(out), 0o644)
	}
}

// runIn runs a tool inside the project directory with Sabdopalon's bin/ first on
// PATH, so a nested `php` call resolves to the bundled interpreter no matter how
// the app was launched. The binary itself is resolved explicitly, because
// exec.Command searches the CURRENT process's PATH rather than cmd.Env.
func runIn(dir, name string, args ...string) error {
	bin, err := resolveTool(name)
	if err != nil {
		return err
	}
	cmd := exec.Command(bin, args...)
	winproc.Quiet(cmd)
	cmd.Dir = dir
	cmd.Env = childEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s failed (%w): %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
