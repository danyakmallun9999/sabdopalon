//go:build !windows

package database

import "github.com/sabdopalon/sabdopalon/internal/config"

// gracefulStop is a no-op on Unix: signalTerm already delivers SIGTERM to the
// daemon's process group, which IS the clean shutdown path. Nothing to add.
func gracefulStop(cfg *config.Engine, engine string, port int) bool { return false }
