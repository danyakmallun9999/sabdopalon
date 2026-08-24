<?php
// example-app — created by Sabdopalon
declare(strict_types=1);

$site   = 'example-app';
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
<title><?= htmlspecialchars($site) ?> · Sabdopalon</title>
<style>
  :root {
    --bg: #0b1020; --card: rgba(255,255,255,.06); --border: rgba(255,255,255,.12);
    --text: #e8ecf8; --muted: #9aa4c0; --accent: #6ee7b7; --accent2: #818cf8;
    --ok-bg: rgba(52,211,153,.12); --warn-bg: rgba(251,191,36,.12); --warn: #fbbf24;
  }
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
    min-height: 100dvh; color: var(--text);
    background:
      radial-gradient(60rem 40rem at 15% -10%, rgba(129,140,248,.22), transparent 60%),
      radial-gradient(50rem 35rem at 110% 20%, rgba(110,231,183,.16), transparent 55%),
      var(--bg);
    display: flex; flex-direction: column; align-items: center;
    padding: 4rem 1.25rem 2.5rem; gap: 2.75rem;
  }
  .hero { text-align: center; max-width: 44rem; }
  .badge {
    display: inline-flex; align-items: center; gap: .5rem;
    background: var(--ok-bg); color: var(--accent);
    border: 1px solid rgba(110,231,183,.35); border-radius: 999px;
    padding: .45rem 1rem; font-size: .85rem; font-weight: 600; letter-spacing: .02em;
  }
  .dot { width: .5rem; height: .5rem; border-radius: 999px; background: var(--accent);
         box-shadow: 0 0 0 4px rgba(110,231,183,.18); animation: pulse 2s infinite; }
  @keyframes pulse { 50% { box-shadow: 0 0 0 7px rgba(110,231,183,.05);} }
  h1 { font-size: clamp(1.9rem, 5vw, 3rem); line-height: 1.15; margin: 1.1rem 0 .7rem; letter-spacing: -.02em; }
  h1 span { background: linear-gradient(92deg, var(--accent2), var(--accent));
            -webkit-background-clip: text; background-clip: text; color: transparent; }
  .sub { color: var(--muted); font-size: 1.02rem; line-height: 1.65; }
  .grid { display: grid; gap: 1rem; width: min(56rem, 100%);
          grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr)); }
  .card {
    background: var(--card); border: 1px solid var(--border); border-radius: 1.1rem;
    padding: 1.35rem 1.4rem; backdrop-filter: blur(10px);
    display: flex; flex-direction: column; gap: .55rem; text-align: left;
    transition: transform .18s ease, border-color .18s ease;
  }
  .card:hover { transform: translateY(-3px); border-color: rgba(255,255,255,.25); }
  .card .k { display: flex; align-items: center; gap: .6rem; font-size: .8rem; font-weight: 700;
             text-transform: uppercase; letter-spacing: .09em; color: var(--muted); }
  .card .v { font-size: 1.28rem; font-weight: 700; word-break: break-all; }
  .card .n { color: var(--muted); font-size: .85rem; line-height: 1.5; }
  .state-ok   { color: var(--accent); }
  .state-warn { color: var(--warn); }
  .chip { align-self: flex-start; font-size: .78rem; font-weight: 600; border-radius: 999px;
          padding: .3rem .75rem; margin-top: .2rem; }
  .chip.ok   { background: var(--ok-bg); color: var(--accent); }
  .chip.warn { background: var(--warn-bg); color: var(--warn); }
  a.btn {
    align-self: flex-start; margin-top: .2rem; text-decoration: none; font-size: .88rem; font-weight: 600;
    color: var(--bg); background: linear-gradient(92deg, var(--accent2), var(--accent));
    padding: .5rem 1rem; border-radius: .65rem;
  }
  .powered { display: flex; flex-direction: column; align-items: center; gap: .9rem; text-align: center; }
  .camel { font-size: 2.6rem; filter: drop-shadow(0 6px 18px rgba(129,140,248,.45)); }
  .powered p { color: var(--muted); max-width: 34rem; line-height: 1.6; font-size: .95rem; }
  .links { display: flex; flex-wrap: wrap; gap: .8rem; justify-content: center; }
  .links a {
    display: inline-flex; align-items: center; gap: .45rem; text-decoration: none; font-size: .9rem; font-weight: 600;
    color: var(--text); background: var(--card); border: 1px solid var(--border);
    padding: .55rem 1.05rem; border-radius: .75rem; transition: border-color .15s ease, transform .15s ease;
  }
  .links a:hover { border-color: var(--accent2); transform: translateY(-2px); }
  footer { color: var(--muted); font-size: .85rem; text-align: center; }
  footer a { color: var(--text); font-weight: 700; text-decoration: none;
             background: linear-gradient(92deg, var(--accent2), var(--accent));
             -webkit-background-clip: text; background-clip: text; color: transparent; }
  @media (prefers-color-scheme: light) {
    :root { --bg:#f4f6fd; --card:rgba(255,255,255,.75); --border:rgba(15,23,42,.12);
            --text:#101830; --muted:#5a6484; }
  }
</style>
</head>
<body>
  <section class="hero">
    <span class="badge"><span class="dot"></span> SEMUA SISTEM BERJALAN</span>
    <h1>Situs <span><?= htmlspecialchars($site) ?></span> hidup!<br>Environment lokal kamu siap dipakai.</h1>
    <p class="sub">Halaman ini disajikan langsung oleh Sabdopalon tanpa Apache/Nginx.
       Edit file di <code>sites/<?= htmlspecialchars($site) ?>/public/index.php</code> untuk mulai membangun.</p>
  </section>

  <section class="grid">
    <div class="card">
      <span class="k">🐘 PHP</span>
      <span class="v"><?= htmlspecialchars($phpVer) ?></span>
      <span class="n">Interpreter aktif yang melayani situs ini.</span>
      <span class="chip ok">● Aktif</span>
    </div>
    <div class="card">
      <span class="k">🗄️ <?= htmlspecialchars($db['label']) ?></span>
      <?php if ($db['ok']): ?>
        <span class="v state-ok"><?= htmlspecialchars($db['ver']) ?></span>
        <span class="n">Server MariaDB di 127.0.0.1:3306 merespons koneksi.</span>
        <span class="chip ok">● Terhubung</span>
      <?php else: ?>
        <span class="v state-warn">Offline</span>
        <span class="n">Nyalakan lewat dashboard → halaman Database.</span>
        <span class="chip warn">○ Tidak terhubung</span>
      <?php endif; ?>
    </div>
    <div class="card">
      <span class="k">⚙️ phpMyAdmin</span>
      <span class="v">Kelola database</span>
      <span class="n">Antarmuka web untuk MariaDB — buat, inspeksi, ekspor.</span>
      <a class="btn" href="<?= htmlspecialchars($pmaUrl) ?>">Buka phpMyAdmin ↗</a>
    </div>
  </section>

  <section class="powered">
    <div class="camel">🐫</div>
    <strong>Powered by Sabdopalon</strong>
    <p>Lingkungan development PHP lokal dalam satu folder — PHP, MariaDB,
       phpMyAdmin, dan tools lain berjalan portabel tanpa instalasi sistem.</p>
    <div class="links">
      <a href="https://github.com/danyakmallun9999/sabdopalon" target="_blank" rel="noopener">★ GitHub</a>
      <a href="https://github.com/danyakmallun9999/sabdopalon/releases" target="_blank" rel="noopener">⬇ Unduh</a>
    </div>
  </section>

  <footer>Dibuat dengan ❤ oleh <a href="https://danyakmallun.dev" target="_blank" rel="noopener">danyakmallun</a></footer>
</body>
</html>
