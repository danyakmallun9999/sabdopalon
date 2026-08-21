// Package trust installs the Sabdopalon root CA so browsers accept the
// locally-issued HTTPS certificates without warnings.
//
// Two installation tiers per platform:
//
//	system-wide (preferred)          per-user fallback (NO sudo/admin)
//	────────────────────────         ─────────────────────────────────
//	Linux:  /usr/local/share/        NSS user DB (~/.pki/nssdb) for
//	        ca-certificates +        Chrome/Chromium/Edge, plus Firefox
//	        update-ca-certificates   profile databases (via certutil)
//	macOS:  System keychain (-d)     login keychain (no -d)
//	Windows: certutil -addstore      certutil -user -addstore Root
//	         Root (admin)
//
// If every automatic route fails, ManualCommand() returns the exact command.
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

const caNickname = "Sabdopalon Local Root CA"

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

// Status describes whether the Sabdopalon root CA is trusted and where.
type Status struct {
	CAExists     bool   `json:"ca_exists"`
	WildcardCert bool   `json:"wildcard_cert"`
	Installed    bool   `json:"installed"`
	FingerMatch  bool   `json:"fingerprint_match"` // false = stale trust after CA regeneration
	Source       string `json:"source,omitempty"`  // "system" or "user"
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
		if sysPEM, err := os.ReadFile("/usr/local/share/ca-certificates/sabdopalon-rootCA.crt"); err == nil {
			st.Installed = true
			st.Source = "system"
			st.FingerMatch = sha256Equal(localPEM, sysPEM)
			break
		}
		checkNSSStatus(&st, localPEM)
	case "darwin":
		if out, err := exec.Command("security", "find-certificate", "-c", caNickname, "-p").Output(); err == nil && len(out) > 0 {
			st.Installed = true
			st.Source = "system"
			st.FingerMatch = sha256Equal(localPEM, out)
			break
		}
		loginKeychain := filepath.Join(homeDir(), "Library", "Keychains", "login.keychain-db")
		if out, err := exec.Command("security", "find-certificate", "-c", caNickname, "-p", loginKeychain).Output(); err == nil && len(out) > 0 {
			st.Installed = true
			st.Source = "user"
			st.FingerMatch = sha256Equal(localPEM, out)
			break
		}
		st.Detail = "not installed in any keychain"
	default: // windows and others: presence checks only
		if out, err := exec.Command("certutil", "-store", "Root", caNickname).CombinedOutput(); err == nil && len(out) > 0 {
			st.Installed, st.Source, st.FingerMatch = true, "system", true
			break
		}
		if out, err := exec.Command("certutil", "-user", "-store", "Root", caNickname).CombinedOutput(); err == nil && len(out) > 0 {
			st.Installed, st.Source, st.FingerMatch = true, "user", true
			break
		}
		st.Detail = "not installed in the certificate store"
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

// InstallCAQuiet installs the CA, preferring the system store and falling
// back to per-user stores that need NO admin rights. Console-free (dashboard
// API). Returns ok=false when every automatic route failed — pair with
// ManualCommand() so the UI can show exact instructions.
func InstallCAQuiet(cfg *config.Engine) (bool, error) {
	caCert := filepath.Join(cfg.RootDir, "certs", "sabdopalon-rootCA.crt")
	if !fileExists(caCert) {
		return false, fmt.Errorf("root CA not found — generate it first")
	}

	// Tier 1: system-wide.
	switch runtime.GOOS {
	case "linux":
		if ok, err := installLinuxQuiet(caCert); ok {
			return true, err
		}
	case "darwin":
		if out, err := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
			"-k", "/Library/Keychains/System.keychain", caCert).CombinedOutput(); err == nil {
			_ = out
			return true, nil
		}
	case "windows":
		if out, err := exec.Command("certutil", "-addstore", "-f", "Root", caCert).CombinedOutput(); err == nil {
			_ = out
			return true, nil
		}
	}

	// Tier 2: per-user stores — no elevation required.
	switch runtime.GOOS {
	case "linux":
		return installLinuxUserNSS(caCert)
	case "darwin":
		loginKeychain := filepath.Join(homeDir(), "Library", "Keychains", "login.keychain-db")
		if out, err := exec.Command("security", "add-trusted-cert", "-r", "trustRoot",
			"-k", loginKeychain, caCert).CombinedOutput(); err == nil {
			_ = out
			return true, nil
		}
		return false, fmt.Errorf("login keychain import failed — approve the macOS prompt or use the manual command")
	case "windows":
		out, err := exec.Command("certutil", "-user", "-addstore", "-f", "Root", caCert).CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("certutil -user: %s", strings.TrimSpace(string(out)))
		}
		return true, nil
	}
	return false, nil
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

