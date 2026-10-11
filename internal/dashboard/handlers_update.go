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
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// releasesAPI is a variable so tests can point it at their own server instead of
// the real GitHub. The list endpoint is what a beta channel needs: /releases/latest
// deliberately excludes pre-releases, so a beta user would never be offered one.
var (
	releasesAPI     = "https://api.github.com/repos/danyakmallun9999/sabdopalon/releases/latest"
	releasesListAPI = "https://api.github.com/repos/danyakmallun9999/sabdopalon/releases"
)

// Channels a user can follow. Stable is the default and the only one that ignores
// pre-releases.
const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

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

	// Asset that would be downloaded and the checksum it must match. Kept here so
	// the download endpoint works from the last check instead of re-asking GitHub.
	AssetURL    string `json:"asset_url,omitempty"`
	AssetDigest string `json:"asset_digest,omitempty"`
	AssetName   string `json:"asset_name,omitempty"`
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
		Name               string `json:"name"`
		Size               int64  `json:"size"`
		Digest             string `json:"digest"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// checkForUpdate asks for the latest stable release.
func checkForUpdate(current string) updatePayload {
	return checkForUpdateChannel(current, ChannelStable)
}

// checkForUpdateChannel asks for the release a channel should offer.
//
// Stable reads /releases/latest, which GitHub keeps free of pre-releases. Beta reads
// the list and takes the highest version on it, pre-releases included — which is the
// whole point of following beta, so it cannot be done with the "latest" shortcut.
func checkForUpdateChannel(current, channel string) updatePayload {
	if channel != ChannelBeta {
		rel, err := fetchRelease(releasesAPI)
		if err != "" {
			return updatePayload{Current: current, CheckedAt: time.Now().UTC().Format(time.RFC3339), Error: err}
		}
		return payloadFromRelease(rel, current)
	}

	list, errMsg := fetchReleaseList(releasesListAPI)
	if errMsg != "" {
		return updatePayload{Current: current, CheckedAt: time.Now().UTC().Format(time.RFC3339), Error: errMsg}
	}
	// Highest version wins, not the first entry: the list is ordered by publication
	// time, and a patch to an older line can be published after a newer pre-release.
	var best *releaseResponse
	bestParts := [3]int{}
	for i := range list {
		parts, ok := parseVersion(list[i].TagName)
		if !ok {
			continue
		}
		if best == nil || versionGreater(parts, bestParts) {
			best = &list[i]
			bestParts = parts
		}
	}
	if best == nil {
		return updatePayload{
			Current:   current,
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			Error:     "No releases were found on the beta channel.",
		}
	}
	return payloadFromRelease(*best, current)
}

// versionGreater reports whether a is a later version than b.
func versionGreater(a, b [3]int) bool {
	for i := 0; i < 3; i++ {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// fetchRelease fetches and decodes one release, returning an actionable sentence
// rather than an HTTP status.
func fetchRelease(url string) (releaseResponse, string) {
	body, status, errMsg := getJSON(url)
	if errMsg != "" {
		return releaseResponse{}, errMsg
	}
	if status != http.StatusOK {
		if status == http.StatusForbidden || status == http.StatusTooManyRequests {
			return releaseResponse{}, "GitHub is rate-limiting release checks. Try again later."
		}
		return releaseResponse{}, fmt.Sprintf("GitHub answered %d.", status)
	}
	var rel releaseResponse
	if err := json.Unmarshal(body, &rel); err != nil {
		return releaseResponse{}, "Could not read the release information."
	}
	return rel, ""
}

// fetchReleaseList fetches and decodes the release list.
func fetchReleaseList(url string) ([]releaseResponse, string) {
	body, status, errMsg := getJSON(url)
	if errMsg != "" {
		return nil, errMsg
	}
	if status != http.StatusOK {
		if status == http.StatusForbidden || status == http.StatusTooManyRequests {
			return nil, "GitHub is rate-limiting release checks. Try again later."
		}
		return nil, fmt.Sprintf("GitHub answered %d.", status)
	}
	var list []releaseResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, "Could not read the release list."
	}
	return list, ""
}

// getJSON performs the one kind of network call this file makes.
func getJSON(url string) ([]byte, int, string) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err.Error()
	}
	// GitHub rejects requests without a User-Agent, and the Accept header selects
	// the stable v3 JSON shape.
	req.Header.Set("User-Agent", "sabdopalon/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, "Could not reach GitHub — check your connection."
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, "Could not read GitHub's answer."
	}
	return body, resp.StatusCode, ""
}

// payloadFromRelease turns a release into what the About page renders.
//
// The path an update would take is decided by the release itself: the workflow
// publishes a bare sidecar binary only when the desktop shell did not change, so the
// presence of that asset is the signal, and no extra metadata has to be kept in step.
func payloadFromRelease(rel releaseResponse, current string) updatePayload {
	out := updatePayload{
		Current:     current,
		CheckedAt:   time.Now().UTC().Format(time.RFC3339),
		Latest:      strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v"),
		PublishedAt: rel.PublishedAt,
		Notes:       firstLines(rel.Body, 12),
		URL:         rel.HTMLURL,
	}
	out.Available = newerRelease(rel.TagName, current)
	out.Kind = "none"
	if !out.Available {
		return out
	}

	out.Kind = "installer"
	for _, a := range rel.Assets {
		if strings.Contains(strings.ToLower(a.Name), "sidecar") {
			out.Kind = "sidecar"
			out.Size = a.Size
			out.AssetURL = a.BrowserDownloadURL
			out.AssetDigest = a.Digest
			out.AssetName = a.Name
			break
		}
	}
	if out.Kind == "installer" {
		for _, a := range rel.Assets {
			if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
				out.Size = a.Size
				out.AssetURL = a.BrowserDownloadURL
				out.AssetDigest = a.Digest
				out.AssetName = a.Name
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
