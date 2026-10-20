// Package sysinstall — node.go: Node.js download URLs, checksums, and
// per-platform extraction.
//
// All URLs and SHA-256 values were verified with `curl -fsI` + the official
// SHASUMS256.txt before pinning (2026-08-25). Node LTS v24 (Krypton).
package sysinstall

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// nodeURL returns the Node.js download URL for this OS/arch, "" if unsupported.
func nodeURL(version string) string {
	switch runtime.GOOS {
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-linux-x64.tar.xz", version, version)
		case "arm64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-linux-arm64.tar.xz", version, version)
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-darwin-x64.tar.gz", version, version)
		case "arm64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-darwin-arm64.tar.gz", version, version)
		}
	case "windows":
		switch runtime.GOARCH {
		case "amd64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-win-x64.zip", version, version)
		case "arm64":
			return fmt.Sprintf("https://nodejs.org/dist/%s/node-%s-win-arm64.zip", version, version)
		}
	}
	return ""
}

// nodeSHA returns the pinned SHA-256 for the platform's Node.js artifact.
//
// Verified against https://nodejs.org/dist/v24.22.0/SHASUMS256.txt. A wrong
// value here is not a cosmetic bug: Install refuses an artifact whose digest
// does not match, so a stale table turns into "checksum mismatch" on the first
// `sabdopalon add node` and nothing else.
func nodeSHA(version string) string {
	if version != nodeVersion {
		return ""
	}
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "linux/amd64":
		return "cc2f601cee82d9a5a233302fe307abc600727035fd7d2c3a3571755989e95008"
	case "linux/arm64":
		return "fef00bb4d36e1c50104a9775a10359beee98f162b4104dd12472c75065c6a143"
	case "darwin/amd64":
		return "495e3f273c645f3c4677d438304cdfc701729ea5dec5728f5c8e76a2dece4b91"
	case "darwin/arm64":
		return "6a9724d9b50a5f22b4816cded441b5a8d9ddae546191a5e5b7efa5a5e2ee26d5"
	case "windows/amd64":
		return "f9683a19877e12a9b167305c8db71040d812876359de21f0d2e97c3f321f80c3"
	case "windows/arm64":
		return "3a5c209a5bc59e65b3bc0fa13ed5e166a26bd3d55332847f498b76b037408d61"
	}
	return ""
}

// installNode extracts the Node.js archive into the per-user prefix and
// symlinks/copies the binaries (node, npm, npx) to the prefix root.
func installNode(archivePath, prefix string, p Progress) (string, error) {
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return "", err
	}

	switch runtime.GOOS {
	case "windows":
		return installNodeWindows(archivePath, prefix, p)
	default:
		return installNodeUnix(archivePath, prefix, p)
	}
}

// installNodeUnix extracts the tarball (xz on Linux, gz on macOS) and links
// node/npm/npx into the prefix.
func installNodeUnix(archivePath, prefix string, p Progress) (string, error) {
	// Linux uses .tar.xz; macOS uses .tar.gz. Detect by extension.
	var r io.Reader
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if strings.HasSuffix(archivePath, ".tar.gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return "", fmt.Errorf("gunzip: %w", err)
		}
		defer gz.Close()
		r = gz
	} else if strings.HasSuffix(archivePath, ".tar.xz") {
		// xz decompression needs an external xz binary (or Go's xz). We shell
		// out to the system tar which handles xz on most modern systems.
		return installNodeUnixXZ(archivePath, prefix, p)
	} else {
		r = f
	}

	// Extract tar.gz.
	tr := tar.NewReader(r)
	rootName := "" // top-level folder name, e.g. "node-v24.19.0-linux-x64"
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read: %w", err)
		}
		if rootName == "" {
			parts := strings.SplitN(filepath.Clean(hdr.Name), "/", 2)
			rootName = parts[0]
		}
		target := filepath.Join(prefix, hdr.Name)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return "", err
			}
			out.Close()
		case tar.TypeSymlink:
			_ = os.Symlink(hdr.Linkname, target)
		}
	}

	// Link node/npm/npx from <prefix>/<root>/bin/* to <prefix>/.
	binDir := filepath.Join(prefix, rootName, "bin")
	for _, name := range []string{"node", "npm", "npx"} {
		src := filepath.Join(binDir, name)
		dst := filepath.Join(prefix, name)
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			p.Printf("  ⚠  could not link %s: %v\n", name, err)
		}
	}
	return filepath.Join(prefix, "node"), nil
}

// installNodeUnixXZ extracts a .tar.xz by shelling out to the system tar (which
// handles xz on modern Linux). Mirrors pkgmgr's extractInnerTxz approach.
func installNodeUnixXZ(archivePath, prefix string, p Progress) (string, error) {
	// Extract to a temp staging dir, then move the bin/ links.
	staging := filepath.Join(prefix, ".node-staging")
	_ = os.RemoveAll(staging)
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	cmd := execCommand("tar", "-xJf", archivePath, "-C", staging)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tar -xJ: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	// Find the extracted root folder.
	entries, err := os.ReadDir(staging)
	if err != nil {
		return "", err
	}
	rootName := ""
	for _, e := range entries {
		if e.IsDir() {
			rootName = e.Name()
			break
		}
	}
	if rootName == "" {
		return "", fmt.Errorf("no top-level folder in extracted archive")
	}

	// Move the extracted tree into the prefix.
	srcDir := filepath.Join(staging, rootName)
	dstDir := filepath.Join(prefix, rootName)
	_ = os.RemoveAll(dstDir)
	if err := osRename(srcDir, dstDir); err != nil {
		return "", fmt.Errorf("move extracted tree: %w", err)
	}

	// Link node/npm/npx from <prefix>/<root>/bin/* to <prefix>/.
	binDir := filepath.Join(dstDir, "bin")
	for _, name := range []string{"node", "npm", "npx"} {
		src := filepath.Join(binDir, name)
		dst := filepath.Join(prefix, name)
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			p.Printf("  ⚠  could not link %s: %v\n", name, err)
		}
	}
	return filepath.Join(prefix, "node"), nil
}
