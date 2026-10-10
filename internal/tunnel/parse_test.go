package tunnel

import "testing"

// The banner is copied from a real cloudflared run, box-drawing characters and
// all: parsing the sentence instead of the hostname is how these integrations
// break when Cloudflare rewords it.
const banner = `2026-10-10T12:00:00Z INF Thank you for trying Cloudflare Tunnel. Doing so, without a Cloudflare account, is a quick way to experiment and try it out.
2026-10-10T12:00:01Z INF Requesting new quick Tunnel on trycloudflare.com...
2026-10-10T12:00:02Z INF +--------------------------------------------------------------------------------------------+
2026-10-10T12:00:02Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |
2026-10-10T12:00:02Z INF |  https://calm-frost-abc-def.trycloudflare.com                                              |
2026-10-10T12:00:02Z INF +--------------------------------------------------------------------------------------------+
`

func TestExtractURLFromBanner(t *testing.T) {
	got := ExtractURL(banner)
	want := "https://calm-frost-abc-def.trycloudflare.com"
	if got != want {
		t.Fatalf("ExtractURL = %q, want %q", got, want)
	}
}

func TestExtractURLFromJSONOutput(t *testing.T) {
	// --output json puts the same text in a log line; matching the hostname
	// rather than the prose survives both formats.
	line := `{"level":"info","time":"2026-10-10T12:00:02Z","message":"Your quick Tunnel has been created! Visit it at https://plain-voice-xyz.trycloudflare.com"}`
	if got, want := ExtractURL(line), "https://plain-voice-xyz.trycloudflare.com"; got != want {
		t.Fatalf("ExtractURL = %q, want %q", got, want)
	}
}

func TestExtractURLCaseAndAbsence(t *testing.T) {
	if got := ExtractURL("INF HTTPS://LOUD-NAME.trycloudflare.com"); got != "https://loud-name.trycloudflare.com" {
		t.Errorf("case not normalised: %q", got)
	}
	if got := ExtractURL("INF no url here https://example.com"); got != "" {
		t.Errorf("matched a non-tunnel URL: %q", got)
	}
	// A lookalike domain must not pass: only the exact suffix counts.
	if got := ExtractURL("https://evil-trycloudflare.com.attacker.net"); got != "" {
		t.Errorf("matched a lookalike host: %q", got)
	}
}

func TestIsQuickTunnelHost(t *testing.T) {
	cases := map[string]bool{
		"calm-frost-abc.trycloudflare.com":     true,
		"calm-frost-abc.TRYCLOUDFLARE.COM":     true,
		"calm-frost-abc.trycloudflare.com:443": true, // port is stripped
		"trycloudflare.com":                    false,
		".trycloudflare.com":                   false,
		"evil-trycloudflare.com.attacker.net":  false,
		"myapp.localhost":                      false,
		"":                                     false,
		"calm-frost-abc.trycloudflare.com/x":   false,
	}
	for host, want := range cases {
		if got := IsQuickTunnelHost(host); got != want {
			t.Errorf("IsQuickTunnelHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestFailureClassifiers(t *testing.T) {
	if !UnknownFlag("Incorrect Usage: flag provided but not defined: -no-autoupdate") {
		t.Error("unknown-flag output not recognised")
	}
	if !EdgeUnreachable("ERR Failed to dial to edge with quic: timeout: no recent network activity") {
		t.Error("edge failure not recognised")
	}
	if !RateLimited("ERR Failed to request quick Tunnel: 429 Too Many Requests") {
		t.Error("rate limit not recognised")
	}
	if UnknownFlag("Your quick Tunnel has been created!") {
		t.Error("false positive on a healthy run")
	}
}

func TestDescribeFindsTheReason(t *testing.T) {
	out := "2026-10-10T12:00:00Z INF Starting tunnel\n" +
		"2026-10-10T12:00:01Z ERR Failed to request quick Tunnel: context deadline exceeded\n"
	if got, want := describe(out, nil), "Failed to request quick Tunnel: context deadline exceeded"; got != want {
		t.Errorf("describe = %q, want %q", got, want)
	}
	// A box line on its own says nothing, so it must not become the message.
	boxOnly := "2026-10-10T12:00:00Z INF +------------------+\n2026-10-10T12:00:00Z INF |                  |\n"
	if got := describe(boxOnly, nil); got == "|" || got == "" {
		t.Errorf("describe returned a meaningless message: %q", got)
	}
}
