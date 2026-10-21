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
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/hostsync"
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
	// note is a successful creation with a caveat — typically a hosts entry
	// that needs administrator rights. It is not an error: the files exist.
	note     string
	log      createLog
	started  string
	finished string
}

// createJob is process-wide on purpose: a dashboard serves one installation, and
// two sites cannot be scaffolded into the same folder at once anyway.
var createJob createTask

// snapshot is the JSON the dialog polls. Output is the whole log rather than a
// delta: it is bounded (see createLog), and re-reading it keeps the page dumb.
//
// Two forms travel: "output" is what a person reads, and "output_raw" is the
// terminal's own bytes for when something needs diagnosing.
func (j *createTask) snapshot() map[string]any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return map[string]any{
		"running":    j.running,
		"done":       j.done,
		"name":       j.name,
		"template":   j.template,
		"url":        j.url,
		"error":      j.err,
		"note":       j.note,
		"output":     j.log.Simplified(),
		"output_raw": j.log.Raw(),
		"started":    j.started,
		"finished":   j.finished,
	}
}

// createJobWriter feeds a template's progress into the job under its mutex, so a
// poll that lands mid-write never sees a torn line.
type createJobWriter struct{ j *createTask }

func (w createJobWriter) Write(p []byte) (int, error) {
	w.j.mu.Lock()
	defer w.j.mu.Unlock()
	return w.j.log.Write(p)
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
	createJob.note = ""
	createJob.url = ""
	createJob.log.reset()
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

		// The site exists on disk now, so its hostname has to exist too. With a
		// custom TLD that means a hosts entry; without this a freshly created
		// site was reachable only at .localhost.
		//
		// A failure here is not a creation failure — the files are all there —
		// but it used to be discarded entirely, so the site was "created" and
		// simply unreachable at the address the dialog had just printed. The
		// note travels in the snapshot so the dialog can say which step to
		// take; the error itself must not be swallowed.
		var hostNote string
		if err == nil {
			if _, herr := s.SyncHosts(); herr != nil {
				if errors.Is(herr, hostsync.ErrPermission) {
					hostNote = fmt.Sprintf(
						"The site was created, but its hostname is not in the hosts file yet: "+
							"writing it needs administrator rights. Click Synchronize (Administrator) on the Settings page, "+
							"or run `sabdopalon hosts:sync` in an elevated terminal. (%s)",
						hostsync.Path())
				} else {
					hostNote = "The site was created, but its hosts entry could not be updated: " + herr.Error()
				}
			}
		}
		createJob.mu.Lock()
		defer createJob.mu.Unlock()
		createJob.running = false
		createJob.done = true
		createJob.note = hostNote
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
