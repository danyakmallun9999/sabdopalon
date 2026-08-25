package devtools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAvailableTools(t *testing.T) {
	dir := t.TempDir()

	// Empty dir: no tools applicable.
	tools := AvailableTools(dir)
	if len(tools) != 0 {
		t.Errorf("empty dir: expected 0 tools, got %d", len(tools))
	}

	// Add vite.config.js → vite applicable.
	_ = os.WriteFile(filepath.Join(dir, "vite.config.js"), []byte("{}"), 0o644)
	tools = AvailableTools(dir)
	if !contains(tools, "vite") {
		t.Errorf("with vite.config.js: expected vite tool, got %v", toolNames(tools))
	}

	// Add artisan → artisan-serve applicable.
	_ = os.WriteFile(filepath.Join(dir, "artisan"), []byte("#!/bin/php"), 0o644)
	tools = AvailableTools(dir)
	if !contains(tools, "artisan-serve") {
		t.Errorf("with artisan: expected artisan-serve tool, got %v", toolNames(tools))
	}

	// Add package.json → npm-build applicable (and npm-dev if no vite config).
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)
	tools = AvailableTools(dir)
	if !contains(tools, "npm-build") {
		t.Errorf("with package.json: expected npm-build tool, got %v", toolNames(tools))
	}

	// Add composer.json → composer-install applicable.
	_ = os.WriteFile(filepath.Join(dir, "composer.json"), []byte("{}"), 0o644)
	tools = AvailableTools(dir)
	if !contains(tools, "composer-install") {
		t.Errorf("with composer.json: expected composer-install tool, got %v", toolNames(tools))
	}
}

func TestManagerStartStopUnknownTool(t *testing.T) {
	m := New()
	_, err := m.Start("site", t.TempDir(), "nonexistent-tool")
	if err == nil {
		t.Error("expected error for unknown tool")
	}
}

func TestManagerStopNonRunning(t *testing.T) {
	m := New()
	if err := m.Stop("site", "vite"); err != nil {
		t.Errorf("stopping a non-running tool should be a no-op, got: %v", err)
	}
}

func TestManagerStopAllForSite(t *testing.T) {
	m := New()
	m.StopAllForSite("site") // no-op, should not panic
}

func TestVitePortNotRunning(t *testing.T) {
	m := New()
	if port := m.VitePort("site"); port != 0 {
		t.Errorf("expected 0 for non-running Vite, got %d", port)
	}
}

func TestStatusEmpty(t *testing.T) {
	m := New()
	status := m.Status("site")
	// Status returns all registered tools, all not running.
	if len(status) == 0 {
		t.Error("expected at least one tool in status")
	}
	for _, st := range status {
		if st.Running {
			t.Errorf("tool %s should not be running", st.Tool)
		}
	}
}

func contains(tools []Status, name string) bool {
	for _, t := range tools {
		if t.Tool == name {
			return true
		}
	}
	return false
}

func toolNames(tools []Status) []string {
	out := make([]string, 0, len(tools))
	for _, t := range tools {
		out = append(out, t.Tool)
	}
	return out
}
