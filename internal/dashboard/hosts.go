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
				"Run `sabdopalon hosts:sync` in an elevated terminal, or keep the .localhost TLD, which resolves by itself. (%s)",
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
