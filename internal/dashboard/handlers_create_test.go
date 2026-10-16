package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
	"github.com/sabdopalon/sabdopalon/internal/proxy"
)

// createServer builds a dashboard over an empty sites root, so a create job has
// somewhere to work. The job is process-wide, so every test leaves it clean.
func createServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	cfg := &config.Engine{RootDir: root, TLD: "localhost"}
	cfg.Root = filepath.Join(root, "sites")
	cfg.Proxy.HTTPPort = 8080
	if err := os.MkdirAll(cfg.Root, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		createJob.mu.Lock()
		defer createJob.mu.Unlock()
		createJob.running, createJob.done = false, false
		createJob.name, createJob.template, createJob.url, createJob.err = "", "", "", ""
		createJob.log.reset()
	})
	return New(cfg, proxy.New(cfg), nil, nil, nil, nil)
}

// waitForCreateJob polls the job endpoint until it reports done, the way the
// dialog does.
func waitForCreateJob(t *testing.T, s *Server) map[string]any {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/sites/job", nil))
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("job body is not JSON: %s", rec.Body.String())
		}
		if done, _ := got["done"].(bool); done {
			return got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the create job never finished")
	return nil
}

// TestCreateSiteRunsAsAJob: POST returns immediately with a job, and the log that
// the dialog shows carries the template's own progress — which is what turns
// "Creating…" into something that cannot be mistaken for a hang.
func TestCreateSiteRunsAsAJob(t *testing.T) {
	s := createServer(t)

	code, resp := doJSON(t, s, http.MethodPost, "/api/sites", `{"name":"Demo","template":"blank"}`)
	if code != http.StatusOK {
		t.Fatalf("POST /api/sites = %d (%v)", code, resp)
	}
	if msg, bad := resp["error"]; bad {
		t.Fatalf("POST reported an error: %v", msg)
	}
	if _, ok := resp["job"]; !ok {
		t.Errorf("the response should carry the job: %v", resp)
	}
	if resp["name"] != "demo" {
		t.Errorf("name = %v, want the normalized \"demo\"", resp["name"])
	}

	job := waitForCreateJob(t, s)
	if job["error"] != "" {
		t.Fatalf("the job failed: %v\n%s", job["error"], job["output"])
	}
	log, _ := job["output"].(string)
	for _, want := range []string{"Creating blank project 'demo'", "Project created at sites/demo/"} {
		if !strings.Contains(log, want) {
			t.Errorf("job log is missing %q:\n%s", want, log)
		}
	}
	if url, _ := job["url"].(string); !strings.Contains(url, "demo.localhost") {
		t.Errorf("url = %q, want the site's own address", url)
	}
	p, err := os.ReadFile(filepath.Join(s.cfg.Root, "demo", "public", "index.php"))
	if err != nil || !strings.Contains(string(p), "<?php") {
		t.Errorf("the scaffolded index.php is missing: %v", err)
	}
}

// TestCreateSiteRefusesASecondJob: two scaffolds into the same tree at once is
// how a half-built project happens, so the second request is refused with a
// sentence naming what to wait for.
func TestCreateSiteRefusesASecondJob(t *testing.T) {
	s := createServer(t)

	createJob.mu.Lock()
	createJob.running, createJob.name = true, "first"
	createJob.mu.Unlock()

	code, resp := doJSON(t, s, http.MethodPost, "/api/sites", `{"name":"second","template":"blank"}`)
	if code != http.StatusConflict {
		t.Fatalf("second POST = %d, want 409 (%v)", code, resp)
	}
	msg, _ := resp["error"].(string)
	if !strings.Contains(msg, "first") {
		t.Errorf("error = %q, want it to name the running job", msg)
	}
}

// TestCreateSiteRefusesTheReservedName: GET /api/sites/job is the job's own URL,
// so a site called "job" would shadow its detail endpoint.
func TestCreateSiteRefusesTheReservedName(t *testing.T) {
	s := createServer(t)
	code, resp := doJSON(t, s, http.MethodPost, "/api/sites", `{"name":"job","template":"blank"}`)
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	msg, _ := resp["error"].(string)
	if !strings.Contains(msg, "reserved") {
		t.Errorf("error = %q, want it to explain the reserved name", msg)
	}
}

// TestCreateJobReportsNothingWhenIdle: the dialog asks on every open, and an
// untouched dashboard must answer with a shape it can render, not an error.
func TestCreateJobReportsNothingWhenIdle(t *testing.T) {
	s := createServer(t)
	code, resp := doJSON(t, s, http.MethodGet, "/api/sites/job", "")
	if code != http.StatusOK {
		t.Fatalf("GET /api/sites/job = %d", code)
	}
	for _, key := range []string{"running", "done", "name", "output", "url", "error"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("idle snapshot is missing %q: %v", key, resp)
		}
	}
	if running, _ := resp["running"].(bool); running {
		t.Error("a fresh dashboard must not report a running job")
	}
}
