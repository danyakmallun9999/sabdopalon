// Package dashboard — hosts.go: making a custom TLD actually resolve.
//
// The Settings page has always let a user replace the TLD, and the proxy has
// always routed it: hostToSite() compares the Host header against cfg.TLD, so a
// request for mysite.acme reached the right site the moment the setting was
// saved. What was missing was the step before it — the name has to resolve before
// a browser can even send that request.
//
//   - .localhost needs nothing: RFC 6761 reserves it, and Windows answers
//     anything.localhost with 127.0.0.1 (measured).
//   - Every other TLD needs an entry. mysite.acme and mysite.test both answer
//     "No such host is known" on a stock Windows box (measured), and nothing in
//     Sabdopalon ever wrote the hosts file.
//
// So changing the TLD produced exactly the reported symptom: the site "broke at
// the new domain", with no error anywhere, because the request never left the
// browser.
package dashboard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/sabdopalon/sabdopalon/internal/hostsync"
	"github.com/sabdopalon/sabdopalon/internal/ssl"
)

// SyncHosts brings the hosts file in step with the current configuration. It is
// safe to call at any time: a sync that changes nothing does not touch the file,
// which also means it needs no elevation when everything is already correct.
func (s *Server) SyncHosts() (hostsync.Result, error) {
	return hostsync.SyncForInstall(s.cfg)
}

// hostFilePath is the seam "what does the Settings page show" tests go through:
// the real hosts file needs administrator rights to write, but its state reads
// like any other file.
var hostFilePath = hostsync.Path

// HostsState answers "is the hosts file already in step?" without writing
// anything, so the page can show the gap and ask for exactly one approval when
// a real correction is needed.
func (s *Server) HostsState() (hostsync.State, error) {
	return hostsync.Of(hostFilePath(), s.cfg)
}

// elevationSupported says whether this OS can raise an administrator prompt
// from inside the process. It is a variable so tests exercise both answers on
// any platform.
var elevationSupported = func() bool { return runtime.GOOS == "windows" }

// startElevatedHostsSync asks Windows to run "hosts:sync" as Administrator.
//
// The dashboard process itself cannot write the hosts file — that is the whole
// point of the elevation step — but Start-Process -Verb RunAs is the documented
// way a normal process raises the UAC prompt. The elevated child inherits this
// process's environment, which is where SABDOPALON_DIR lives, and the working
// directory covers installs that were started from a plain terminal instead.
// It is a variable so tests can stub the spawn.
var startElevatedHostsSync = func(exe, dir string) error {
	script := fmt.Sprintf(
		"Start-Process -FilePath %s -ArgumentList 'hosts:sync' -WorkingDirectory %s -Verb RunAs -WindowStyle Hidden",
		psQuote(exe), psQuote(dir),
	)
	return exec.Command("powershell", "-NoProfile", "-Command", script).Run()
}

// psQuote writes a single-quoted PowerShell string literal, doubling any quote
// the path contains — a path under "Program Files"'-style names would otherwise
// break the script.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// RequestHostsElevation answers the Settings page's synchronize button.
//
// It deliberately refuses to raise a UAC prompt when nothing needs correcting,
// and on systems without an elevation mechanism it returns the CLI command the
// docs have always given, so every outcome is something the page can show.
func (s *Server) RequestHostsElevation() (started bool, message string) {
	st, err := s.HostsState()
	switch {
	case err != nil:
		return false, "Could not read the hosts file: " + err.Error()
	case st.OK():
		if st.SelfResolving {
			return false, "Nothing to map: ." + orDefault(st.TLD, "localhost") + " resolves by itself."
		}
		return false, fmt.Sprintf("The hosts file already maps %d hostname(s) for .%s.", len(st.Wanted), st.TLD)
	}
	if !elevationSupported() {
		return false, "Run `sudo sabdopalon hosts:sync` once in a terminal, then reopen this page."
	}
	exe, err := os.Executable()
	if err != nil {
		return false, "Could not locate the running executable: " + err.Error()
	}
	dir, _ := os.Getwd()
	if err := startElevatedHostsSync(exe, dir); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "cancel") {
			return false, "The administrator prompt was cancelled — the hosts file is unchanged."
		}
		return false, "Could not start the elevated sync: " + err.Error()
	}
	fixes := len(st.Missing) + len(st.Stale)
	return true, fmt.Sprintf(
		"Windows asked for approval to update %d hostname(s) for .%s — accept the prompt; this page shows the new state once the block lands.",
		fixes, st.TLD,
	)
}

