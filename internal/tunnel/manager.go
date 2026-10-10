package tunnel

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/winproc"
)

// Aliaser is the part of the proxy this package needs.
//
// A Quick Tunnel's hostname is random and only known once cloudflared has
// started, so the proxy has to learn it at runtime. Runtime hosts are kept apart
// from .sabdopalon.yml aliases, which are rebuilt from disk whenever a site's
// config changes and would otherwise wipe them.
type Aliaser interface {
	AddTunnelHost(host, site string)
	RemoveTunnelHost(host string)
}

// Runner starts cloudflared and blocks until it exits, mirroring its output
// into w. It exists as a seam: the whole launch pipeline — argument building for
// each retry plan, log mirroring, URL scraping, alias registration, stop — is
// testable without a network, a proxy or an installed cloudflared.
type Runner interface {
	Run(ctx context.Context, bin string, args []string, w io.Writer) error
}

// execRunner is the real one. The context is the kill switch: cancelling it
// terminates the process, which is how instance.stop() ends a tunnel.
type execRunner struct{}

func (execRunner) Run(ctx context.Context, bin string, args []string, w io.Writer) error {
	cmd := newCommand(ctx, bin, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// newCommand builds the exec.Cmd for cloudflared and marks it console-free.
//
// The marking matters more than it looks. The installed sidecar is linked with
// -H windowsgui (desktop/scripts/build-sidecar.sh), so it owns no console, and
// Windows hands any console-subsystem child a brand-new, visible window. That
// window is not cosmetic: it IS cloudflared's console, so closing it kills the
// tunnel — which is how "share publicly" presented in 0.11.5. Every other
// subsystem here (services, php, mysqld, devtools) already routes through
// winproc.Quiet; this package missed it, and a plain local `go build` hides the
// bug entirely because a console-linked sidecar lets the child inherit its
// console instead of allocating a new one.
//
// It is a function, not an inline exec.Command, so the Windows regression test
// can assert the flags directly — the window itself is unobservable from a
// console-linked test binary.
func newCommand(ctx context.Context, bin string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if ctx == nil {
		cmd = exec.Command(bin, args...)
	} else {
		cmd = exec.CommandContext(ctx, bin, args...)
	}
	winproc.Quiet(cmd)
	return cmd
}

// Options configures a Manager. Everything is injected so the package can be
// tested without a network, a proxy or an installed cloudflared.
type Options struct {
	RootDir  string        // install dir: holds bin/ and config/
	LogsDir  string        // where tunnel-<site>.log goes
	ProxyURL func() string // e.g. http://127.0.0.1:8080, asked at start time
	Aliaser  Aliaser       // may be nil (no proxy to teach)
	Binary   string        // explicit cloudflared path; empty = auto-detect
	Runner   Runner        // nil = run the real binary
	Logf     func(format string, args ...any)
	// RetryDelay spaces out restarts of a tunnel that dropped. nil = the real
	// backoff; tests shorten it so they do not sleep for seconds.
	RetryDelay func(attempt int) time.Duration
}

// Manager owns one process per shared site.
type Manager struct {
	opt   Options
	store *store

	mu     sync.Mutex
	insts  map[string]*instance
	bin    string
	closed bool
}

func New(opt Options) *Manager {
	if opt.Logf == nil {
		opt.Logf = func(string, ...any) {}
	}
	return &Manager{
		opt:   opt,
		store: loadStore(opt.RootDir),
		insts: map[string]*instance{},
	}
}

// binary resolves cloudflared, caching a success but re-probing a failure: the
// user who just ran the install command we printed should not be told to restart
// Sabdopalon.
func (m *Manager) binary() (string, error) {
	m.mu.Lock()
	if m.bin != "" {
		bin := m.bin
		m.mu.Unlock()
		return bin, nil
	}
	m.mu.Unlock()

	bin, err := findBinary(m.opt.RootDir, m.opt.Binary)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.bin = bin
	m.mu.Unlock()
	return bin, nil
}

// BinaryInfo reports which cloudflared would be used, so the dashboard can say
// so before anything is started (and quote the install command when there is
// none).
func (m *Manager) BinaryInfo() (path, ver string, err error) {
	bin, err := m.binary()
	if err != nil {
		return "", "", err
	}
	return bin, version(bin), nil
}

// SavedMail is the email list remembered for a site, for pre-filling the field.
func (m *Manager) SavedMail(site string) []string {
	return m.store.get(site).AllowedMail
}

// Start shares a site. It returns as soon as the process is launched; the
// public URL arrives a moment later through the state snapshot (the UI is
// subscribed to the SSE feed, which pushes it).
func (m *Manager) Start(site string, allowedMail []string) (State, error) {
	if err := validSite(site); err != nil {
		return State{}, err
	}
	if _, err := m.binary(); err != nil {
		return State{}, err
	}

	mail := normalizeMail(allowedMail)

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return State{}, fmt.Errorf("sabdopalon is shutting down")
	}
	if cur, ok := m.insts[site]; ok {
		st := cur.snapshot()
		if st.Status == StatusStarting || st.Status == StatusOnline {
			m.mu.Unlock()
			return st, nil // already shared — starting again is a no-op
		}
	}
	inst := newInstance(site, mail, m.logPath(site))
	m.insts[site] = inst
	m.mu.Unlock()

	_ = m.store.set(site, Prefs{AllowedMail: mail})
	go m.run(inst)
	return inst.snapshot(), nil
}

// Stop closes a site's tunnel and forgets its hostname.
func (m *Manager) Stop(site string) State {
	m.mu.Lock()
	inst := m.insts[site]
	m.mu.Unlock()
	if inst == nil {
		return State{Status: StatusOff}
	}
	inst.stop()
	// Deregister before clearing the URL: the hostname to remove lives there.
	m.removeAlias(inst)
	inst.setOff()
	return inst.snapshot()
}

// StopAll is the shutdown path. It also runs when the app is closing with the
// window still open, so it must not block: cloudflared is killed outright.
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.closed = true
	insts := make([]*instance, 0, len(m.insts))
	for _, i := range m.insts {
		insts = append(insts, i)
	}
	m.mu.Unlock()

	for _, i := range insts {
		i.stop()
	}
	for _, i := range insts {
		m.removeAlias(i)
		i.setOff()
	}
}

