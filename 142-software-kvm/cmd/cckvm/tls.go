package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Barrier's TLS model: the server uses a self-signed certificate and the
// client pins its SHA-256 fingerprint. This follows it. The fingerprint is
// printed as "SHA256:AA:BB:...", the form Barrier 2.4 shows.

func fingerprintOf(der []byte) string {
	sum := sha256.Sum256(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return "SHA256:" + strings.Join(parts, ":")
}

func normalizeFingerprint(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "SHA256:")
	return strings.NewReplacer(":", "", " ", "").Replace(s)
}

func defaultCertPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cckvm", "server.pem"), nil
}

// serverTLS loads the certificate at path, generating a self-signed one
// first if it does not exist.
func serverTLS(path string) (*tls.Config, string, error) {
	if path == "" {
		p, err := defaultCertPath()
		if err != nil {
			return nil, "", err
		}
		path = p
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := generateCert(path); err != nil {
			return nil, "", fmt.Errorf("generating certificate: %w", err)
		}
	}
	cert, err := tls.LoadX509KeyPair(path, path)
	if err != nil {
		return nil, "", fmt.Errorf("loading %s: %w", path, err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		fingerprintOf(cert.Certificate[0]), nil
}

func generateCert(path string) error {
	// RSA rather than ECDSA because that is what Barrier itself generates,
	// so every OpenSSL build a Barrier client might use accepts it.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "cckvm"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}
	return pem.Encode(f, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
}

func tlsListener(ln net.Listener, cfg *tls.Config) net.Listener { return tls.NewListener(ln, cfg) }

// clientTLS returns a dialer that accepts only the pinned certificate. A
// missing fingerprint is an error: connecting anyway would let anyone on
// the network read every keystroke.
func clientTLS(fingerprint string) (func(context.Context, string) (net.Conn, error), error) {
	want := normalizeFingerprint(fingerprint)
	if want == "" {
		return nil, errors.New("--tls needs --fingerprint (run \"cckvm fingerprint\" on the server)")
	}
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		// The chain is self-signed, so the usual verification cannot work;
		// VerifyPeerCertificate does the pinning instead.
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return errors.New("server sent no certificate")
			}
			got := fingerprintOf(raw[0])
			if subtle.ConstantTimeCompare([]byte(normalizeFingerprint(got)), []byte(want)) != 1 {
				return fmt.Errorf("server certificate fingerprint %s does not match the pinned one", got)
			}
			return nil
		},
	}
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := tls.Dialer{Config: cfg}
		return d.DialContext(ctx, "tcp", addr)
	}, nil
}
