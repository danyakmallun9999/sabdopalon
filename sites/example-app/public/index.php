<?php
// Sabdopalon demo site — shows PHP + database working.
$dbEngine = getenv('SABDOPALON_DB_ENGINE') ?: 'sqlite';
$dbPath = getenv('SABDOPALON_DB_PATH') ?: __DIR__ . '/../../data/sabdopalon.db';
$dsn = '';
$connected = false;
$error = '';

// Build DSN based on engine
if ($dbEngine === 'sqlite') {
    $dsn = 'sqlite:' . $dbPath;
} else {
    // MySQL/MariaDB on localhost:3306
    $port = 3306;
    $dsn = "mysql:host=127.0.0.1;port={$port};dbname=sabdopalon;charset=utf8mb4";
}

try {
    if ($dbEngine === 'sqlite') {
        @mkdir(dirname($dbPath), 0755, true);
        $db = new PDO($dsn);
    } else {
        // root with no password (local dev default after mariadb-install-db)
        $db = new PDO($dsn, 'root', '', [PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION]);
    }
    $db->setAttribute(PDO::ATTR_ERRMODE, PDO::ERRMODE_EXCEPTION);
    $connected = true;

    // Create a visits table and insert a row
    $db->exec('CREATE TABLE IF NOT EXISTS visits (id INT AUTO_INCREMENT PRIMARY KEY, ts VARCHAR(40), ip VARCHAR(45))');
    $stmt = $db->prepare('INSERT INTO visits (ts, ip) VALUES (?, ?)');
    $stmt->execute([date('c'), $_SERVER['REMOTE_ADDR'] ?? 'unknown']);
    $count = (int) $db->query('SELECT COUNT(*) FROM visits')->fetchColumn();

    $recent = $db->query('SELECT * FROM visits ORDER BY id DESC LIMIT 5')->fetchAll(PDO::FETCH_ASSOC);
} catch (Exception $e) {
    $error = $e->getMessage();
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>example-app — Sabdopalon</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: system-ui, sans-serif; background: #0f1117; color: #e0e0e0; padding: 2rem 1rem; }
    .wrap { max-width: 680px; margin: 0 auto; }
    h1 { color: #58a6ff; }
    .meta { color: #8b949e; margin: .3rem 0 1.5rem; }
    table { width: 100%; border-collapse: collapse; margin: 1rem 0; }
    th, td { text-align: left; padding: .6rem .8rem; border-bottom: 1px solid #30363d; }
    th { color: #8b949e; font-size: .85rem; text-transform: uppercase; }
    .ok { color: #3fb950; } .fail { color: #f85149; }
    .card { background: #161b22; border-radius: 10px; padding: 1rem 1.2rem; margin: 1rem 0; }
    code { background: #1f242b; padding: .15em .4em; border-radius: 4px; font-size: .9em; }
  </style>
</head>
<body>
<div class="wrap">
  <h1>🐫 example-app</h1>
  <p class="meta">Served by Sabdopalon proxy → PHP <?= phpversion() ?> · <?= $dbEngine ?></p>

  <div class="card">
    <h3>Environment</h3>
    <table>
      <tr><th>PHP</th><td><?= phpversion() ?></td></tr>
      <tr><th>SAPI</th><td><?= php_sapi_name() ?></td></tr>
      <tr><th>Host</th><td><?= $_SERVER['HTTP_HOST'] ?></td></tr>
      <tr><th>Docroot</th><td><?= $_SERVER['DOCUMENT_ROOT'] ?></td></tr>
      <tr><th>DB Engine</th><td><?= htmlspecialchars($dbEngine) ?></td></tr>
      <tr><th>DB DSN</th><td><code><?= htmlspecialchars($dsn) ?></code></td></tr>
    </table>
  </div>

  <div class="card">
    <h3>Database Test</h3>
    <?php if ($connected): ?>
      <p class="ok">✅ <?= htmlspecialchars($dbEngine) ?> connected. Total visits: <strong><?= $count ?></strong></p>
      <table><tr><th>ID</th><th>Timestamp</th><th>IP</th></tr>
      <?php foreach ($recent as $row): ?>
        <tr><td><?= $row['id'] ?></td><td><?= $row['ts'] ?></td><td><?= $row['ip'] ?></td></tr>
      <?php endforeach; ?>
      </table>
    <?php else: ?>
      <p class="fail">❌ DB error: <?= htmlspecialchars($error) ?></p>
      <p style="color:#8b949e;font-size:.85rem;margin-top:.5rem">
        If using MariaDB/MySQL, make sure the daemon started and the database was created.
      </p>
    <?php endif; ?>
  </div>

  <div class="card">
    <h3>Loaded Extensions</h3>
    <p style="color:#8b949e;font-size:.9rem">
      <?php
      $exts = get_loaded_extensions();
      $interesting = array_filter($exts, fn($e) => in_array(strtolower($e), [
          'pdo','pdo_sqlite','pdo_mysql','pdo_pgsql','mysqli','curl','mbstring','openssl','json','zip','gd','intl'
      ]));
      echo implode(', ', $interesting ?: ['(none of interest)']);
      ?>
    </p>
  </div>
</div>
</body>
</html>
