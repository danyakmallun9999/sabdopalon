// Package update — download.go: fetch a release asset and prove it is the one we
// asked for, before anything runs it.
//
// The rule this file exists to enforce: an update that runs unverified code is worse
// than no update at all. The verified bytes are therefore never visible under the
// destination name until the checksum has matched.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DigestHex returns the hex part of a GitHub asset digest ("sha256:abc…"), or the
// value itself when it carries no algorithm prefix.
func DigestHex(digest string) string {
	_, hexPart, found := strings.Cut(strings.TrimSpace(digest), ":")
	if found && strings.TrimSpace(hexPart) != "" {
		return strings.ToLower(strings.TrimSpace(hexPart))
	}
	return strings.ToLower(strings.TrimSpace(digest))
}

// Download fetches url into dir under name and verifies it against digest.
//
// The bytes land in a temporary file and are hashed as they arrive; the file is
// renamed into place only after the hash matches, so an interrupted or corrupted
// download can never be mistaken for a finished one. progress, when supplied, is
// called with bytes received and the advertised total (-1 when the server does not
// say).
func Download(
	ctx context.Context,
	url, digest, dir, name string,
	progress func(done, total int64),
) (string, error) {
	want := DigestHex(digest)
	if want == "" {
		return "", fmt.Errorf("no checksum was published for this asset")
	}
	if name == "" {
		name = filepath.Base(url)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "sabdopalon-updater")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}

	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return "", err
	}
	tmpName := tmp.Name()
	// Whatever happens next, a partial file must not survive. This also covers the
	// success path: the rename below moves it, so the removal finds nothing.
	defer func() { _ = os.Remove(tmpName) }()

	hash := sha256.New()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := tmp.Write(buf[:n]); err != nil {
				_ = tmp.Close()
				return "", err
			}
			hash.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = tmp.Close()
			return "", fmt.Errorf("download interrupted: %w", readErr)
		}
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", want, got)
	}

	final := filepath.Join(dir, name)
	if err := os.Rename(tmpName, final); err != nil {
		return "", err
	}
	return final, nil
}