func (m *Manager) isClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// State returns one site's tunnel state ("off" when it has never been shared).
func (m *Manager) State(site string) State {
	m.mu.Lock()
	inst := m.insts[site]
	m.mu.Unlock()
	if inst == nil {
		return State{Status: StatusOff}
	}
	return inst.snapshot()
}

// Snapshot returns every state, for the SSE status feed the dashboard pages
// already subscribe to. Nothing here needs a push channel of its own: the feed
// re-serialises the snapshot every two seconds and sends it only when it
// changed, so a URL appears in the UI the moment it exists.
func (m *Manager) Snapshot() map[string]State {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]State, len(m.insts))
	for k, v := range m.insts {
		out[k] = v.snapshot()
	}
	return out
}

// plan is one way of starting cloudflared. The flags differ because cloudflared
// builds and networks do:
//
//   - --no-autoupdate stops a background self-update from restarting the process
//     mid-session, but builds that do not know the flag reject the whole command;
//   - the default transport is QUIC over UDP 7844, which plenty of networks
//     block, and http/2 over TCP 443 is the documented fallback.
type plan struct {
	auto     bool
	protocol string
}

var plans = []plan{
	{auto: true},
	{auto: false},
	{auto: false, protocol: "http2"},
}

// nextPlan reports which plan can rescue the failure of plans[i], or -1 when the
// failure is not one a different flag set would fix.
func nextPlan(i int, out string) int {
	if i < 0 || i+1 >= len(plans) {
		return -1
	}
	if plans[i].auto && UnknownFlag(out) {
		return i + 1
	}
	if plans[i].protocol == "" && EdgeUnreachable(out) {
		return len(plans) - 1
	}
	return -1
}

// run keeps a site's tunnel alive: launch, and decide what a death means.
func (m *Manager) run(inst *instance) {
	const maxDrops = 3
	drops := 0
	pi := 0

	for {
		out, err, gotURL := m.launch(inst, plans[pi])
		m.removeAlias(inst)

		if m.isClosed() || inst.stopped() {
			inst.setOff()
			return
		}
		if gotURL {
			// It was up and then died. Bring it back a few times, then stop
			// pretending: the URL changes on every attempt, so a link the user
			// already sent will not survive anyway.
			drops++
			if drops > maxDrops {
				inst.setError("The tunnel kept dropping — gave up after 3 restarts.")
				return
			}
			inst.setStarting()
			m.opt.Logf("tunnel %s: dropped, restarting (%d/%d)", inst.site, drops, maxDrops)
			time.Sleep(m.retryDelay(drops))
			continue
		}
		if next := nextPlan(pi, out); next >= 0 {
			m.opt.Logf("tunnel %s: retrying (plan %d) — %s", inst.site, next, describe(out, err))
			pi = next
			continue
		}
		if RateLimited(out) {
			inst.setError("Cloudflare is rate-limiting new quick tunnels — try again in a minute.")
			return
		}
		inst.setError(describe(out, err))
		return
	}
}

