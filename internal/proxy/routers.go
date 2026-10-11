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

// sharedRouter returns the one router script used by every site, stamped with the
// current router version so an outdated file can be recognised and replaced.
//
// One file, not one per framework: what to forward to is decided at request time
// from $_SERVER['DOCUMENT_ROOT'], which keeps the script in one place and removes
// the per-site version stamping the previous design needed.
func sharedRouter() string {
	return strings.Replace(sharedRouterBody, "<?php", "<?php\n"+routerMarker+" "+strconv.Itoa(routerVersion), 1)
}

// pickRouter is kept for callers that still ask per framework; every framework
// now gets the same script.
func pickRouter(f Framework) string {
	return sharedRouter()
}

// sharedRouterBody forwards a request that is not a file to the application's
// front controller.
//
// Laravel is detected by its artisan file one level above the docroot (Sabdopalon
// serves Laravel from public/), which is also what tells us to set
// SCRIPT_NAME/SCRIPT_FILENAME/PATH_INFO — Laravel's Request::createFromGlobals()
// and Route matching depend on them under php -S. WordPress, CodeIgniter and
// Symfony keep index.php in the docroot itself and read REQUEST_URI, so they only
// need the forwarding.
//
// Every path comes from $_SERVER['DOCUMENT_ROOT'] and never from __DIR__: the
// script lives in the data root, so anything relative to its own location would
// point at the wrong place.
const sharedRouterBody = `<?php
// Sabdopalon router — one script for every site.
// Non-file requests go to the application's front controller.
` + routerProxyPrelude + `
$uri = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);
$docroot = $_SERVER['DOCUMENT_ROOT'];

// Static files are served by php -S itself, without booting the application.
if ($uri !== '/' && is_file($docroot . $uri)) {
    return false;
}

$laravel = is_file($docroot . '/../artisan');
$index   = $docroot . '/index.php';

if ($laravel && is_file($index)) {
    // Laravel: SCRIPT_NAME/SCRIPT_FILENAME/PATH_INFO make Request::capture()
    // compute the right pathinfo and base URL.
    $_SERVER['SCRIPT_NAME']     = '/index.php';
    $_SERVER['SCRIPT_FILENAME'] = $index;
    $_SERVER['PATH_INFO']       = $uri;
    require $index;
    return true;
}

if (is_file($index)) {
    // WordPress, CodeIgniter, Symfony and plain apps that route on their own.
    require $index;
    return true;
}

http_response_code(404);
echo "404 Not Found — no index.php in " . $docroot;
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
