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

	"github.com/sabdopalon/sabdopalon/internal/database"
	"github.com/sabdopalon/sabdopalon/internal/winproc"
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
	// Validate project name (normalized to lowercase — site folders are
	// addressed via lowercase hostnames)
	projectName = strings.ToLower(strings.TrimSpace(projectName))
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
	content := strings.ReplaceAll(blankLandingPage, "{{NAME}}", name)
	return os.WriteFile(filepath.Join(dir, "index.php"), []byte(content), 0o644)
}

// blankLandingPage is the starter landing page for every fresh site: it
// celebrates a working install, reports the live stack (PHP / database /
// phpMyAdmin) and credits the project. Zero external assets — fully offline.
const blankLandingPage = `<?php
// {{NAME}} — created by Sabdopalon
declare(strict_types=1);

$site   = '{{NAME}}';
$phpVer = PHP_VERSION;

// --- database probe (bundled MariaDB: root@127.0.0.1, no password) --------
$db = ['ok' => false, 'ver' => '', 'label' => 'MariaDB'];
try {
    $pdo = new PDO(
        'mysql:host=127.0.0.1;port=3306;charset=utf8mb4',
        'root',
        '',
        [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION, PDO::ATTR_TIMEOUT => 2]
    );
    $db['ok']  = true;
    $db['ver'] = (string) $pdo->query('SELECT VERSION()')->fetchColumn();
} catch (Throwable $e) {
    $db['label'] = 'Database tidak aktif';
}

// --- phpMyAdmin link derived from the host we were reached on -------------
$hostPart = (string) ($_SERVER['HTTP_HOST'] ?? 'localhost');
$hostname = strtok($hostPart, ':') ?: 'localhost';
$suffix   = str_contains($hostPart, ':') ? substr($hostPart, (int) strpos($hostPart, ':')) : '';
$tld      = str_contains($hostname, '.') ? substr($hostname, (int) strpos($hostname, '.') + 1) : 'localhost';
$pmaUrl   = 'http://phpmyadmin.' . $tld . $suffix;
?>
<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title><?= htmlspecialchars($site) ?> &middot; Sabdopalon</title>
<style>
  :root {
    --bg: #030712;
    --card-bg: #080d1a;
    --border: #1f293d;
    --border-subtle: #111827;
    --text-title: #f8fafc;
    --text-body: #94a3b8;
    --text-muted: #64748b;
    --accent: #38bdf8;
    --accent-bg: rgba(56, 189, 248, 0.1);
    --success: #10b981;
    --success-bg: rgba(16, 185, 129, 0.12);
    --warning: #f59e0b;
    --warning-bg: rgba(245, 158, 11, 0.12);
    --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  html, body {
    min-height: 100vh;
    background-color: var(--bg);
    color: var(--text-body);
    font-family: var(--font-sans);
    line-height: 1.5;
    -webkit-font-smoothing: antialiased;
  }

  .page-layout {
    display: grid;
    grid-template-columns: 32px minmax(0, 1fr) 32px;
    grid-template-rows: auto 16px 1fr 16px auto;
    min-height: 100vh;
    width: 100%;
    background-color: var(--bg);
  }

  .gutter {
    background-color: var(--bg);
    background-image: repeating-linear-gradient(
      -45deg,
      rgba(255, 255, 255, 0.035),
      rgba(255, 255, 255, 0.035) 1px,
      transparent 1px,
      transparent 8px
    );
    z-index: 2;
  }

  .gutter-left {
    grid-column: 1;
    grid-row: 1 / -1;
    border-right: 1px solid var(--border);
  }

  .gutter-right {
    grid-column: 3;
    grid-row: 1 / -1;
    border-left: 1px solid var(--border);
  }

  .navbar {
    grid-column: 2;
    grid-row: 1;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 1rem 2rem;
    max-width: 86rem;
    width: 100%;
    margin: 0 auto;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .hatched-divider {
    grid-column: 1 / -1;
    height: 16px;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    background-color: var(--bg);
    background-image: repeating-linear-gradient(
      -45deg,
      rgba(255, 255, 255, 0.035),
      rgba(255, 255, 255, 0.035) 1px,
      transparent 1px,
      transparent 8px
    );
    z-index: 1;
  }

  .divider-nav {
    grid-row: 2;
  }

  .divider-footer {
    grid-row: 4;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 1.25rem;
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    text-decoration: none;
    color: var(--text-title);
    font-weight: 700;
    font-size: 1.15rem;
    letter-spacing: -0.02em;
  }

  .brand-tag {
    font-weight: 400;
    color: var(--accent);
    font-size: 0.85rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .announcement-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    background-color: rgba(255, 255, 255, 0.04);
    border: 1px solid rgba(255, 255, 255, 0.1);
    color: #e2e8f0;
    padding: 0.35rem 0.85rem;
    border-radius: 9999px;
    font-size: 0.8125rem;
    text-decoration: none;
    transition: border-color 0.15s ease, background-color 0.15s ease;
  }

  .announcement-pill:hover {
    background-color: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.2);
  }

  .sparkle {
    color: var(--accent);
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 1.5rem;
  }

  .nav-links {
    display: flex;
    align-items: center;
    gap: 1.25rem;
    font-size: 0.875rem;
    font-weight: 500;
  }

  .nav-links a {
    color: var(--text-body);
    text-decoration: none;
    transition: color 0.15s ease;
  }

  .nav-links a:hover {
    color: var(--text-title);
  }

  .btn-primary {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background-color: #f8fafc;
    color: #030712;
    text-decoration: none;
    font-size: 0.85rem;
    font-weight: 600;
    padding: 0.45rem 1rem;
    border-radius: 9999px;
    transition: background-color 0.15s ease;
  }

  .btn-primary:hover {
    background-color: #e2e8f0;
  }

  .btn-secondary {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background-color: rgba(255, 255, 255, 0.05);
    color: #f8fafc;
    border: 1px solid rgba(255, 255, 255, 0.12);
    text-decoration: none;
    font-size: 0.85rem;
    font-weight: 600;
    padding: 0.55rem 1.15rem;
    border-radius: 9999px;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .btn-secondary:hover {
    background-color: rgba(255, 255, 255, 0.1);
    border-color: rgba(255, 255, 255, 0.25);
  }

  .hero-container {
    grid-column: 2;
    grid-row: 3;
    display: grid;
    grid-template-columns: 1.05fr 1.15fr;
    gap: 3rem;
    padding: 3.5rem 2rem 4rem;
    align-items: center;
    max-width: 86rem;
    margin: 0 auto;
    width: 100%;
  }

  .hero-left {
    display: flex;
    flex-direction: column;
  }

  .eyebrow {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.12em;
    text-transform: uppercase;
    color: var(--text-muted);
    margin-bottom: 1rem;
  }

  .hero-title {
    font-size: clamp(2.2rem, 4vw, 3.4rem);
    font-weight: 700;
    letter-spacing: -0.035em;
    line-height: 1.12;
    color: var(--text-title);
    margin-bottom: 1.5rem;
  }

  .tech-bar {
    display: flex;
    align-items: center;
    gap: 1.75rem;
    padding: 0.9rem 0;
    border-top: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    margin-bottom: 1.5rem;
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-title);
    flex-wrap: wrap;
  }

  .tech-item {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .tech-status {
    font-size: 0.7rem;
    font-family: var(--font-mono);
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
  }

  .status-ok {
    color: var(--success);
    background-color: var(--success-bg);
  }

  .status-warn {
    color: var(--warning);
    background-color: var(--warning-bg);
  }

  .hero-desc {
    color: var(--text-body);
    font-size: 1rem;
    line-height: 1.65;
    margin-bottom: 2rem;
    max-width: 36rem;
  }

  .hero-desc code {
    font-family: var(--font-mono);
    color: #e2e8f0;
    background-color: rgba(255, 255, 255, 0.06);
    padding: 0.15rem 0.4rem;
    border-radius: 4px;
    font-size: 0.9em;
  }

  .hero-actions {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .hero-right {
    position: relative;
    border-radius: 1.25rem;
    border: 1px solid var(--border);
    background-color: #060a14;
    overflow: hidden;
    height: 480px;
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.08), 0 20px 40px rgba(0, 0, 0, 0.6);
  }

  .showcase-grid {
    position: absolute;
    inset: -30px -40px -40px -30px;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.25rem;
    transform: rotate(-14deg) skewX(8deg) scale(0.98);
    transform-origin: center center;
    pointer-events: none;
    user-select: none;
  }

  .mock-card {
    background-color: #0f172a;
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    box-shadow: 0 10px 25px rgba(0, 0, 0, 0.5);
  }

  .mock-card-light {
    background-color: #f1f5f9;
    color: #0f172a;
    border-color: rgba(0, 0, 0, 0.1);
  }

  .mock-card-light .mock-title {
    color: #0f172a;
  }

  .mock-card-light .mock-sub {
    color: #475569;
  }

  .mock-card-light .mock-code {
    background-color: #e2e8f0;
    color: #0f172a;
  }

  .mock-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .mock-title {
    font-size: 0.95rem;
    font-weight: 700;
    color: #f8fafc;
  }

  .mock-pill {
    font-size: 0.65rem;
    font-family: var(--font-mono);
    font-weight: 600;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
  }

  .mock-sub {
    font-size: 0.775rem;
    color: #94a3b8;
    line-height: 1.45;
  }

  .mock-code {
    font-family: var(--font-mono);
    font-size: 0.725rem;
    background-color: #090d16;
    padding: 0.5rem 0.65rem;
    border-radius: 6px;
    color: #38bdf8;
    border: 1px solid rgba(255, 255, 255, 0.05);
  }

  .footer {
    grid-column: 2;
    grid-row: 5;
    padding: 1.25rem 2rem;
    display: flex;
    align-items: center;
    justify-content: space-between;
    max-width: 86rem;
    width: 100%;
    margin: 0 auto;
    font-size: 0.8125rem;
    color: var(--text-muted);
    flex-wrap: wrap;
    gap: 1rem;
  }

  .footer-links {
    display: flex;
    align-items: center;
    gap: 1.25rem;
  }

  .footer-links a {
    color: var(--text-body);
    text-decoration: none;
  }

  .footer-links a:hover {
    color: var(--text-title);
  }

  .footer-author a {
    color: var(--text-body);
    text-decoration: underline;
  }

  @media (max-width: 1024px) {
    .page-layout {
      grid-template-columns: 1fr;
      grid-template-rows: auto auto 1fr auto auto;
    }
    .gutter {
      display: none;
    }
    .hero-container {
      grid-template-columns: 1fr;
      gap: 2.5rem;
      padding: 2rem 1.25rem;
    }
    .navbar {
      padding: 1rem 1.25rem;
    }
    .footer {
      padding: 1.25rem;
    }
  }
</style>
</head>
<body>
  <div class="page-layout">
    <div class="gutter gutter-left"></div>

    <header class="navbar">
      <div class="nav-left">
        <a class="brand" href="http://localhost:9900" title="Dashboard Sabdopalon">
          <span>&#128042;</span>
          <span>sabdopalon</span>
          <span class="brand-tag">local</span>
        </a>
        <a class="announcement-pill" href="http://localhost:9900">
          <span class="sparkle">&#10022;</span>
          <span>Environment Aktif &middot; <?= htmlspecialchars($site) ?>.localhost &rsaquo;</span>
        </a>
      </div>

      <nav class="nav-right">
        <div class="nav-links">
          <a href="http://localhost:9900">Dashboard</a>
          <a href="<?= htmlspecialchars($pmaUrl) ?>">phpMyAdmin</a>
          <a href="https://github.com/danyakmallun9999/sabdopalon" target="_blank" rel="noopener">GitHub</a>
        </div>
        <a class="btn-primary" href="<?= htmlspecialchars($pmaUrl) ?>">Buka phpMyAdmin &nearr;</a>
      </nav>
    </header>

    <div class="hatched-divider divider-nav"></div>

    <main class="hero-container">
      <div class="hero-left">
        <div class="eyebrow">LOCAL DEVELOPMENT ENVIRONMENT &middot; SABDOPALON</div>
        <h1 class="hero-title">Situs <?= htmlspecialchars($site) ?>, siap dikembangkan.</h1>

        <div class="tech-bar">
          <div class="tech-item">
            <span>&#128024; PHP <?= htmlspecialchars($phpVer) ?></span>
            <span class="tech-status status-ok">Aktif</span>
          </div>
          <div class="tech-item">
            <span>&#128452;&#65039; <?= htmlspecialchars($db['label']) ?></span>
            <?php if ($db['ok']): ?>
              <span class="tech-status status-ok"><?= htmlspecialchars($db['ver']) ?></span>
            <?php else: ?>
              <span class="tech-status status-warn">Offline</span>
            <?php endif; ?>
          </div>
          <div class="tech-item">
            <span>&#9881;&#65039; phpMyAdmin</span>
          </div>
        </div>

        <p class="hero-desc">
          Lingkungan lokal Sabdopalon menyajikan situs ini langsung melalui reverse proxy mandiri.
          Mulai membangun aplikasi dengan mengedit berkas di <code>sites/<?= htmlspecialchars($site) ?>/public/index.php</code>.
        </p>

        <div class="hero-actions">
          <a class="btn-primary" href="<?= htmlspecialchars($pmaUrl) ?>">Buka phpMyAdmin &nearr;</a>
          <a class="btn-secondary" href="http://localhost:9900">Dashboard Sabdopalon</a>
        </div>
      </div>

      <div class="hero-right">
        <div class="showcase-grid">
          <div class="mock-card mock-card-light">
            <div class="mock-head">
              <span class="mock-title">&#128024; PHP Runtime</span>
              <span class="mock-pill status-ok">● <?= htmlspecialchars($phpVer) ?></span>
            </div>
            <p class="mock-sub">Interpreter aktif melayani HTTP request pada folder root.</p>
            <div class="mock-code">docroot: sites/<?= htmlspecialchars($site) ?>/public</div>
          </div>

          <div class="mock-card">
            <div class="mock-head">
              <span class="mock-title">&#128452;&#65039; <?= htmlspecialchars($db['label']) ?></span>
              <?php if ($db['ok']): ?>
                <span class="mock-pill status-ok">● 127.0.0.1:3306</span>
              <?php else: ?>
                <span class="mock-pill status-warn">○ Tidak Terhubung</span>
              <?php endif; ?>
            </div>
            <p class="mock-sub">Koneksi basis data MySQL/MariaDB dengan pengguna <code>root</code>.</p>
            <div class="mock-code"><?= $db['ok'] ? 'server: ' . htmlspecialchars($db['ver']) : 'status: offline' ?></div>
          </div>

          <div class="mock-card">
            <div class="mock-head">
              <span class="mock-title">&#9881;&#65039; phpMyAdmin</span>
              <span class="mock-pill status-ok">GUI SQL</span>
            </div>
            <p class="mock-sub">Kelola tabel, skema, impor, dan ekspor SQL dengan mudah.</p>
            <div class="mock-code">url: <?= htmlspecialchars($pmaUrl) ?></div>
          </div>

          <div class="mock-card mock-card-light">
            <div class="mock-head">
              <span class="mock-title">&#127760; Virtual Host</span>
              <span class="mock-pill status-ok">Port 80/443</span>
            </div>
            <p class="mock-sub">Alamat lokal terdaftar otomatis tanpa edit hosts manual.</p>
            <div class="mock-code">host: http://<?= htmlspecialchars($site) ?>.localhost</div>
          </div>
        </div>
      </div>
    </main>

    <div class="hatched-divider divider-footer"></div>

    <footer class="footer">
      <div class="footer-links">
        <a href="https://github.com/danyakmallun9999/sabdopalon" target="_blank" rel="noopener">&#9733; GitHub</a>
        <a href="https://github.com/danyakmallun9999/sabdopalon/releases" target="_blank" rel="noopener">&#8595; Unduh Rilis</a>
        <a href="http://localhost:9900" target="_blank" rel="noopener">&#9776; Dashboard (:9900)</a>
      </div>
      <div class="footer-author">
        Dibuat oleh <a href="https://danyakmallun.dev" target="_blank" rel="noopener">danyakmallun</a> &middot; Sabdopalon
      </div>
    </footer>

    <div class="gutter gutter-right"></div>
  </div>
</body>
</html>
`

