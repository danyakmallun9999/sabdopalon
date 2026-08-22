<?php header('Content-Type: text/plain');
// MinIO S3 round-trip check (bucket + object) using SABDOPALON_S3_* env.
$endpoint = rtrim(getenv('SABDOPALON_S3_ENDPOINT') ?: 'http://127.0.0.1:9000', '/');
$key      = getenv('SABDOPALON_S3_KEY')      ?: 'sabdopalon';
$secret   = getenv('SABDOPALON_S3_SECRET')   ?: 'sabdopalon-secret';
$bucket   = getenv('SABDOPALON_S3_BUCKET')   ?: 'sabdopalon-bucket';

// Very small AWS SigV4 implementation (only what the probe needs).
function sigv4($method, $host, $path, $key, $secret, $now, $payload) {
  $amz = gmdate('Ymd\THis\Z', $now);
  $datestamp = gmdate('Ymd', $now);
  $scope = $datestamp . '/us-east-1/s3/aws4_request';
  $canonicalHeaders = "host:$host\nx-amz-content-sha256:" . hash('sha256', $payload) . "\nx-amz-date:$amz\n";
  $signedHeaders = 'host;x-amz-content-sha256;x-amz-date';
  $canonicalRequest = "$method\n$path\n\n$canonicalHeaders\n$signedHeaders\n" . hash('sha256', $payload);
  $stringToSign = "AWS4-HMAC-SHA256\n$amz\n$scope\n" . hash('sha256', $canonicalRequest);
  $kDate = hash_hmac('sha256', $datestamp, 'AWS4' . $secret, true);
  $kRegion = hash_hmac('sha256', 'us-east-1', $kDate, true);
  $kService = hash_hmac('sha256', 's3', $kRegion, true);
  $kSigning = hash_hmac('sha256', 'aws4_request', $kService, true);
  $signature = hash_hmac('sha256', $stringToSign, $kSigning);
  return "AWS4-HMAC-SHA256 Credential=$key/$scope, SignedHeaders=$signedHeaders, Signature=$signature";
}

$host = parse_url($endpoint, PHP_URL_HOST) . ':' . (parse_url($endpoint, PHP_URL_PORT) ?: 80);
$now = time();

function s3req($method, $endpoint, $host, $path, $key, $secret, $now, $payload = '') {
  $url = $endpoint . $path;
  $headers = [
    'Host: ' . $host,
    'x-amz-content-sha256: ' . hash('sha256', $payload),
    'x-amz-date: ' . gmdate('Ymd\THis\Z', $now),
    'Authorization: ' . sigv4($method, $host, $path, $key, $secret, $now, $payload),
  ];
  $ctx = stream_context_create(['http' => [
    'method' => $method,
    'header' => implode("\r\n", $headers) . "\r\n",
    'content' => $payload,
    'ignore_errors' => true,
    'timeout' => 5,
  ]]);
  return file_get_contents($url, false, $ctx);
}

try {
  // Ensure bucket exists (PutBucket).
  $r = s3req('PUT', $endpoint, $host, '/' . $bucket, $key, $secret, $now);
  $code = isset($http_response_header[0]) ? (int)substr($http_response_header[0], 9, 3) : 0;
  if ($code !== 200 && $code !== 409) { echo "FAIL bucket create: HTTP $code $r\n"; exit; }

  // PutObject then GetObject.
  $body = "sabdopalon-probe-" . $now;
  $r = s3req('PUT', $endpoint, $host, '/' . $bucket . '/probe.txt', $key, $secret, $now, $body);
  $code = (int)substr($http_response_header[0], 9, 3);
  if ($code !== 200) { echo "FAIL put: HTTP $code $r\n"; exit; }

  $got = s3req('GET', $endpoint, $host, '/' . $bucket . '/probe.txt', $key, $secret, $now);
  echo ($got === $body)
    ? "S3 OK: round-trip '$got' via $endpoint\n"
    : "FAIL get: expected '$body' got '$got'\n";
} catch (Exception $e) { echo "FAIL: " . $e->getMessage() . "\n"; }
echo "SABDOPALON_S3_ENDPOINT=" . getenv('SABDOPALON_S3_ENDPOINT') . "\n";
