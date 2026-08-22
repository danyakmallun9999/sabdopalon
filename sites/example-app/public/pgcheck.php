<?php header('Content-Type: text/plain');
try {
  $pdo = new PDO('pgsql:host=127.0.0.1;port=5432;dbname=postgres', 'sabdopalon', '');
  echo "PG CONNECTED: " . $pdo->query('select version()')->fetchColumn() . "\n";
} catch (Exception $e) { echo "FAIL: ".$e->getMessage()."\n"; }
echo "SABDOPALON_PG=" . getenv('SABDOPALON_PG_HOST') . "\n";
