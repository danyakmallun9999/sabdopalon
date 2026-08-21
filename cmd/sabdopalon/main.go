// Command sabdopalon is the single entry point for the Sabdopalon local
// development environment orchestrator.
//
// Build:
//
//	go build -o sabdopalon ./cmd/sabdopalon
package main

import (
	"os"

	"github.com/sabdopalon/sabdopalon/internal/app"
)

func main() {
	a, err := app.New()
	if err != nil {
		// Even if config load fails, allow pure informational commands to work.
		if len(os.Args) >= 2 && (os.Args[1] == "version" || os.Args[1] == "-v" || os.Args[1] == "--version") {
			os.Exit(runBare())
		}
		os.Stderr.WriteString("sabdopalon: " + err.Error() + "\n")
		os.Stderr.WriteString("hint: run this binary from the Sabdopalon root dir (where config/engine.toml lives)\n")
		os.Exit(1)
	}
	os.Exit(a.Run(os.Args[1:]))
}

// runBare handles version printing when config cannot be loaded.
func runBare() int {
	os.Stdout.WriteString("sabdopalon " + app.Version + "\n")
	return 0
}
