package templates

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestNormalizeTesting: Pest is the default because that is what `laravel new`
// installs unless --phpunit is given.
func TestNormalizeTesting(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", TestingPest, false},
		{"pest", TestingPest, false},
		{"Pest", TestingPest, false},
		{"phpunit", TestingPHPUnit, false},
		{" PHPUnit ", TestingPHPUnit, false},
		{"jest", "", true},
	} {
		got, err := NormalizeTesting(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeTesting(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("NormalizeTesting(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}

	// PHPUnit is the "do nothing" branch: the skeleton already ships it, so no
	// command may be attempted against a directory that does not exist.
	if err := configureTesting(filepath.Join(t.TempDir(), "missing"), TestingPHPUnit, nil); err != nil {
		t.Errorf("phpunit must not touch the project: %v", err)
	}
}

// TestNormalizeDatabase: SQLite is the default, exactly as `laravel new` treats
// it, and "mysql" is accepted as the same thing as "mariadb" because that is
// what Laravel's .env calls it.
func TestNormalizeDatabase(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", DBDefault, false},
		{"sqlite", DBDefault, false},
		{"SQLite", DBDefault, false},
		{"mariadb", DBMariaDB, false},
		{"mysql", DBMariaDB, false},
		{" MariaDB ", DBMariaDB, false},
		{"pgsql", "", true},
		{"sqlsrv", "", true},
		{"nonsense", "", true},
	} {
		got, err := NormalizeDatabase(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeDatabase(%q) = %q, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("NormalizeDatabase(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

// TestDatabaseName mirrors Laravel's installer: lower-cased, dashes replaced,
// because a dash is not valid in a database name.
func TestDatabaseName(t *testing.T) {
	for in, want := range map[string]string{
		"myapp":       "myapp",
		"SabKit-Test": "sabkit_test",
		"My-Cool-App": "my_cool_app",
	} {
		if got := DatabaseName(in); got != want {
			t.Errorf("DatabaseName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPatchDotenvHandlesCommentedLines is the case that matters: Laravel ships
// the MySQL settings commented out because SQLite is the default, so a patcher
// that only matched live lines would leave the project on SQLite while claiming
// to use MariaDB.
func TestPatchDotenvHandlesCommentedLines(t *testing.T) {
	in := strings.Join([]string{
		"APP_NAME=Laravel",
		"DB_CONNECTION=sqlite",
		"# DB_HOST=127.0.0.1",
		"# DB_PORT=3306",
		"# DB_DATABASE=laravel",
		"# DB_USERNAME=root",
		"# DB_PASSWORD=",
		"",
	}, "\n")

	out := patchDotenv(in, map[string]string{
		"DB_CONNECTION": "mysql",
		"DB_HOST":       "127.0.0.1",
		"DB_PORT":       "3307",
		"DB_DATABASE":   "sabkit_test",
		"DB_USERNAME":   "root",
		"DB_PASSWORD":   "",
	})

	for _, want := range []string{
		"DB_CONNECTION=mysql",
		"DB_HOST=127.0.0.1",
		"DB_PORT=3307",
		"DB_DATABASE=sabkit_test",
		"DB_USERNAME=root",
		"DB_PASSWORD=",
	} {
		if !strings.Contains(out, want+"\n") && !strings.HasSuffix(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "# DB_HOST") {
		t.Errorf("the commented form survived:\n%s", out)
	}
	if strings.Contains(out, "DB_CONNECTION=sqlite") {
		t.Errorf("the old driver survived:\n%s", out)
	}
	if strings.Contains(out, "DB_PORT=3306") {
		t.Errorf("the old port survived:\n%s", out)
	}
	// APP_NAME must be untouched.
	if !strings.Contains(out, "APP_NAME=Laravel") {
		t.Errorf("unrelated keys were disturbed:\n%s", out)
	}
}

// TestPatchDotenvAppendsMissingKeys: a kit may ship an .env that never mentions
// the server settings at all.
func TestPatchDotenvAppendsMissingKeys(t *testing.T) {
	out := patchDotenv("APP_NAME=Laravel\nDB_CONNECTION=sqlite\n", map[string]string{
		"DB_CONNECTION": "mysql",
		"DB_DATABASE":   "myapp",
	})
	if !strings.Contains(out, "DB_CONNECTION=mysql") {
		t.Errorf("existing key not replaced:\n%s", out)
	}
	if !strings.Contains(out, "DB_DATABASE=myapp") {
		t.Errorf("missing key not appended:\n%s", out)
	}
	// Appended keys must be sorted so the file is stable between runs.
	if strings.Index(out, "DB_DATABASE=myapp") < strings.Index(out, "DB_CONNECTION=mysql") {
		t.Errorf("no rewrite should be needed here, but order is unstable:\n%s", out)
	}

	// Idempotent: patching an already-patched file changes nothing.
	once := patchDotenv("DB_CONNECTION=sqlite\n", map[string]string{"DB_CONNECTION": "mysql", "DB_HOST": "127.0.0.1"})
	twice := patchDotenv(once, map[string]string{"DB_CONNECTION": "mysql", "DB_HOST": "127.0.0.1"})
	if once != twice {
		t.Errorf("not idempotent:\n%q\n%q", once, twice)
	}
}

// TestDotenvKeyIgnoresNonAssignments keeps comments and blank lines out of the
// way, and stops a quoted value from being read as a key.
func TestDotenvKeyIgnoresNonAssignments(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"APP_NAME=Laravel", "APP_NAME"},
		{"# DB_HOST=127.0.0.1", "DB_HOST"},
		{"  DB_PORT = 3306", "DB_PORT"},
		{"", ""},
		{"# just a comment", ""},
		{"=", ""},
		{"no equals sign", ""},
	} {
		if got := dotenvKey(tc.line); got != tc.want {
			t.Errorf("dotenvKey(%q) = %q, want %q", tc.line, got, tc.want)
		}
	}
}

// TestConfigureDatabaseRejectsUnknownDrivers: the caller must hear about a
// choice Sabdopalon cannot honour instead of getting a project it did not ask
// for.
func TestConfigureDatabaseRejectsUnknownDrivers(t *testing.T) {
	if _, err := configureDatabase(t.TempDir(), "myapp", "pgsql", nil); err == nil {
		t.Error("pgsql must be rejected: Sabdopalon does not manage a PostgreSQL server")
	}
	// SQLite is a no-op that needs no database and no migration.
	needs, err := configureDatabase(t.TempDir(), "myapp", "", nil)
	if err != nil || needs {
		t.Errorf("sqlite default = (%v, %v), want (false, nil)", needs, err)
	}
}
