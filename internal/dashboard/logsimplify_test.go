package dashboard

import (
	"os"
	"strings"
	"testing"
)

// TestSimplifiedRealComposerOutput runs the simplified view over a real captured
// log: `composer create-project laravel/laravel`, escape codes and all. The
// fixture is byte-for-byte what composer and artisan wrote, because a hand-made
// sample is exactly how a filter ends up not matching the output it must filter.
func TestSimplifiedRealComposerOutput(t *testing.T) {
	raw, err := os.ReadFile("testdata/create-laravel.log")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}

	var log createLog
	// Fed in awkward chunks on purpose: a child process does not deliver lines,
	// it delivers pipe buffers, and an escape sequence can arrive in two pieces.
	for i := 0; i < len(raw); i += 37 {
		end := i + 37
		if end > len(raw) {
			end = len(raw)
		}
		if _, err := log.Write(raw[i:end]); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got := log.Simplified()
	if got == "" {
		t.Fatal("the simplified log is empty")
	}

	// Nothing a terminal eats should reach the page.
	for _, bad := range []string{"\x1b", "\r", "\b", "[37;44m", "[39;49m", "[90m", "DONE\x1b"} {
		if strings.Contains(got, bad) {
			t.Errorf("simplified log still carries %q:\n%s", bad, got)
		}
	}

	// The load-bearing lines survive, and the noise does not.
	for _, want := range []string{
		"Creating laravel project 'blog'",
		"Running composer create-project laravel/laravel",
		"Creating a \"laravel/laravel\" project at \"./blog\"",
		"Lock file operations: 109 installs, 0 updates, 0 removals",
		"Writing lock file",
		"Installing dependencies from lock file (including require-dev)",
		"Project created at sites/blog/",
		"Visit: http://blog.localhost/",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("simplified log lost %q:\n%s", want, got)
		}
	}

	// Composer's two hundred package lines became counting lines.
	if !strings.Contains(got, "Locking packages… ") {
		t.Errorf("the Locking run was not collapsed:\n%s", got)
	}
	if !strings.Contains(got, "Installing packages… ") {
		t.Errorf("the Installing run was not collapsed:\n%s", got)
	}

	// Every line has to say something: no dots-only rules, no progress bars.
	for _, line := range strings.Split(got, "\n") {
		if !hasLetter(line) {
			t.Errorf("line without a letter survived: %q", line)
		}
		if strings.Contains(line, "....") {
			t.Errorf("dot padding survived: %q", line)
		}
		if len(line) > maxLogLineLen+1 {
			t.Errorf("line is %d chars, past the %d limit: %q", len(line), maxLogLineLen, line)
		}
	}

	// The whole point: a fraction of the bytes, and a readable number of lines.
	if len(got) >= len(raw)/2 {
		t.Errorf("simplified output is %d bytes against %d raw: it is not simplifying", len(got), len(raw))
	}
	if n := len(strings.Split(got, "\n")); n > 60 {
		t.Errorf("simplified log is %d lines; that is still a wall of text", n)
	}
	t.Logf("raw %d bytes → simplified %d bytes, %d lines", len(raw), len(got), len(strings.Split(got, "\n")))
}

// TestSimplifiedCarriageReturnRewrites: composer redraws its progress with bare
// carriage returns, so a bar drawn ten times is one line and only the last state
// is true.
func TestSimplifiedCarriageReturnRewrites(t *testing.T) {
	var log createLog
	if _, err := log.Write([]byte("working 10%\rworking 50%\rworking 100%\nDone.\n")); err != nil {
		t.Fatal(err)
	}
	got := log.Simplified()
	if strings.Contains(got, "10%") || strings.Contains(got, "50%") {
		t.Errorf("intermediate states survived:\n%s", got)
	}
	if !strings.Contains(got, "working 100%") || !strings.Contains(got, "Done.") {
		t.Errorf("the final state is missing:\n%s", got)
	}
}