func setupLaravel(sitesDir, name string) error {
	// We use composer create-project, then adjust the docroot.
	// Laravel's public/ is the docroot, which matches Sabdopalon's convention.
	if err := requireComposer(); err != nil {
		return err
	}
	dir := filepath.Join(sitesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Printf("  ⏳  Running composer create-project laravel/laravel...\n")
	// Run: composer create-project laravel/laravel <name> --prefer-dist
	// in the sites dir
	if err := runCommand(sitesDir, "composer", "create-project", "laravel/laravel", name, "--prefer-dist"); err != nil {
		_ = os.RemoveAll(dir) // clean up the partial scaffold so a retry is possible
		return fmt.Errorf("composer create-project failed: %w", err)
	}
	return nil
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
		"define('DB_USER', '" + database.DatabaseRootUser + "');\n" +
		"define('DB_PASSWORD', '" + database.DatabaseRootPassword + "');\n" +
		"define('DB_HOST', '127.0.0.1:3306');\n" +
		"define('DB_CHARSET', 'utf8mb4');\n" +
		"define('DB_COLLATE', '');\n\n" +
		"$table_prefix = 'wp_';\n\n" +
		"if (!defined('ABSPATH')) define('ABSPATH', __DIR__ . '/');\n" +
		"require_once ABSPATH . 'wp-settings.php';\n"
	return os.WriteFile(filepath.Join(dir, "wp-config.php"), []byte(config), 0o644)
}

func setupCodeIgniter(sitesDir, name string) error {
	if err := requireComposer(); err != nil {
		return err
	}
	dir := filepath.Join(sitesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Printf("  ⏳  Running composer create-project codeigniter4/appstarter...\n")
	if err := runCommand(sitesDir, "composer", "create-project", "codeigniter4/appstarter", name, "--prefer-dist"); err != nil {
		_ = os.RemoveAll(dir) // clean up the partial scaffold so a retry is possible
		return fmt.Errorf("composer create-project failed: %w", err)
	}
	return nil
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	winproc.Quiet(cmd)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// requireComposer verifies that the composer executable is on PATH and returns
// a clear, actionable error when it is missing. Templates that delegate project
// scaffolding to `composer create-project` call this up front so a missing
// composer produces one helpful message instead of a raw exec error.
func requireComposer() error {
	if _, err := exec.LookPath("composer"); err != nil {
		return fmt.Errorf("composer is required but was not found on your PATH.\n" +
			"Install Composer from https://getcomposer.org/download/ and retry.")
	}
	return nil
}
