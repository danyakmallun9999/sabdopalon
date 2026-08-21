package toml

import "testing"

const sample = `
# comment line
[sabdopalon]
tld = "localhost"
root = "./sites"

[proxy]
http_port = 8080
enabled = true

[dashboard]
auto_open = false
`

func TestDecodeSections(t *testing.T) {
	d, err := DecodeString(sample)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := d.GetString("sabdopalon", "tld", ""); got != "localhost" {
		t.Errorf("tld = %q", got)
	}
	if got := d.GetInt("proxy", "http_port", 0); got != 8080 {
		t.Errorf("http_port = %d", got)
	}
	if !d.GetBool("proxy", "enabled", false) {
		t.Error("enabled should be true")
	}
}

func TestDefaults(t *testing.T) {
	d, err := DecodeString(sample)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := d.GetString("nope", "key", "fallback"); got != "fallback" {
		t.Errorf("missing section default = %q", got)
	}
	// auto_open explicitly false must not fall back to default true.
	if d.GetBool("dashboard", "auto_open", true) {
		t.Error("explicit false must win over default")
	}
}
