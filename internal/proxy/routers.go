// Package proxy — routers.go: framework-specific PHP router scripts.
//
// The PHP built-in server uses a router script to implement pretty URLs /
// front-controller patterns. Sabdopalon writes one .sabdopalon-router.php
// into each site folder; this file picks the right content based on the
// detected framework.
package proxy

// pickRouter returns the PHP router script content for the given framework.
func pickRouter(f Framework) string {
	switch f {
	case FrameworkLaravel:
		return laravelRouter
	case FrameworkWordPress:
		return defaultRouter // WordPress ships its own index.php front controller
	default:
		return defaultRouter
	}
}

// laravelRouter forwards all non-static requests to public/index.php with the
// correct PATH_INFO and SCRIPT_NAME so Laravel's Request::capture() and
// Route matching work under the PHP built-in server.
const laravelRouter = `<?php
// Sabdopalon Laravel router — forwards all non-static requests to
// public/index.php (the Laravel front controller) with the correct
// PATH_INFO and SCRIPT_NAME so routing and asset loading work under
// php -S.
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
