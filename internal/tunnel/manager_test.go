package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type runnerFunc func(ctx context.Context, bin string, args []string, w io.Writer) error

func (f runnerFunc) Run(ctx context.Context, bin string, args []string, w io.Writer) error {
	return f(ctx, bin, args, w)
}

// fakeAliaser stands in for the proxy: it records what the manager taught it.
type fakeAliaser struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeAliaser() *fakeAliaser { return &fakeAliaser{m: map[string]string{}} }

func (f *fakeAliaser) AddTunnelHost(host, site string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[host] = site
}

func (f *fakeAliaser) RemoveTunnelHost(host string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.m, host)
}

func (f *fakeAliaser) get(host string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.m[host]
}

const quickHost = "calm-frost-abc-def.trycloudflare.com"
const quickURL = "https://" + quickHost

// fakeBinary creates the file binary() insists on. The Runner never executes it.
func fakeBinary(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "cloudflared-fake")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestManager(t *testing.T, r Runner, alias Aliaser) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	m := New(Options{
		RootDir:    dir,
		LogsDir:    filepath.Join(dir, "logs"),
		Binary:     fakeBinary(t, dir),
		ProxyURL:   func() string { return "http://127.0.0.1:8080" },
		Aliaser:    alias,
		Runner:     r,
		RetryDelay: func(int) time.Duration { return time.Millisecond },
	})
	return m, dir
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// argLog records the arguments of every launch attempt.
type argLog struct {
	mu sync.Mutex
	v  [][]string
}

func (l *argLog) add(args []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.v = append(l.v, append([]string{}, args...))
}

func (l *argLog) at(i int) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if i >= len(l.v) {
		return nil
	}
	return l.v[i]
}

func (l *argLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.v)
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// argValue returns the value following a flag.
func argValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestStartPublishesURLAndTeachesTheProxy(t *testing.T) {
	alias := newFakeAliaser()
	log := &argLog{}
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		log.add(args)
		fmt.Fprint(w, banner)
		<-ctx.Done() // stay up until the manager stops us
		return nil
	})
	m, dir := newTestManager(t, r, alias)

	st, err := m.Start("myapp", []string{"a@b.com", " a@b.com ", ""})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if st.Status != StatusStarting {
		t.Errorf("immediate status = %q, want starting", st.Status)
	}

	waitFor(t, "status online", func() bool { return m.State("myapp").Status == StatusOnline })
	st = m.State("myapp")
	if st.URL != quickURL {
		t.Errorf("URL = %q, want %q", st.URL, quickURL)
	}
	if !st.Enabled {
		t.Error("an online tunnel must report enabled")
	}
	if got := alias.get(quickHost); got != "myapp" {
		t.Errorf("proxy taught %q, want myapp — without this the public host 404s", got)
	}

	// The tunnel points at the proxy, not at PHP, and carries the email list.
	args := log.at(0)
	if got := argValue(args, "--url"); got != "http://127.0.0.1:8080" {
		t.Errorf("--url = %q, want the proxy", got)
	}
	if !hasArg(args, "--no-autoupdate") {
		t.Error("--no-autoupdate missing: a background update would restart the tunnel mid-session")
	}
	if got := argValue(args, "--allowed-mail"); got != "a@b.com" {
		t.Errorf("--allowed-mail = %q, want a@b.com", got)
	}
	for _, a := range args {
		if a == "" {
			t.Error("an empty email was passed through")
		}
	}

	// The email list is remembered (normalised, de-duplicated) for next time.
	if got := m.SavedMail("myapp"); len(got) != 1 || got[0] != "a@b.com" {
		t.Errorf("SavedMail = %v, want [a@b.com]", got)
	}

	// cloudflared's own output is kept on disk for troubleshooting.
	data, err := os.ReadFile(filepath.Join(dir, "logs", "tunnel-myapp.log"))
	if err != nil {
		t.Fatalf("log file: %v", err)
	}
	if !strings.Contains(string(data), "quick Tunnel has been created") {
		t.Error("log file does not hold cloudflared's output")
	}

	if st := m.Stop("myapp"); st.Status != StatusOff {
		t.Errorf("after Stop status = %q, want off", st.Status)
	}
	if got := alias.get(quickHost); got != "" {
		t.Errorf("alias survived Stop (%q): the public host must stop resolving", got)
	}
}

func TestStartTwiceLaunchesOneProcess(t *testing.T) {
	alias := newFakeAliaser()
	log := &argLog{}
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		log.add(args)
		fmt.Fprint(w, banner)
		<-ctx.Done()
		return nil
	})
	m, _ := newTestManager(t, r, alias)

	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "online", func() bool { return m.State("myapp").Status == StatusOnline })
	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if n := log.count(); n != 1 {
		t.Errorf("launched %d processes, want 1 (sharing twice is a no-op)", n)
	}
	m.StopAll()
}