// launch runs cloudflared until it exits, mirroring its output to the log file
// and watching for the public URL.
func (m *Manager) launch(inst *instance, p plan) (out string, err error, gotURL bool) {
	bin, berr := m.binary()
	if berr != nil {
		return berr.Error(), berr, false
	}

	args := []string{"tunnel"}
	if p.auto {
		args = append(args, "--no-autoupdate")
	}
	args = append(args, "--url", m.proxyURL())
	for _, e := range inst.mail {
		args = append(args, "--allowed-mail", e)
	}
	if p.protocol != "" {
		args = append(args, "--protocol", p.protocol)
	}

	var logFile *os.File
	if f, ferr := m.openLog(inst.logPath); ferr == nil {
		logFile = f
		defer f.Close()
	}

	tail := newTailBuffer(8 << 10)
	var seen atomic.Bool
	w := &lineWriter{
		file:     logFile,
		onOutput: tail.add,
		onLine: func(line string) {
			u := ExtractURL(line)
			if u == "" || seen.Load() {
				return
			}
			host := hostFromURL(u)
			if !IsQuickTunnelHost(host) {
				return
			}
			seen.Store(true)
			inst.setURL(u)
			m.addAlias(host, inst.site)
			m.opt.Logf("tunnel %s: %s", inst.site, u)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	inst.setCancel(cancel)
	defer cancel()

	werr := m.runner().Run(ctx, bin, args, w)
	return tail.String(), werr, seen.Load()
}

func (m *Manager) runner() Runner {
	if m.opt.Runner != nil {
		return m.opt.Runner
	}
	return execRunner{}
}

func (m *Manager) retryDelay(attempt int) time.Duration {
	if m.opt.RetryDelay != nil {
		return m.opt.RetryDelay(attempt)
	}
	return backoff(attempt)
}

func (m *Manager) proxyURL() string {
	if m.opt.ProxyURL == nil {
		return "http://127.0.0.1:8080"
	}
	return m.opt.ProxyURL()
}

func (m *Manager) addAlias(host, site string) {
	if m.opt.Aliaser == nil || !IsQuickTunnelHost(host) {
		return
	}
	m.opt.Aliaser.AddTunnelHost(host, site)
}

func (m *Manager) removeAlias(inst *instance) {
	st := inst.snapshot()
	if st.URL == "" || m.opt.Aliaser == nil {
		return
	}
	host := hostFromURL(st.URL)
	if IsQuickTunnelHost(host) {
		m.opt.Aliaser.RemoveTunnelHost(host)
	}
}

func (m *Manager) logPath(site string) string {
	if m.opt.LogsDir == "" {
		return ""
	}
	return filepath.Join(m.opt.LogsDir, "tunnel-"+site+".log")
}

// openLog truncates a log that has grown past 1 MB: these files are read while
// a tunnel misbehaves, and an unbounded one is a slow disk leak on a dev box.
func (m *Manager) openLog(path string) (*os.File, error) {
	if path == "" {
		return nil, os.ErrInvalid
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if st, err := os.Stat(path); err == nil && st.Size() > 1<<20 {
		_ = os.Remove(path)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 2 * time.Second
	if d > 15*time.Second {
		d = 15 * time.Second
	}
	return d
}

// normalizeMail trims, drops empties and de-duplicates, keeping the order the
// user typed.
func normalizeMail(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, e := range in {
		e = strings.TrimSpace(e)
		if e == "" || seen[strings.ToLower(e)] {
			continue
		}
		seen[strings.ToLower(e)] = true
		out = append(out, e)
	}
	return out
}

// validSite rejects anything that is not a single safe path segment: the name
// reaches the filesystem (log file) and the proxy.
func validSite(site string) error {
	if site == "" || strings.ContainsAny(site, `/\`) || strings.Contains(site, "..") ||
		strings.HasPrefix(site, ".") {
		return fmt.Errorf("invalid site name %q", site)
	}
	return nil
}
