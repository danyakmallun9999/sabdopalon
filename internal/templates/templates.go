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
    --bg: #090a0f;
    --frame: #10121a;
    --cell-bg: #121520;
    --cell-hover: #161a27;
    --border: #1e2436;
    --border-light: #2a334c;
    --text-head: #f8fafc;
    --text-body: #94a3b8;
    --text-muted: #64748b;
    --accent: #38bdf8;
    --accent-bg: #082f49;
    --accent-border: #0369a1;
    --success: #10b981;
    --success-bg: #022c22;
    --success-border: #065f46;
    --warning: #f59e0b;
    --warning-bg: #2d1800;
    --warning-border: #78350f;
    --font-sans: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    --font-mono: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  }

  * {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  html, body {
    height: 100%;
  }

  body {
    font-family: var(--font-sans);
    background-color: var(--bg);
    color: var(--text-body);
    line-height: 1.45;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1.25rem;
    position: relative;
    overflow-x: hidden;
  }

  .grid-bg {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    pointer-events: none;
    z-index: 0;
    opacity: 0.65;
  }

  .console {
    position: relative;
    z-index: 1;
    width: 100%;
    max-width: 58rem;
    background-color: var(--frame);
    border: 1px solid var(--border);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.4);
  }

  .console-top {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.85rem 1.25rem;
    border-bottom: 1px solid var(--border);
  }

  .brand {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    text-decoration: none;
    color: var(--text-head);
    font-weight: 700;
    font-size: 1rem;
    letter-spacing: -0.01em;
  }

  .brand-icon {
    font-size: 1.2rem;
    line-height: 1;
  }

  .top-meta {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .domain-tag {
    font-family: var(--font-mono);
    font-size: 0.75rem;
    color: var(--accent);
    background-color: var(--accent-bg);
    border: 1px solid var(--accent-border);
    padding: 0.2rem 0.55rem;
    border-radius: 4px;
  }

  .status-tag {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    background-color: var(--success-bg);
    color: var(--success);
    border: 1px solid var(--success-border);
    padding: 0.2rem 0.6rem;
    border-radius: 9999px;
    font-size: 0.725rem;
    font-weight: 600;
    font-family: var(--font-mono);
  }

  .status-dot {
    width: 0.4rem;
    height: 0.4rem;
    background-color: var(--success);
    border-radius: 9999px;
  }

  .console-hero {
    padding: 1.5rem 1.5rem 1.25rem;
    display: grid;
    grid-template-columns: 1fr auto;
    gap: 1.5rem;
    align-items: center;
    border-bottom: 1px solid var(--border);
  }

  .hero-text {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .hero-text h1 {
    font-size: 1.45rem;
    font-weight: 700;
    letter-spacing: -0.025em;
    color: var(--text-head);
    line-height: 1.2;
  }

  .hero-text p {
    font-size: 0.875rem;
    color: var(--text-body);
  }

  .file-box {
    background-color: var(--cell-bg);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 0.65rem 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    min-width: 17rem;
  }

  .file-box-lbl {
    font-family: var(--font-mono);
    font-size: 0.65rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--text-muted);
  }

  .file-box-path {
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    color: var(--text-head);
    word-break: break-all;
  }

  .console-stack {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    border-bottom: 1px solid var(--border);
  }

  .stack-cell {
    padding: 1.25rem 1.5rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 0.85rem;
  }

  .stack-cell:not(:last-child) {
    border-right: 1px solid var(--border);
  }

  .stack-cell-head {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .stack-icon {
    font-size: 1.1rem;
    line-height: 1;
  }

  .stack-cell-body {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .stack-val {
    font-size: 1.2rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--text-head);
  }

  .stack-desc {
    font-size: 0.775rem;
    color: var(--text-body);
    line-height: 1.4;
  }

  .pill {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    font-family: var(--font-mono);
    font-size: 0.7rem;
    font-weight: 600;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    width: fit-content;
  }

  .pill-ok {
    background-color: var(--success-bg);
    color: var(--success);
    border: 1px solid var(--success-border);
  }

  .pill-warn {
    background-color: var(--warning-bg);
    color: var(--warning);
    border: 1px solid var(--warning-border);
  }

  .action-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background-color: #1e293b;
    color: #f8fafc;
    border: 1px solid #334155;
    text-decoration: none;
    font-size: 0.775rem;
    font-weight: 600;
    padding: 0.35rem 0.75rem;
    border-radius: 4px;
    transition: background-color 0.15s ease, border-color 0.15s ease;
    width: fit-content;
  }

  .action-btn:hover {
    background-color: #334155;
    border-color: #475569;
  }

  .console-specs {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    padding: 0.85rem 1.5rem;
    gap: 1.25rem;
    background-color: var(--cell-bg);
    border-bottom: 1px solid var(--border);
    font-size: 0.775rem;
  }

  .spec-entry {
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
  }

  .spec-lbl {
    font-weight: 600;
    color: var(--text-head);
    white-space: nowrap;
  }

  .spec-val {
    color: var(--text-body);
  }

  .spec-val code {
    font-family: var(--font-mono);
    color: var(--accent);
  }

  .console-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.75rem 1.5rem;
    font-size: 0.775rem;
  }

  .footer-nav {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .footer-nav a {
    color: var(--text-body);
    text-decoration: none;
    transition: color 0.15s ease;
  }

  .footer-nav a:hover {
    color: var(--text-head);
  }

  .footer-author {
    color: var(--text-muted);
  }

  .footer-author a {
    color: var(--text-body);
    text-decoration: underline;
    text-underline-offset: 2px;
  }

  .footer-author a:hover {
    color: var(--text-head);
  }

  @media (max-width: 768px) {
    body {
      height: auto;
      padding: 1rem;
    }
    .console-hero {
      grid-template-columns: 1fr;
    }
    .console-stack {
      grid-template-columns: 1fr;
    }
    .stack-cell:not(:last-child) {
      border-right: none;
      border-bottom: 1px solid var(--border);
    }
    .console-specs {
      grid-template-columns: 1fr;
      gap: 0.5rem;
    }
    .console-footer {
      flex-direction: column;
      gap: 0.6rem;
      text-align: center;
    }
  }
</style>
</head>
<body>
  <svg class="grid-bg" xmlns="http://www.w3.org/2000/svg" width="100%" height="100%">
    <defs>
      <pattern id="grid-pattern" width="80" height="80" patternUnits="userSpaceOnUse">
        <path d="M 80 0 L 0 0 0 80" fill="none" stroke="#161a26" stroke-width="1"/>
        <circle cx="80" cy="80" r="1.5" fill="#252c40"/>
      </pattern>
    </defs>
    <rect width="100%" height="100%" fill="url(#grid-pattern)" />
    <line x1="12%" y1="0" x2="12%" y2="100%" stroke="#1c2233" stroke-width="1" stroke-dasharray="4 6"/>
    <line x1="28%" y1="0" x2="28%" y2="100%" stroke="#1c2233" stroke-width="1"/>
    <line x1="50%" y1="0" x2="50%" y2="100%" stroke="#242c42" stroke-width="1" stroke-dasharray="8 4"/>
    <line x1="72%" y1="0" x2="72%" y2="100%" stroke="#1c2233" stroke-width="1"/>
    <line x1="88%" y1="0" x2="88%" y2="100%" stroke="#1c2233" stroke-width="1" stroke-dasharray="4 6"/>
    <line x1="0" y1="120" x2="100%" y2="120" stroke="#1c2233" stroke-width="1" stroke-dasharray="3 6"/>
    <line x1="0" y1="360" x2="100%" y2="360" stroke="#1c2233" stroke-width="1" stroke-dasharray="3 6"/>
    <line x1="0" y1="640" x2="100%" y2="640" stroke="#1c2233" stroke-width="1" stroke-dasharray="3 6"/>
    <text x="12.5%" y="30" fill="#2d3752" font-size="9" font-family="monospace">110&deg; 22&apos; E</text>
    <text x="50.5%" y="30" fill="#394566" font-size="9" font-family="monospace">MERIDIAN 0.0</text>
    <text x="88.5%" y="30" fill="#2d3752" font-size="9" font-family="monospace">112&deg; 45&apos; E</text>
  </svg>

  <main class="console">
    <div class="console-top">
      <a class="brand" href="http://localhost:9900" title="Buka Dashboard Sabdopalon">
        <span class="brand-icon">&#128042;</span>
        <span>Sabdopalon</span>
      </a>
      <div class="top-meta">
        <span class="domain-tag"><?= htmlspecialchars($site) ?>.localhost</span>
        <div class="status-tag">
          <span class="status-dot"></span>
          <span>SISTEM AKTIF</span>
        </div>
      </div>
    </div>

    <div class="console-hero">
      <div class="hero-text">
        <h1>Situs siap dikembangkan</h1>
        <p>Virtual host dialokasikan otomatis oleh Sabdopalon. Siap menerima request PHP.</p>
      </div>
      <div class="file-box">
        <span class="file-box-lbl">Titik Masuk Utama</span>
        <span class="file-box-path">sites/<?= htmlspecialchars($site) ?>/public/index.php</span>
      </div>
    </div>

    <div class="console-stack">
      <div class="stack-cell">
        <div class="stack-cell-head">
          <span class="stack-icon">&#128024;</span>
          <span>PHP Runtime</span>
        </div>
        <div class="stack-cell-body">
          <div class="stack-val"><?= htmlspecialchars($phpVer) ?></div>
          <p class="stack-desc">Interpreter aktif melayani HTTP.</p>
        </div>
        <span class="pill pill-ok">&#9679; Aktif</span>
      </div>

      <div class="stack-cell">
        <div class="stack-cell-head">
          <span class="stack-icon">&#128452;&#65039;</span>
          <span><?= htmlspecialchars($db['label']) ?></span>
        </div>
        <div class="stack-cell-body">
          <?php if ($db['ok']): ?>
            <div class="stack-val"><?= htmlspecialchars($db['ver']) ?></div>
            <p class="stack-desc">127.0.0.1:3306 (user: <code>root</code>)</p>
          <?php else: ?>
            <div class="stack-val" style="color: var(--warning);">Offline</div>
            <p class="stack-desc">Layanan database belum aktif.</p>
          <?php endif; ?>
        </div>
        <?php if ($db['ok']): ?>
          <span class="pill pill-ok">&#9679; Terhubung</span>
        <?php else: ?>
          <span class="pill pill-warn">&#9675; Tidak Terhubung</span>
        <?php endif; ?>
      </div>

      <div class="stack-cell">
        <div class="stack-cell-head">
          <span class="stack-icon">&#9881;&#65039;</span>
          <span>phpMyAdmin</span>
        </div>
        <div class="stack-cell-body">
          <div class="stack-val">Basis Data</div>
          <p class="stack-desc">Kelola skema &amp; tabel SQL visual.</p>
        </div>
        <a class="action-btn" href="<?= htmlspecialchars($pmaUrl) ?>">Buka phpMyAdmin &rarr;</a>
      </div>
    </div>

    <div class="console-specs">
      <div class="spec-entry">
        <span class="spec-lbl">Dokumen Root:</span>
        <span class="spec-val"><code>public/</code></span>
      </div>
      <div class="spec-entry">
        <span class="spec-lbl">Virtual Host:</span>
        <span class="spec-val"><code><?= htmlspecialchars($site) ?>.localhost</code></span>
      </div>
      <div class="spec-entry">
        <span class="spec-lbl">Dashboard:</span>
        <span class="spec-val"><code>localhost:9900</code></span>
      </div>
    </div>

    <div class="console-footer">
      <nav class="footer-nav">
        <a href="https://github.com/danyakmallun9999/sabdopalon" target="_blank" rel="noopener">&#9733; GitHub</a>
        <a href="https://github.com/danyakmallun9999/sabdopalon/releases" target="_blank" rel="noopener">&#8595; Unduh Rilis</a>
        <a href="http://localhost:9900" target="_blank" rel="noopener">&#9776; Dashboard</a>
      </nav>
      <div class="footer-author">
        Dibuat oleh <a href="https://danyakmallun.dev" target="_blank" rel="noopener">danyakmallun</a> &middot; Sabdopalon
      </div>
    </div>
  </main>
</body>
</html>
`

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
	dir := filepath.Join(sitesDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	fmt.Printf("  ⏳  Running composer create-project codeigniter4/appstarter...\n")
	return runCommand(sitesDir, "composer", "create-project", "codeigniter4/appstarter", name, "--prefer-dist")
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	winproc.Quiet(cmd)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
