// Package dashboard — handlers_create.go: creating a site as a job you can watch.
//
// Creating a Laravel project runs composer — and for a starter kit, npm as well —
// for minutes. Doing that inside the POST meant one HTTP request stayed open for
// the whole build: the dialog said "Creating…" and nothing else, so progress was
// indistinguishable from a hang, and a browser or proxy that gave up made a
// perfectly healthy build look like an error.
//
// So creation is a job, the same shape as a package install: POST starts it and
// returns immediately, GET reports its output while it runs, and the dashboard
// shows the template's own words — composer resolving packages included.
package dashboard

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/templates"
)

// createTask is the one site creation that can be in flight.
type createTask struct {
	mu       sync.Mutex
	running  bool
	done     bool
	name     string
	template string
	url      string
	err      string
	output   bytes.Buffer
	started  string
	finished string
}

// createJob is process-wide on purpose: a dashboard serves one installation, and
// two sites cannot be scaffolded into the same folder at once anyway.
var createJob createTask

// snapshot is the JSON the dialog polls. Output is the whole log rather than a
// delta: it is bounded by what a scaffold prints, and re-reading it keeps the
// page dumb.
func (j *createTask) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return map[string]any{
		"running":  j.running,
		"done":     j.done,
		"name":     j.name,
		"template": j.template,
		"url":      j.url,
		"error":    j.err,
		"output":   j.output.String(),
		"started":  j.started,
		"finished": j.finished,
	}
}

// createJobWriter feeds a template's progress into the job under its mutex, so a
// poll that lands mid-write never sees a torn line.
type createJobWriter struct{ j *createTask }

func (w createJobWriter) Write(p []byte) (int, error) {
	w.j.mu.Lock()
	defer w.j.mu.Unlock()
	return w.j.output.Write(p)
}

// startSiteCreate begins the job and returns an empty string, or a sentence
// explaining why it could not start — so the caller can answer with an error
// rather than hand the dialog a job that is already dead.
func (s *Server) startSiteCreate(name, tmpl, database, testing string) string {
	createJob.mu.Lock()
	if createJob.running {
		current := createJob.name
		createJob.mu.Unlock()
		return "another site is still being created (" + current + ") — wait for it to finish"
	}
	createJob.running = true
	createJob.done = false
	createJob.name = name
	createJob.template = tmpl
	createJob.err = ""
	createJob.url = ""
	createJob.output.Reset()
	createJob.started = time.Now().UTC().Format(time.RFC3339)
	createJob.finished = ""
	createJob.mu.Unlock()

	cfg := s.cfg
	go func() {
		err := templates.CreateWith(templates.Options{
			SitesDir: cfg.Root,
			Template: tmpl,
			Name:     name,
			Database: database,
			Testing:  testing,
			Cfg:      cfg,
			Progress: templates.NewProgress(createJobWriter{&createJob}),
		})

		createJob.mu.Lock()
		defer createJob.mu.Unlock()
		createJob.running = false
		createJob.done = true
		createJob.finished = time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			createJob.err = err.Error()
			return
		}
		url, _ := s.siteURLs(name)
		createJob.url = url
	}()
	return ""
}

// handleAPICreateJob reports the creation in flight, or the last one.
//
//	GET /api/sites/job → {running, done, name, template, url, error, output, …}
//
// The path is why a site may not be called "job": the dashboard needs one URL
// that cannot collide with a site's own detail endpoint.
func (s *Server) handleAPICreateJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.methodNotAllowed(w, "GET")
		return
	}
	s.json(w, createJob.snapshot())
}
