// Package dashboard — handlers_update.go: is there a newer release?
//
// Checking is deliberately separate from applying. A local development tool holds
// the user's database and running servers, so it may tell the user that a release
// exists, and must never install one on its own. Nothing here downloads or runs
// anything.
package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// releasesAPI is a variable so tests can point it at their own server instead of
// the real GitHub.
var releasesAPI = "https://api.github.com/repos/danyakmallun9999/sabdopalon/releases/latest"

// updateCache holds the last check. One process serves one dashboard, so a package
// level value is equivalent to a server field without widening the Server struct.
// It is memory only: a restart simply means the next check happens sooner.
var updateCache struct {
	mu  sync.Mutex
	got updatePayload
}

// updatePayload is what the About page renders. Kind decides which update path
// applies: a release that carries a bare sidecar asset can be applied in place,
// while one that also changed the desktop shell needs the installer.
type updatePayload struct {
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	Available   bool   `json:"available"`
	Kind        string `json:"kind,omitempty"` // sidecar | installer | none
	Notes       string `json:"notes,omitempty"`
	URL         string `json:"url,omitempty"`
	Size        int64  `json:"size,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	CheckedAt   string `json:"checked_at,omitempty"`
	Error       string `json:"error,omitempty"`
}

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

// parseVersion reads a "1.2.3" or "v1.2.3" version. Anything else — "dev", an
// empty build stamp — is reported as unparsable rather than as zero.
func parseVersion(s string) (parts [3]int, ok bool) {
	m := versionPattern.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return parts, false
	}
	for i := 0; i < 3; i++ {
		n, err := strconv.Atoi(m[i+1])
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// newerRelease reports whether latest is a later release than current.
//
// A current version that cannot be parsed (a development build) counts as older
// than any released version, so a dev build is told a release exists instead of
// being told it is up to date.
func newerRelease(latest, current string) bool {
	l, lok := parseVersion(latest)
	if !lok {
		return false
	}
	c, cok := parseVersion(current)
	if !cok {
		return true
	}
	for i := 0; i < 3; i++ {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// releaseResponse is the subset of the GitHub release object we use.
type releaseResponse struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

// checkForUpdate asks GitHub for the latest release and decides which path an
// update would take.
//
// The path is decided by the release itself: the workflow publishes a bare sidecar
// binary only when the desktop shell did not change, so the presence of that asset
// is the signal, and no extra metadata file has to be kept in step. Until the
// workflow does that, every release reports the installer path, which is honest.
func checkForUpdate(current string) updatePayload {
	out := updatePayload{
		Current:   current,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	// GitHub rejects requests without a User-Agent, and the Accept header selects
	// the stable v3 JSON shape.
	req.Header.Set("User-Agent", "sabdopalon/"+current)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		out.Error = "Could not reach GitHub — check your connection."
		return out
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		out.Error = "GitHub is rate-limiting release checks. Try again later."
		return out
	}
	if resp.StatusCode != http.StatusOK {
		out.Error = fmt.Sprintf("GitHub answered %s.", resp.Status)
		return out
	}

	var rel releaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		out.Error = "Could not read the release information."
		return out
	}

	out.Latest = strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	out.PublishedAt = rel.PublishedAt
	out.Notes = firstLines(rel.Body, 12)
	out.URL = rel.HTMLURL
	out.Available = newerRelease(rel.TagName, current)
	out.Kind = "none"
	if !out.Available {
		return out
	}

	// A sidecar asset means the shell is unchanged, so the update can be applied
	// in place. Otherwise the release includes shell changes and needs the
	// installer.
	out.Kind = "installer"
	for _, a := range rel.Assets {
		if strings.Contains(strings.ToLower(a.Name), "sidecar") {
			out.Kind = "sidecar"
			out.Size = a.Size
			break
		}
	}
	if out.Kind == "installer" {
		for _, a := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
				out.Size = a.Size
				break
			}
		}
	}
	return out
}

// firstLines keeps a release note readable in a card: the first few non-empty
// lines, with markdown headings and list markers stripped so they do not render as
// literal syntax.
func firstLines(body string, max int) string {
	var kept []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimSpace(strings.TrimLeft(line, "#"))   // headings
		line = strings.TrimSpace(strings.TrimLeft(line, "-*+")) // list markers
		if line == "" {
			continue
		}
		kept = append(kept, line)
		if len(kept) == max {
			break
		}
	}
	return strings.Join(kept, "\n")
}

// handleAPIUpdate serves GET /api/update — the result of the last check, never a
// network call. Checking is an explicit action.
func (s *Server) handleAPIUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.json(w, map[string]string{"error": "method not allowed"})
		return
	}
	updateCache.mu.Lock()
	defer updateCache.mu.Unlock()
	payload := updateCache.got
	payload.Current = Version
	s.json(w, payload)
}

// handleAPIUpdateCheck serves POST /api/update/check.
//
// Rate limiting is left to the caller for now: GitHub allows 60 unauthenticated
// requests an hour, and the About page only checks when asked (or once per start),
// which stays far below that.
func (s *Server) handleAPIUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.json(w, map[string]string{"error": "method not allowed"})
		return
	}
	payload := checkForUpdate(Version)

	updateCache.mu.Lock()
	updateCache.got = payload
	updateCache.mu.Unlock()

	s.json(w, payload)
}
