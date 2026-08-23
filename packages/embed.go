// Package packages embeds the default package registry so the Sabdopalon
// binary is self-contained: desktop installs and fresh portable runs always
// have a registry even when packages/packages.toml was not shipped next to
// the executable.
//
// This file must live next to packages.toml — go:embed cannot cross package
// directory boundaries.
package packages

import _ "embed"

// DefaultRegistry holds the contents of packages.toml at build time.
//
//go:embed packages.toml
var DefaultRegistry string
