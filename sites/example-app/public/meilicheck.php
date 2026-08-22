<?php header('Content-Type: text/plain');
// Meilisearch index + search round-trip using SABDOPALON_MEILI_HOST env.
$host = rtrim(getenv('SABDOPALON_MEILI_HOST') ?: 'http://127.0.0.1:7700', '/');

function meili($method, $host, $path, $payload = null) {
  $opts = [
    'http' => [
      'method' => $method,
      'header' => "Content-Type: application/json\r\n",
      'content' => $payload,
      'ignore_errors' => true,
      'timeout' => 5,
    ],
  ];
  return file_get_contents($host . $path, false, stream_context_create($opts));
}

try {
  $doc = json_encode([['id' => 1, 'title' => 'sabdopalon meilisearch probe']]);
  $r = meili('POST', $host, '/indexes/probe/documents', $doc);
  $code = isset($http_response_header[0]) ? (int)substr($http_response_header[0], 9, 3) : 0;
  if ($code !== 202) { echo "FAIL add documents: HTTP $code $r\n"; exit; }

  // Wait for indexing (task may still be processing).
  for ($i = 0; $i < 10; $i++) {
    $search = meili('POST', $host, '/indexes/probe/search', '{"q":"probe","limit":5}');
    if (str_contains($search, 'sabdopalon meilisearch probe')) { break; }
    usleep(300000);
  }
  echo str_contains($search, 'sabdopalon meilisearch probe')
    ? "MEILI OK: index+search round-trip via $host\n"
    : "FAIL search: $search\n";
} catch (Exception $e) { echo "FAIL: " . $e->getMessage() . "\n"; }
echo "SABDOPALON_MEILI_HOST=" . getenv('SABDOPALON_MEILI_HOST') . "\n";