// orDefault falls back to def when s is blank — a TLD gone empty counts as
// .localhost everywhere in the messages.
func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// applyTLDChange makes a newly configured TLD usable immediately and describes
// what it did, so the Settings page can say more than "restart required".
//
// Order matters: the hosts entries are what make the name resolve, the wildcard
// certificate is what makes HTTPS work at that name, and aliases are rebuilt so
// per-site overrides written against the old TLD are re-read.
func (s *Server) applyTLDChange() string {
	var parts []string

	res, err := s.SyncHosts()
	switch {
	case errors.Is(err, hostsync.ErrPermission):
		parts = append(parts, fmt.Sprintf(
			" The new domains could not be added to the hosts file: it needs administrator rights. "+
				"Click Synchronize (Administrator) on this page, or run `sabdopalon hosts:sync` "+
				"in an elevated terminal; keep the .localhost TLD and no rights are needed. (%s)",
			hostsync.Path()))
	case err != nil:
		parts = append(parts, " Hosts file: "+err.Error())
	case len(res.Hosts) == 0:
		parts = append(parts, " TLD .localhost resolves by itself, so no hosts entries are needed.")
	case res.Changed:
		parts = append(parts, " Added "+describeHosts(res.Hosts)+" to the hosts file.")
	default:
		parts = append(parts, " "+describeHosts(res.Hosts)+" were already in the hosts file.")
	}

	if issued, err := s.issueWildcardForTLD(); err != nil {
		parts = append(parts, " Certificate: "+err.Error())
	} else if issued {
		parts = append(parts, fmt.Sprintf(" Issued a wildcard certificate for *.%s; restart Sabdopalon so the HTTPS listener uses it.", s.cfg.TLD))
	}

	s.proxy.RebuildAliases()
	restarted := 0
	for _, site := range s.proxy.RunningSites() {
		name := strings.TrimSuffix(site.Host, "."+s.cfg.TLD)
		if err := s.proxy.RestartSite(name); err == nil {
			restarted++
		}
	}
	if restarted > 0 {
		parts = append(parts, fmt.Sprintf(" Restarted %d running site(s).", restarted))
	}
	return strings.Join(parts, "")
}

// issueWildcardForTLD issues *.tld, but only when this installation already has
// a local CA.
//
// Creating a CA is a trust decision, so it is never taken here on the back of
// saving a form: without a CA there is no HTTPS to fix, and the SSL page already
// offers to set one up. When a CA does exist, the old wildcard belongs to the
// previous TLD and would be served (or silently fall back to localhost.crt) at
// the new names, which reads as a broken certificate rather than a missing one.
func (s *Server) issueWildcardForTLD() (bool, error) {
	tld := strings.TrimSpace(s.cfg.TLD)
	if tld == "" || strings.EqualFold(tld, "localhost") {
		return false, nil
	}
	m := ssl.New(s.cfg)
	if !m.HasCA() {
		// No local CA, so there is no HTTPS to repair. Creating one here would
		// install a trust anchor because somebody edited a settings field; the
		// SSL page is where that decision belongs.
		return false, nil
	}
	if err := m.IssueSite("*." + tld); err != nil {
		return false, err
	}
	return true, nil
}

// describeHosts renders a host list for a sentence, collapsing the common suffix
// so ten sites read as "*.acme" rather than ten names.
func describeHosts(hosts []string) string {
	if len(hosts) == 0 {
		return "no hostnames"
	}
	if len(hosts) <= 3 {
		return strings.Join(hosts, ", ")
	}
	if i := strings.IndexByte(hosts[0], '.'); i >= 0 && i+1 < len(hosts[0]) {
		return fmt.Sprintf("%d hostnames (*.%s)", len(hosts), hosts[0][i+1:])
	}
	return fmt.Sprintf("%d hostnames", len(hosts))
}

// validTLD accepts a single DNS label: the TLD is appended as ".<tld>" and is
// also the suffix the proxy matches, so a value with dots or slashes would
// silently change what counts as a site host (or produce an unmatchable suffix).
func validTLD(tld string) bool {
	if tld == "" || len(tld) > 63 {
		return false
	}
	for i, r := range tld {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
		case r == '-':
			// A label may not start or end with a hyphen (RFC 1123).
			if i == 0 || i == len(tld)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
