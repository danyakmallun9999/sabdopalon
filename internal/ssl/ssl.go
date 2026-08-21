// Package ssl manages a local root CA and per-site certificates so that
// pretty URLs (https://name.test) work in the browser without warnings.
//
// Approach mirrors mkcert: generate one local root CA, install it into the
// OS trust store, then issue per-site leaf certs signed by that CA.
//
// v0.1 is a stub that documents the plan and provides an Ensure API that
// reports "not yet implemented" so the CLI can call it safely. Full crypto
// wiring lands in v0.2 using Go's crypto/x509.
package ssl

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/sabdopalon/sabdopalon/internal/config"
)

// Manager handles cert generation under certs/.
type Manager struct {
	cfg *config.Engine
}

// New creates an ssl Manager.
func New(cfg *config.Engine) *Manager {
	return &Manager{cfg: cfg}
}

// CertPaths returns the expected cert and key paths for a host.
func (m *Manager) CertPaths(host string) (cert, key string) {
	return filepath.Join(m.certDir(), host+".crt"),
		filepath.Join(m.certDir(), host+".key")
}

// EnsureCA generates the root CA if missing. Returns whether it was created.
func (m *Manager) EnsureCA() (created bool, err error) {
	caCert, caKey := m.caPaths()
	if fileExists(caCert) && fileExists(caKey) {
		return false, nil
	}
	if err := os.MkdirAll(m.certDir(), 0o755); err != nil {
		return false, err
	}
	if err := generateCA(caCert, caKey); err != nil {
		return false, fmt.Errorf("generate root CA: %w", err)
	}
	fmt.Printf("  •  root CA created: %s\n", caCert)
	fmt.Printf("  !  trust it manually for now (auto-install planned v0.2)\n")
	return true, nil
}

// IssueSite generates a leaf certificate for host signed by the local CA.
func (m *Manager) IssueSite(host string) error {
	caCertPath, caKeyPath := m.caPaths()
	if !fileExists(caCertPath) || !fileExists(caKeyPath) {
		if _, err := m.EnsureCA(); err != nil {
			return err
		}
	}
	certPath, keyPath := m.CertPaths(host)
	return generateLeaf(host, certPath, keyPath, caCertPath, caKeyPath)
}

func (m *Manager) certDir() string { return filepath.Join(m.cfg.RootDir, "certs") }

func (m *Manager) caPaths() (string, string) {
	return filepath.Join(m.certDir(), "sabdopalon-rootCA.crt"),
		filepath.Join(m.certDir(), "sabdopalon-rootCA.key")
}

// --- crypto helpers (std-lib only) ---

func generateCA(certPath, keyPath string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "Sabdopalon Local Root CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := writePEM(certPath, "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEMKey(keyPath, key)
}

func generateLeaf(host, certPath, keyPath, caCertPath, caKeyPath string) error {
	// load CA
	caDer, err := readPEM(caCertPath, "CERTIFICATE")
	if err != nil {
		return err
	}
	caCert, err := x509.ParseCertificate(caDer)
	if err != nil {
		return err
	}
	caKeyDer, err := readPEM(caKeyPath, "RSA PRIVATE KEY")
	if err != nil {
		return err
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyDer)
	if err != nil {
		return err
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: host},
		DNSNames:     []string{host},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(2, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := writePEM(certPath, "CERTIFICATE", der); err != nil {
		return err
	}
	return writePEMKey(keyPath, leafKey)
}

func writePEM(path, blockType string, der []byte) error {
	return os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o644)
}

func writePEMKey(path string, key *rsa.PrivateKey) error {
	return os.WriteFile(path,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0o600)
}

func readPEM(path, wantType string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != wantType {
		return nil, fmt.Errorf("unexpected PEM type in %s", path)
	}
	return block.Bytes, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