func TestUnknownFlagRetriesWithoutAutoupdate(t *testing.T) {
	alias := newFakeAliaser()
	log := &argLog{}
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		log.add(args)
		if log.count() == 1 {
			fmt.Fprintln(w, "Incorrect Usage: flag provided but not defined: -no-autoupdate")
			return errors.New("exit status 1")
		}
		fmt.Fprint(w, banner)
		<-ctx.Done()
		return nil
	})
	m, _ := newTestManager(t, r, alias)

	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "online after retry", func() bool { return m.State("myapp").Status == StatusOnline })
	if !hasArg(log.at(0), "--no-autoupdate") {
		t.Error("first attempt should carry --no-autoupdate")
	}
	if hasArg(log.at(1), "--no-autoupdate") {
		t.Error("retry after an unknown-flag error must drop --no-autoupdate")
	}
	m.StopAll()
}

func TestEdgeFailureRetriesOverHTTP2(t *testing.T) {
	alias := newFakeAliaser()
	log := &argLog{}
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		log.add(args)
		if log.count() == 1 {
			fmt.Fprintln(w, "ERR Failed to dial to edge with quic: timeout: no recent network activity")
			return errors.New("exit status 1")
		}
		fmt.Fprint(w, banner)
		<-ctx.Done()
		return nil
	})
	m, _ := newTestManager(t, r, alias)

	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "online after http/2 retry", func() bool { return m.State("myapp").Status == StatusOnline })
	if got := argValue(log.at(1), "--protocol"); got != "http2" {
		t.Errorf("retry protocol = %q, want http2 (QUIC is blocked on this class of network)", got)
	}
	m.StopAll()
}

func TestRateLimitedExplainsItself(t *testing.T) {
	alias := newFakeAliaser()
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		fmt.Fprintln(w, "ERR Failed to request quick Tunnel: 429 Too Many Requests")
		return errors.New("exit status 1")
	})
	m, _ := newTestManager(t, r, alias)

	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "error state", func() bool { return m.State("myapp").Status == StatusError })
	if got := m.State("myapp").Error; !strings.Contains(got, "rate-limiting") {
		t.Errorf("error = %q, want a sentence about Cloudflare rate-limiting", got)
	}
}

func TestDroppedTunnelGivesUp(t *testing.T) {
	alias := newFakeAliaser()
	log := &argLog{}
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		log.add(args)
		fmt.Fprint(w, banner)
		return nil // dies immediately, every time
	})
	m, _ := newTestManager(t, r, alias)

	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "giving up", func() bool { return m.State("myapp").Status == StatusError })
	if n := log.count(); n != 4 { // the first run plus three restarts
		t.Errorf("attempts = %d, want 4", n)
	}
	if got := m.State("myapp").Error; !strings.Contains(got, "kept dropping") {
		t.Errorf("error = %q, want a sentence about dropping", got)
	}
	if got := alias.get(quickHost); got != "" {
		t.Errorf("alias left behind after giving up: %q", got)
	}
}

func TestMissingCloudflaredExplainsHowToInstall(t *testing.T) {
	if _, err := exec.LookPath("cloudflared"); err == nil {
		t.Skip("cloudflared is installed here, so the not-found path cannot be exercised")
	}
	dir := t.TempDir()
	m := New(Options{
		RootDir:  filepath.Join(dir, "empty"),
		Binary:   filepath.Join(dir, "nowhere", "cloudflared"),
		ProxyURL: func() string { return "http://127.0.0.1:8080" },
		Runner:   runnerFunc(func(context.Context, string, []string, io.Writer) error { return nil }),
	})
	_, err := m.Start("myapp", nil)
	if err == nil {
		t.Fatal("expected an error without cloudflared")
	}
	if !strings.Contains(err.Error(), InstallHint()) {
		t.Errorf("error %q does not quote the install command %q", err, InstallHint())
	}
}

func TestInvalidSiteNameIsRejected(t *testing.T) {
	m, _ := newTestManager(t, runnerFunc(func(context.Context, string, []string, io.Writer) error { return nil }), newFakeAliaser())
	for _, bad := range []string{"", "../etc", "a/b", `a\b`, ".hidden"} {
		if _, err := m.Start(bad, nil); err == nil {
			t.Errorf("Start(%q) was accepted", bad)
		}
	}
}

func TestStopAllClearsEveryAlias(t *testing.T) {
	alias := newFakeAliaser()
	r := runnerFunc(func(ctx context.Context, bin string, args []string, w io.Writer) error {
		fmt.Fprintf(w, "INF |  https://%s.trycloudflare.com  |\n", strings.TrimPrefix(filepath.Base(bin), "."))
		<-ctx.Done()
		return nil
	})
	m, _ := newTestManager(t, r, alias)
	if _, err := m.Start("myapp", nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "online", func() bool { return m.State("myapp").Status == StatusOnline })

	m.StopAll()
	snap := m.Snapshot()
	if st := snap["myapp"]; st.Status != StatusOff {
		t.Errorf("StopAll left status %q", st.Status)
	}
	for host := range alias.m {
		t.Errorf("alias %q survived StopAll", host)
	}
}