// TestSimplifiedHandlesSplitEscapes: an escape sequence split across two writes
// must not leave half of itself in the output.
func TestSimplifiedHandlesSplitEscapes(t *testing.T) {
	var log createLog
	for _, chunk := range []string{"\x1b[3", "7;44m INFO \x1b[39", ";49m Preparing database.\n"} {
		if _, err := log.Write([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	if got := log.Simplified(); got != "INFO Preparing database." {
		t.Errorf("got %q, want the text without any escape remnant", got)
	}
}

// TestSimplifiedKeepsAMovingLine: a line still being written is shown as it
// stands, which is what makes a long step look alive instead of frozen.
func TestSimplifiedKeepsAMovingLine(t *testing.T) {
	var log createLog
	if _, err := log.Write([]byte("Downloading laravel/laravel")); err != nil {
		t.Fatal(err)
	}
	if got := log.Simplified(); !strings.Contains(got, "Downloading laravel/laravel") {
		t.Errorf("a partial line was held back: %q", got)
	}
	if _, err := log.Write([]byte(" (v13.11.0)\n")); err != nil {
		t.Fatal(err)
	}
	got := log.Simplified()
	if strings.Count(got, "Downloading") != 1 {
		t.Errorf("the completed line was duplicated:\n%s", got)
	}
	if !strings.Contains(got, "Downloading laravel/laravel (v13.11.0)") {
		t.Errorf("the completed line is wrong:\n%s", got)
	}
}

// TestLogIsBounded: a long install cannot grow the payload without limit, and the
// raw copy keeps the end of the output on a line boundary.
func TestLogIsBounded(t *testing.T) {
	var log createLog
	line := strings.Repeat("installing something\n", 40)
	for i := 0; i < 400; i++ {
		if _, err := log.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	simplified := log.Simplified()
	if n := len(strings.Split(simplified, "\n")); n > maxLogLines {
		t.Errorf("simplified view holds %d lines, past the %d limit", n, maxLogLines)
	}
	raw := log.Raw()
	if len(raw) > rawLogKeep {
		t.Errorf("raw view holds %d bytes, past the %d limit", len(raw), rawLogKeep)
	}
	if strings.HasPrefix(raw, "stalling") {
		t.Error("the raw view starts mid-line")
	}
}

// TestSimplifiedDropsProgressBars pins the rule that keeps composer's bars out:
// a line with no letters is drawn for a terminal, not said to a person.
func TestSimplifiedDropsProgressBars(t *testing.T) {
	var log createLog
	body := "    0 [>---------------------------]    0 [->--------------------------]\n" +
		" 12/99 [====>-----------------------]  12%\n" +
		"----------------------------\n" +
		"Installing packages from lock file\n"
	if _, err := log.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	got := log.Simplified()
	if strings.Contains(got, "[") || strings.Contains(got, "---") {
		t.Errorf("a progress bar survived:\n%s", got)
	}
	if !strings.Contains(got, "Installing packages from lock file") {
		t.Errorf("a real line was dropped:\n%s", got)
	}
}

// TestCollapsedRunResetsBetweenSteps: the counter belongs to one composer step,
// and a later run counts from one again.
func TestCollapsedRunResetsBetweenSteps(t *testing.T) {
	var log createLog
	body := "  - Locking aaa/one (1.0)\n" +
		"  - Locking bbb/two (2.0)\n" +
		"Writing lock file\n" +
		"  - Installing ccc/three (3.0): Extracting archive\n" +
		"  - Installing ddd/four (4.0): Extracting archive\n"
	if _, err := log.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	got := log.Simplified()
	if !strings.Contains(got, "Locking packages… 2 (last: bbb/two 2.0)") {
		t.Errorf("the Locking counter is wrong:\n%s", got)
	}
	if !strings.Contains(got, "Installing packages… 2 (last: ddd/four 4.0)") {
		t.Errorf("the Installing counter did not start fresh:\n%s", got)
	}
	if strings.Contains(got, "Installing packages… 3") {
		t.Errorf("the two runs were counted together:\n%s", got)
	}
}
