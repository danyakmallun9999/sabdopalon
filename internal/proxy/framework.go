// Package proxy — framework.go: detects the PHP framework of a site so the
// proxy can choose the right router script (Laravel front controller, generic,
// etc.) instead of the one-size-fits-all default.
package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Framework is the detected PHP framework for a site directory.
type Framework string

const (
	FrameworkUnknown     Framework = ""
	FrameworkLaravel     Framework = "laravel"
	FrameworkWordPress   Framework = "wordpress"
	FrameworkCodeIgniter Framework = "codeigniter"
	FrameworkSymfony     Framework = "symfony"
)

// String returns a human label for the framework.
func (f Framework) String() string {
	switch f {
	case FrameworkLaravel:
		return "Laravel"
	case FrameworkWordPress:
		return "WordPress"
	case FrameworkCodeIgniter:
		return "CodeIgniter"
	case FrameworkSymfony:
		return "Symfony"
	default:
		return "unknown"
	}
}

// DetectFramework inspects a site directory and returns the framework name.
// It looks for framework signature files and composer.json dependencies.
func DetectFramework(siteDir string) Framework {
	// Laravel: artisan + composer.json with "laravel/framework"
	if fileExists(filepath.Join(siteDir, "artisan")) {
		if hasComposerReq(siteDir, "laravel/framework") {
			return FrameworkLaravel
		}
	}
	// WordPress: wp-config.php or wp-config-sample.php
	if fileExists(filepath.Join(siteDir, "wp-config.php")) ||
		fileExists(filepath.Join(siteDir, "wp-config-sample.php")) {
		return FrameworkWordPress
	}
	// CodeIgniter 4: composer.json with "codeigniter4"
	if hasComposerReq(siteDir, "codeigniter4/codeigniter4") {
		return FrameworkCodeIgniter
	}
	// Symfony: symfony.lock or composer.json with "symfony/framework-bundle"
	if fileExists(filepath.Join(siteDir, "symfony.lock")) {
		return FrameworkSymfony
	}
	if hasComposerReq(siteDir, "symfony/framework-bundle") {
		return FrameworkSymfony
	}
	return FrameworkUnknown
}

// composerReq represents the relevant fields of a composer.json file.
type composerReq struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

// hasComposerReq checks whether a specific package is listed in the
// require or require-dev sections of composer.json.
func hasComposerReq(siteDir, pkg string) bool {
	data, err := os.ReadFile(filepath.Join(siteDir, "composer.json"))
	if err != nil {
		return false
	}
	var cr composerReq
	if err := json.Unmarshal(data, &cr); err != nil {
		return false
	}
	if _, ok := cr.Require[pkg]; ok {
		return true
	}
	if _, ok := cr.RequireDev[pkg]; ok {
		return true
	}
	return false
}

// keep strings imported (used in String labels)
var _ = strings.TrimSpace
