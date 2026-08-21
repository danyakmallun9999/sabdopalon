// Package trust installs the Sabdopalon root CA into the OS certificate
// trust store so browsers accept the locally-issued HTTPS certificates
// without warnings.
//
// Platform support:
//   - Linux: copies the CA to /usr/local/share/ca-certificates/ and runs
//     update-ca-certificates (Debian/Ubuntu/Mint). Requires root.
//   - macOS: uses `security add-trusted-cert` (requires root/admin).
//   - Windows: uses `certutil -addstore` (requires admin).
//
// If elevation is not available, the user is instructed to run it manually.
package trust

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// InstallCA installs the root CA into the system trust store.
// Returns true if it succeeded, false if manual action is needed.
func InstallCA(cfg *config.Engine) (bool, error) {
	caCert := filepath.Join(cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	if !fileExists(caCert) {
		return false, fmt.Errorf("root CA not found at %s — run 'sabdopalon ssl:ca' first", caCert)
	}

	switch runtime.GOOS {
	case "linux":
		return installLinux(caCert)
	case "darwin":
		return installMacOS(caCert)
	case "windows":
		return installWindows(caCert)
	default:
		return false, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// UninstallCA removes the root CA from the system trust store.
func UninstallCA(cfg *config.Engine) error {
	switch runtime.GOOS {
	case "linux":
		dest := "/usr/local/share/ca-certificates/sabdopalon-rootCA.crt"
		_ = os.Remove(dest)
		_, err := exec.Command("update-ca-certificates", "--fresh").CombinedOutput()
		return err
	case "darwin":
		_, err := exec.Command("security", "delete-certificate", "-c", "Sabdopalon Local Root CA").CombinedOutput()
		return err
	case "windows":
		_, err := exec.Command("certutil", "-delstore", "Root", "Sabdopalon Local Root CA").CombinedOutput()
		return err
	}
	return nil
}

// IsTrusted checks whether the CA appears to be installed.
func IsTrusted() bool {
	switch runtime.GOOS {
	case "linux":
		return fileExists("/usr/local/share/ca-certificates/sabdopalon-rootCA.crt")
	case "darwin":
		out, err := exec.Command("security", "find-certificate", "-c", "Sabdopalon Local Root CA").CombinedOutput()
		return err == nil && len(out) > 0
	case "windows":
		out, err := exec.Command("certutil", "-store", "Root", "Sabdopalon Local Root CA").CombinedOutput()
		return err == nil && len(out) > 0
	}
	return false
}

func installLinux(caCert string) (bool, error) {
	dest := "/usr/local/share/ca-certificates/sabdopalon-rootCA.crt"
	if err := copyFile(caCert, dest); err != nil {
		// Permission denied → user needs sudo
		if os.IsPermission(err) {
			fmt.Printf("  ⚠  Permission denied. Run with sudo:\n")
			fmt.Printf("     sudo cp %s %s\n", caCert, dest)
			fmt.Printf("     sudo update-ca-certificates\n")
			return false, nil
		}
		return false, err
	}
	out, err := exec.Command("update-ca-certificates").CombinedOutput()
	if err != nil {
		fmt.Printf("  ⚠  update-ca-certificates failed: %s\n", string(out))
		fmt.Printf("     Run manually: sudo update-ca-certificates\n")
		return false, nil
	}
	return true, nil
}

func installMacOS(caCert string) (bool, error) {
	out, err := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
		"-k", "/Library/Keychains/System.keychain", caCert).CombinedOutput()
	if err != nil {
		fmt.Printf("  ⚠  Failed (may need sudo): %s\n", string(out))
		return false, nil
	}
	return true, nil
}

func installWindows(caCert string) (bool, error) {
	out, err := exec.Command("certutil", "-addstore", "-f", "Root", caCert).CombinedOutput()
	if err != nil {
		fmt.Printf("  ⚠  Failed (may need admin): %s\n", string(out))
		return false, nil
	}
	return true, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