// homeDir returns the current user's home directory ("" if undetectable).
func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}

// --- Linux per-user NSS stores (Chrome/Chromium/Edge + Firefox) ---

// nssDB describes one NSS certificate database.
type nssDB struct {
	dir   string
	label string
}

// userNSSDBs lists candidate NSS databases for the current user.
func userNSSDBs() []nssDB {
	home := homeDir()
	if home == "" {
		return nil
	}
	var dbs []nssDB
	dbs = append(dbs, nssDB{dir: filepath.Join(home, ".pki", "nssdb"), label: "Chrome/Chromium"})
	patterns := []string{
		filepath.Join(home, ".mozilla", "firefox", "*", "cert9.db"),
		filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox", "*", "cert9.db"),
	}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		for _, m := range matches {
			dbs = append(dbs, nssDB{dir: filepath.Dir(m), label: "Firefox"})
		}
	}
	return dbs
}

// checkNSSStatus fills st from the per-user NSS databases when the CA is
// present there (system-wide install absent).
func checkNSSStatus(st *Status, localPEM []byte) {
	if _, err := exec.LookPath("certutil"); err != nil {
		st.Detail = "not installed in the OS trust store"
		return
	}
	for _, db := range userNSSDBs() {
		out, err := exec.Command("certutil", "-L", "-a", "-d", "sql:"+db.dir, "-n", caNickname).Output()
		if err != nil || len(out) == 0 {
			continue
		}
		st.Installed = true
		st.Source = "user"
		st.FingerMatch = sha256Equal(localPEM, out)
		return
	}
	st.Detail = "not installed in the OS trust store"
}

// installLinuxUserNSS imports the CA into every reachable per-user NSS store:
// Chrome/Chromium (~/.pki/nssdb, created when missing) and Firefox profiles.
// Requires the `certutil` tool (libnss3-tools); without it we report failure
// so the UI falls back to manual instructions.
func installLinuxUserNSS(caCert string) (bool, error) {
	if _, err := exec.LookPath("certutil"); err != nil {
		return false, fmt.Errorf(
			"per-user trust needs the NSS tool: install it once with 'sudo apt install libnss3-tools', then press Trust CA again")
	}
	imported := 0
	for _, db := range userNSSDBs() {
		if db.label != "Firefox" {
			// Ensure Chrome's DB exists; Firefox profile DBs always do.
			if _, err := os.Stat(filepath.Join(db.dir, "cert9.db")); os.IsNotExist(err) {
				_ = os.MkdirAll(db.dir, 0o700)
				cmd := exec.Command("certutil", "-N", "-d", "sql:"+db.dir, "--empty-password")
				if out, err := cmd.CombinedOutput(); err != nil {
					_ = out
					continue
				}
			}
		}
		cmd := exec.Command("certutil", "-A", "-d", "sql:"+db.dir,
			"-n", caNickname, "-t", "C,,", "-i", caCert)
		if out, err := cmd.CombinedOutput(); err == nil {
			_ = out
			imported++
		}
	}
	if imported == 0 {
		return false, fmt.Errorf("no NSS database could be updated")
	}
	return true, nil
}
