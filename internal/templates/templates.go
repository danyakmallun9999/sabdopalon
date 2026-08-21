// Package templates provides project scaffolding for common PHP frameworks.
// Running `sabdopalon new <template> <name>` creates a new site folder with
// the framework's starter files pre-configured for the Sabdopalon environment.
package templates

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Template describes a project template.
type Template struct {
	Name        string
	Description string
	Setup       func(sitesDir, name string) error
}

// Registry of available templates.
var Registry = []Template{
	{
		Name:        "blank",
		Description: "Empty PHP site with a hello-world index.php",
		Setup:       setupBlank,
	},
	{
		Name:        "laravel",
		Description: "Laravel project (requires composer, runs composer create-project)",
		Setup:       setupLaravel,
	},
	{
		Name:        "wordpress",
		Description: "WordPress installation (downloads latest WP)",
		Setup:       setupWordPress,
	},
	{
		Name:        "codeigniter",
		Description: "CodeIgniter 4 starter (requires composer)",
		Setup:       setupCodeIgniter,
	},
}

// Create sets up a new project from a template.
func Create(sitesDir, templateName, projectName string) error {
	// Validate project name
	if projectName == "" {
		return fmt.Errorf("project name is required")
	}
	if strings.ContainsAny(projectName, "/\\") {
		return fmt.Errorf("project name must not contain path separators")
	}

	tmpl := findTemplate(templateName)
	if tmpl == nil {
		return fmt.Errorf("unknown template: %s\nAvailable: %s", templateName, ListNames())
	}

	projectDir := filepath.Join(sitesDir, projectName)
	if _, err := os.Stat(projectDir); err == nil {
		return fmt.Errorf("project already exists: %s", projectDir)
	}

	fmt.Printf("  📦  Creating %s project '%s'...\n", templateName, projectName)
	if err := tmpl.Setup(sitesDir, projectName); err != nil {
		return err
	}
	fmt.Printf("  ✓  Project created at sites/%s/\n", projectName)
	fmt.Printf("  ✦  Visit: http://%s.localhost/\n", projectName)
	return nil
}

// ListNames returns a comma-separated list of template names.
func ListNames() string {
	names := make([]string, len(Registry))
	for i, t := range Registry {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

// findTemplate looks up a template by name (case-insensitive).
func findTemplate(name string) *Template {
	for i, t := range Registry {
		if strings.EqualFold(t.Name, name) {
			return &Registry[i]
		}
	}
	return nil
}

// --- Template implementations ---

func setupBlank(sitesDir, name string) error {
	dir := filepath.Join(sitesDir, name, "public")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`<?php
// %s — created by Sabdopalon
echo "<h1>Hello from %s!</h1>";
echo "<p>Served by Sabdopalon. PHP " . phpversion() . "</p>";
echo "<p>Edit this file at sites/%s/public/index.php</p>";
`, name, name, name)
	return os.WriteFile(filepath.Join(dir, "index.php"), []byte(content), 0o644)
}

func setupLaravel(sitesDir, name string) error {
	// We use composer create-project, then adjust the docroot.
	// Laravel's public/ is the docroot, which matches Sabdopalon's convention.
	dir := filepath.Join(sitesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Printf("  ⏳  Running composer create-project laravel/laravel...\n")
	// Run: composer create-project laravel/laravel <name> --prefer-dist
	// in the sites dir
	return runCommand(sitesDir, "composer", "create-project", "laravel/laravel", name, "--prefer-dist")
}

func setupWordPress(sitesDir, name string) error {
	dir := filepath.Join(sitesDir, name, "public")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Download WordPress using wp-cli if available, otherwise direct download
	fmt.Printf("  ⏳  Downloading latest WordPress...\n")
	// Use the official tarball
	if err := runCommand(dir, "curl", "-sL", "https://wordpress.org/latest.tar.gz", "-o", "wp.tar.gz"); err != nil {
		return fmt.Errorf("download WordPress: %w", err)
	}
	if err := runCommand(dir, "tar", "xzf", "wp.tar.gz", "--strip-components=1"); err != nil {
		return fmt.Errorf("extract WordPress: %w", err)
	}
	_ = os.Remove(filepath.Join(dir, "wp.tar.gz"))
	// Create wp-config.php with Sabdopalon DB settings
	config := "<?php\n" +
		"// Sabdopalon-generated wp-config.php\n" +
		"define('DB_NAME', 'sabdopalon');\n" +
		"define('DB_USER', 'root');\n" +
		"define('DB_PASSWORD', '');\n" +
		"define('DB_HOST', '127.0.0.1:3306');\n" +
		"define('DB_CHARSET', 'utf8mb4');\n" +
		"define('DB_COLLATE', '');\n\n" +
		"$table_prefix = 'wp_';\n\n" +
		"if (!defined('ABSPATH')) define('ABSPATH', __DIR__ . '/');\n" +
		"require_once ABSPATH . 'wp-settings.php';\n"
	return os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte(config), 0o644)
}

func setupCodeIgniter(sitesDir, name string) error {
	dir := filepath.Join(sitesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Printf("  ⏳  Running composer create-project codeigniter4/appstarter...\n")
	return runCommand(sitesDir, "composer", "create-project", "codeigniter4/appstarter", name, "--prefer-dist")
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
