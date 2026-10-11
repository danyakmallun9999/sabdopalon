package proxy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRouterFileIsSharedAndOutsideSites: one router for the whole installation,
// and never inside a site folder — for a linked site that folder is the user's
// own repository.
func TestRouterFileIsSharedAndOutsideSites(t *testing.T) {
	root := filepath.Join("C:", "sabdopalon")
	got := routerFile(root)
	want := filepath.Join(root, "routes", "router.php")
	if got != want {
		t.Errorf("routerFile(%q) = %q, want %q", root, got, want)
	}
	// The same answer for every site: no site name may leak into the path.
	if got != routerFile(root) {
		t.Error("routerFile must not depend on the site")
	}
	if strings.Contains(got, routerFileName) {
		t.Errorf("the router must not be named %s any more: %s", routerFileName, got)
	}
}

// TestNeedsRouter: plain PHP needs no front controller, because php -S finds
// index.php in the docroot by itself. That was verified by serving a hand-written
// CRUD app with the router deleted. Frameworks whose URLs do not map to files do.
func TestNeedsRouter(t *testing.T) {
	if needsRouter(FrameworkUnknown) {
		t.Error("plain PHP must not get a router")
	}
	for _, f := range []Framework{FrameworkLaravel, FrameworkWordPress, FrameworkCodeIgniter, FrameworkSymfony} {
		if !needsRouter(f) {
			t.Errorf("%s needs a router: its URLs do not map to files", f)
		}
	}
}

// TestSharedRouterPathsComeFromDocumentRoot is the regression guard for the one
// mistake this design invites: the script lives in the data root, so a path
// relative to its own location (__DIR__) would point at the wrong place and break
// every framework at once.
func TestSharedRouterPathsComeFromDocumentRoot(t *testing.T) {
	body := sharedRouter()

	if !strings.Contains(body, "$_SERVER['DOCUMENT_ROOT']") {
		t.Error("the router must resolve paths from $_SERVER['DOCUMENT_ROOT']")
	}
	if strings.Contains(body, "__DIR__") || strings.Contains(body, "__FILE__") {
		t.Error("the router lives in the data root: paths relative to its own location point at the wrong place")
	}
	if !strings.Contains(body, routerMarker) {
		t.Error("the generated router must carry the version marker")
	}
	if got := routerVersionOf(body); got != routerVersion {
		t.Errorf("router version = %d, want %d", got, routerVersion)
	}
	// Laravel needs these three, and they must exist for it to route under php -S.
	for _, want := range []string{"SCRIPT_NAME", "SCRIPT_FILENAME", "PATH_INFO"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s for Laravel's front controller", want)
		}
	}
	// The front controller is detected at request time, which is what allows one
	// file to serve every site.
	if !strings.Contains(body, "artisan") {
		t.Error("the router must detect Laravel at request time")
	}
}

// TestDumpRouterForLiveCheck materialises the generated router so it can be run
// against a real site with `php -S`, which is the only way to check that the
// request-time framework detection behaves. Skipped unless SABDOPALON_DUMP_ROUTER
// names the file to write, so an ordinary test run leaves nothing behind.
func TestDumpRouterForLiveCheck(t *testing.T) {
	path := os.Getenv("SABDOPALON_DUMP_ROUTER")
	if path == "" {
		t.Skip("set SABDOPALON_DUMP_ROUTER=<path> to write the router for a live check")
	}
	body := sharedRouter()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("router written to %s (%d bytes, version %d)", path, len(body), routerVersionOf(body))
}

// folders, and never touch a file that is not ours.
func TestRemoveStaleSiteRouter(t *testing.T) {
	dir := t.TempDir()

	ours := filepath.Join(dir, routerFileName)
	if err := os.WriteFile(ours, []byte(sharedRouter()), 0o644); err != nil {
		t.Fatal(err)
	}
	removeStaleSiteRouter(dir)
	if _, err := os.Stat(ours); !os.IsNotExist(err) {
		t.Error("a router this program wrote must be removed from the site folder")
	}

	theirs := filepath.Join(dir, routerFileName)
	if err := os.WriteFile(theirs, []byte("<?php // my own router, thank you\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeStaleSiteRouter(dir)
	if _, err := os.Stat(theirs); err != nil {
		t.Errorf("someone else's router must be left alone: %v", err)
	}
}
