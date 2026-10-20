package dashboard

import "testing"

// TestValidTLD pins what a TLD may be. There is no allow-list of "known" TLDs:
// the suffix is only a DNS label that Sabdopalon appends to each site name and
// that the proxy matches, so any well-formed label works — .test is merely the
// IANA-reserved testing suffix, not something the code treats specially.
func TestValidTLD(t *testing.T) {
	accept := []string{
		"localhost", "test", "acme", "internal",
		"wowo", "wiwi", "wkwk", // arbitrary invented labels must work
		"my-tld", "x1", "a", "xn--p1ai",
	}
	for _, tld := range accept {
		if !validTLD(tld) {
			t.Errorf("validTLD(%q) = false, want true", tld)
		}
	}

	reject := []string{
		"",               // a bare "." would make every host match
		"wo.wo",          // dots would change what counts as a site host
		"wo wo",          // a space breaks the hosts line
		"wowo/",          // path separator
		"-wowo", "wowo-", // a label may not start or end with a hyphen
		"WOWO",                // the handler lowercases first; the label itself must
		"wowo#",               // a comment character would truncate the hosts line
		"wowo.wiwi.wkwk.wowo", // far too long, and dots
	}
	for _, tld := range reject {
		if validTLD(tld) {
			t.Errorf("validTLD(%q) = true, want false", tld)
		}
	}
}
