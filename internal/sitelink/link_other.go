// Package sitelink — link_other.go: a plain directory symlink on Unix.
//
//go:build !windows

package sitelink

import (
	"fmt"
	"os"
)

func createLink(link, target string) error {
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create symlink %s -> %s: %w", link, target, err)
	}
	return nil
}
