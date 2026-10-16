# Creating a site — design note

Status: implemented in 0.11.24. Before that, creation ran inside the POST request.

## What was wrong

`POST /api/sites` built the project synchronously. For a blank template that is
milliseconds, so nobody noticed; for `laravel` it runs `composer create-project`
and for a starter kit `npm install` as well, which is minutes. The dialog was left
with a disabled button reading "Creating…", and three things followed from it:

1. **Progress was invisible.** Composer prints the package it is on, and all of it
   went to the sidecar's own stdout (`logs/sidecar.log`), where no one looks.
2. **A hang and a healthy build looked identical.** The only signal was the absence
   of a response.
3. **The request was the build's lifetime.** A proxy or browser that timed out
   turned a working scaffold into an error the user could not distinguish from a
   real failure.

## The shape now

Creation is a job, the same shape as a package install or an update download:

```
POST /api/sites        {name, template, database, testing}  → 200 {ok, name, job}
GET  /api/sites/job                                          → {running, done, name,
                                                                template, url, error,
                                                                output, started, finished}
```

- `POST` validates, starts the job and returns immediately. A second create while
  one runs is refused with `409` and a sentence naming the job to wait for.
- The job's `output` is the whole log, not a delta: bounded by what a scaffold
  prints, and it keeps the page dumb. The dialog polls it every 500 ms.
- `GET /api/sites/job` is a literal path, so a site may not be named `job` — it
  would shadow its own detail endpoint. The create validator refuses that name with
  an explanation rather than leaving a trap.
- Closing the dialog does not cancel anything: it says "Run in background", the
  polling continues in the page, and the completion toast and the site list follow
  the job to the end. Reopening the dialog re-attaches to a running job, which also
  covers a page reload mid-build.
- A **finished** job is history, not state: opening the dialog clears it (a running
  one is kept), so "New Site" always lands on the form. The success state carries a
  "Create another" button for the same reason — 0.11.24 shipped without this and the
  dialog reopened on the previous result with nothing but "Close", so a second site
  needed a page reload.

## Where the log comes from

`internal/templates` reports through one type, `Progress`:

```go
p.Printf("  ⏳  Running composer create-project laravel/laravel...")
err := p.Run(sitesDir, "composer", "create-project", "laravel/laravel", name, "--prefer-dist")
```

`Progress.Run` resolves the tool with `bin/` first, gives the child the same
environment (`childEnv`), streams its stdout and stderr to the writer as they
arrive, and keeps the last 8 KB for the error message. A nil `*Progress` discards
everything, which is how a test and a CLI run that does not care say so.

The CLI (`sabdopalon new`) passes a Progress writing to stdout, so its output is
unchanged apart from no longer being buffered: the tool's own lines now appear while
it works, the same as in the dashboard.
