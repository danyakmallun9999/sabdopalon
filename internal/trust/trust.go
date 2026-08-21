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
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

// Status describes whether the Sabdopalon root CA is installed in the OS
// trust store and whether it still matches the local certs/sabdopalon-rootCA.crt.
type Status struct {
	CAExists     bool   `json:"ca_exists"`
	Installed    bool   `json:"installed"`
	FingerMatch  bool   `json:"fingerprint_match"` // false = stale trust after CA regeneration
	WildcardCert bool   `json:"wildcard_cert"`
	Detail       string `json:"detail,omitempty"`
}

// CheckStatus inspects local PKI state and (where possible) compares the
// fingerprint of the trusted CA against the local one. A mismatch means the
// CA was regenerated after being installed — every HTTPS request will be
// rejected until it is re-trusted.
func CheckStatus(cfg *config.Engine) Status {
	st := Status{}
	caPath := filepath.Join(cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	localPEM, err := os.ReadFile(caPath)
	if err != nil {
		st.Detail = "no root CA generated yet — start at step 1"
		return st
	}
	st.CAExists = true

	wildcard := filepath.Join(cfg.RootDir, "certs", "*."+cfg.TLD+".crt")
	if _, err := os.Stat(wildcard); err == nil {
		st.WildcardCert = true
	}

	switch runtime.GOOS {
	case "linux":
		sysPEM, err := os.ReadFile("/usr/local/share/ca-certificates/sabdopalon-rootCA.crt")
		if err != nil {
			st.Detail = "not installed in the OS trust store"
			return st
		}
		st.Installed = true
		st.FingerMatch = sha256Equal(localPEM, sysPEM)
	case "darwin":
		out, err := exec.Command("security", "find-certificate", "-c", "Sabdopalon Local Root CA", "-p").Output()
		if err != nil || len(out) == 0 {
			st.Detail = "not installed in the macOS keychain"
			return st
		}
		st.Installed = true
		st.FingerMatch = sha256Equal(localPEM, out)
	default: // windows and others: presence check only
		st.Installed = IsTrusted()
		st.FingerMatch = st.Installed
	}

	if st.Installed && !st.FingerMatch {
		st.Detail = "an OLDER Sabdopalon CA is still trusted — run Trust CA again"
	}
	return st
}

func sha256Equal(a, b []byte) bool {
	ha := sha256.Sum256(bytes.TrimSpace(a))
	hb := sha256.Sum256(bytes.TrimSpace(b))
	return ha == hb
}

// InstallCAQuiet installs the CA like InstallCA but without console output,
// for use by the dashboard API. Returns ok=false when elevation is required.
func InstallCAQuiet(cfg *config.Engine) (bool, error) {
	caCert := filepath.Join(cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	if !fileExists(caCert) {
		return false, fmt.Errorf("root CA not found — generate it first")
	}
	var ok bool
	var err error
	switch runtime.GOOS {
	case "linux":
		ok, err = installLinuxQuiet(caCert)
	case "darwin":
		_, err = exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
			"-k", "/Library/Keychains/System.keychain", caCert).CombinedOutput()
		ok = err == nil
	case "windows":
		_, err = exec.Command("certutil", "-addstore", "-f", "Root", caCert).CombinedOutput()
		ok = err == nil
	}
	return ok, err
}

// ManualCommand returns the exact terminal command a user should run when
// automatic trust installation fails due to missing privileges.
func ManualCommand(cfg *config.Engine) string {
	caCert := filepath.Join(cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	switch runtime.GOOS {
	case "linux":
		return fmt.Sprintf("sudo cp %s /usr/local/share/ca-certificates/sabdopalon-rootCA.crt && sudo update-ca-certificates", caCert)
	case "darwin":
		return fmt.Sprintf("sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain %s", caCert)
	case "windows":
		return "Run this app as Administrator once, then press Trust CA."
	default:
		return ""
	}
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

// installLinuxQuiet is the console-free variant used by the dashboard.
func installLinuxQuiet(caCert string) (bool, error) {
	dest := "/usr/local/share/ca-certificates/sabdopalon-rootCA.crt"
	if err := copyFile(caCert, dest); err != nil {
		if os.IsPermission(err) {
			return false, nil // needs sudo — caller shows manual command
		}
		return false, err
	}
	if out, err := exec.Command("update-ca-certificates").CombinedOutput(); err != nil {
		return false, fmt.Errorf("update-ca-certificates: %s", strings.TrimSpace(string(out)))
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
