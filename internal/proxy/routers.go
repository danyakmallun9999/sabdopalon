// Package proxy — routers.go: framework-specific PHP router scripts.
//
// The PHP built-in server uses a router script to implement pretty URLs /
// front-controller patterns. Sabdopalon writes one .sabdopalon-router.php
// into each site folder; this file picks the right content based on the
// detected framework.
package proxy

import (
	"os"
	"strconv"
	"strings"
)

// routerVersion is bumped whenever the generated router changes in a way that
// EXISTING sites must also receive. Without it a fix could only ever reach sites
// created after the upgrade: ensureRouters deliberately does not overwrite a
// router that is already there, so every existing site would keep its old file
// forever.
const routerVersion = 2

// routerMarker prefixes the version line of every generated router.
const routerMarker = "// sabdopalon-router-version:"

// routerProxyPrelude makes the request look like the one the visitor actually
// made.
//
// A shared site is reached over HTTPS (Cloudflare terminates TLS), but the
// request reaches PHP over plain HTTP. Every framework then generates http://
// URLs for an https:// page; the browser blocks them as mixed content, and an
// app that ships its HTML and then loads its JavaScript renders an empty page.
// That is precisely how "share publicly" came out blank for a Laravel site
// while the same site worked locally.
//
// Laravel/Symfony decide the scheme from $_SERVER['HTTPS'], which the PHP
// built-in server never sets for a plain request, so PHP is the only place this
// can be corrected. It is guarded by the forwarded header, so local
// http://<site>.localhost traffic is untouched.
const routerProxyPrelude = `
// Honour the scheme the visitor actually used (set by Sabdopalon's proxy for
// requests arriving through a public tunnel hostname).
if (($_SERVER['HTTP_X_FORWARDED_PROTO'] ?? '') === 'https') {
    $_SERVER['HTTPS'] = 'on';
    $_SERVER['REQUEST_SCHEME'] = 'https';
    $_SERVER['SERVER_PORT'] = '443';
    if (!empty($_SERVER['HTTP_X_FORWARDED_HOST'])) {
        $_SERVER['HTTP_HOST'] = $_SERVER['HTTP_X_FORWARDED_HOST'];
    }
}
`

// pickRouter returns the PHP router script for the given framework, stamped with
// the current router version so an outdated file can be recognised and replaced.
func pickRouter(f Framework) string {
	body := defaultRouter
	if f == FrameworkLaravel {
		body = laravelRouter
	}
	return strings.Replace(body, "<?php", "<?php\n"+routerMarker+" "+strconv.Itoa(routerVersion), 1)
}

// laravelRouter forwards all non-static requests to public/index.php with the
// correct PATH_INFO and SCRIPT_NAME so Laravel's Request::capture() and
// Route matching work under the PHP built-in server.
const laravelRouter = `<?php
// Sabdopalon Laravel router — forwards all non-static requests to
// public/index.php (the Laravel front controller) with the correct
// PATH_INFO and SCRIPT_NAME so routing and asset loading work under
// php -S.
` + routerProxyPrelude + `
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$docroot = $_SERVER['DOCUMENT_ROOT'];

// Serve static files directly from public/ (css, js, images, favicon).
// This lets the built-in server stream assets without booting Laravel.
if ($uri !== '/') {
    $file = $docroot . $uri;
    if (is_file($file)) {
        return false; // let PHP's built-in server serve the file
    }
}

// Everything else → Laravel front controller.
// Setting SCRIPT_NAME/SCRIPT_FILENAME lets Laravel's Request::createFromGlobals()
// compute the correct pathinfo and base URL under php -S.
$_SERVER['SCRIPT_NAME']     = '/index.php';
$_SERVER['SCRIPT_FILENAME'] = $docroot . '/index.php';
$_SERVER['PATH_INFO']       = $uri;

require $docroot . '/index.php';
return true;
`

const defaultRouter = `<?php
// Sabdopalon default PHP router — serves static files, falls back to index.php.
` + routerProxyPrelude + `
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$docroot = $_SERVER['DOCUMENT_ROOT'];
$file = $docroot . $uri;
if ($uri !== '/' && is_file($file)) {
    return false; // let PHP's built-in server serve the static file
}
$index = $docroot . '/index.php';
if (is_file($index)) {
    require $index;
    return true;
}
http_response_code(404);
echo "404 Not Found — create index.php or the requested file in " . $docroot;
return true;
`

// routerNeedsRewrite reports whether the router file at path should be written.
//
// A file we never wrote (no Sabdopalon signature) is left alone: a site may
// legitimately ship its own router under this name. A file we wrote that carries
// no version, or an older one, is ours to update.
func routerNeedsRewrite(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true // missing
	}
	content := string(data)
	if !strings.Contains(content, "Sabdopalon") {
		return false // someone else's router
	}
	return routerVersionOf(content) < routerVersion
}

// routerVersionOf reads the version marker, or 0 when the file predates it.
func routerVersionOf(content string) int {
	for _, line := range strings.Split(content, "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), routerMarker)
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}
