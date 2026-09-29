package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestCA writes a self-signed root CA into a temp dir and returns the cert
// and key paths. Generating the CA in-test keeps this hermetic: the repo's
// certs/ directory is gitignored, so relying on it made the test skip in CI.
func newTestCA(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	dir := t.TempDir()
	certPath = filepath.Join(dir, "root-ca.pem")
	keyPath = filepath.Join(dir, "root-ca.key")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         "Argus Test Root CA",
			OrganizationalUnit: []string{"Argus Test"},
			Organization:       []string{"Argus IoT"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal CA key: %v", err)
	}

	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write CA cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatalf("write CA key: %v", err)
	}

	return certPath, keyPath
}

func TestCertificateAuthority(t *testing.T) {
	certPath, keyPath := newTestCA(t)

	ca, err := NewCertificateAuthority(certPath, keyPath)
	if err != nil {
		t.Fatalf("Failed to initialize CA: %v", err)
	}

	deviceID := "75596200-a44a-43bc-8ff0-56d9e843cbfc"
	workspaceID := "5ab8eeaa-a44a-43bc-8ff0-56d9e843cbfc"

	issued, err := ca.IssueDeviceCertificate(deviceID, workspaceID)
	if err != nil {
		t.Fatalf("Failed to issue device certificate: %v", err)
	}

	if issued.CertPEM == "" {
		t.Error("Issued certificate PEM is empty")
	}
	if issued.PrivateKeyPEM == "" {
		t.Error("Issued private key PEM is empty")
	}

	// Verify Certificate Structure
	certBlock, _ := pem.Decode([]byte(issued.CertPEM))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatal("Failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse X.509 certificate: %v", err)
	}

	expectedCN := "device:" + deviceID
	if cert.Subject.CommonName != expectedCN {
		t.Errorf("Expected CN=%q, got %q", expectedCN, cert.Subject.CommonName)
	}

	expectedOU := "workspace:" + workspaceID
	if len(cert.Subject.OrganizationalUnit) != 1 || cert.Subject.OrganizationalUnit[0] != expectedOU {
		t.Errorf("Expected OU=%v, got %v", []string{expectedOU}, cert.Subject.OrganizationalUnit)
	}

	if len(cert.Subject.Organization) != 1 || cert.Subject.Organization[0] != "Argus IoT" {
		t.Errorf("Expected O=%v, got %v", []string{"Argus IoT"}, cert.Subject.Organization)
	}

	if cert.IsCA {
		t.Error("Expected IsCA=false, got true")
	}

	var hasClientAuth bool
	for _, usage := range cert.ExtKeyUsage {
		if usage == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
			break
		}
	}
	if !hasClientAuth {
		t.Error("Expected ExtKeyUsage to contain ClientAuth")
	}

	// Verify Private Key Structure
	keyBlock, _ := pem.Decode([]byte(issued.PrivateKeyPEM))
	if keyBlock == nil || keyBlock.Type != "EC PRIVATE KEY" {
		t.Fatal("Failed to decode PEM private key (expected 'EC PRIVATE KEY')")
	}

	deviceKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key as EC: %v", err)
	}

	// The issued private key must belong to the issued certificate.
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("Expected ECDSA public key, got %T", cert.PublicKey)
	}
	if !pub.Equal(deviceKey.Public()) {
		t.Error("Issued private key does not match issued certificate public key")
	}

	// The issued certificate must verify against the CA that signed it.
	roots := x509.NewCertPool()
	rootPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("Failed to re-read CA cert: %v", err)
	}
	if !roots.AppendCertsFromPEM(rootPEM) {
		t.Fatal("Failed to load CA cert into pool")
	}
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Errorf("Issued certificate does not verify against CA: %v", err)
	}
}

func TestCertificateAuthorityRejectsInvalidInput(t *testing.T) {
	dir := t.TempDir()
	goodCert, goodKey := newTestCA(t)

	goodCertPEM, err := os.ReadFile(goodCert)
	if err != nil {
		t.Fatalf("read CA cert: %v", err)
	}
	goodKeyPEM, err := os.ReadFile(goodKey)
	if err != nil {
		t.Fatalf("read CA key: %v", err)
	}

	// A second, unrelated CA: pairing its key with the first CA's cert must be
	// rejected, since the resulting authority could never sign anything.
	_, otherKey := newTestCA(t)
	otherKeyPEM, err := os.ReadFile(otherKey)
	if err != nil {
		t.Fatalf("read other CA key: %v", err)
	}

	cases := []struct {
		name    string
		cert    []byte
		key     []byte
		wantErr string
	}{
		{"missing cert", nil, goodKeyPEM, "failed to read CA cert file"},
		{"missing key", goodCertPEM, nil, "failed to read CA key file"},
		{"cert is not PEM", []byte("not a pem file"), goodKeyPEM, "failed to decode PEM CA cert"},
		{"key is not PEM", goodCertPEM, []byte("not a pem file"), "failed to decode PEM CA private key"},
		{"key does not match cert", goodCertPEM, otherKeyPEM, "does not match"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			certPath := filepath.Join(dir, tc.name+"-cert.pem")
			keyPath := filepath.Join(dir, tc.name+"-key.pem")
			if tc.cert != nil {
				if err := os.WriteFile(certPath, tc.cert, 0o600); err != nil {
					t.Fatalf("write cert: %v", err)
				}
			}
			if tc.key != nil {
				if err := os.WriteFile(keyPath, tc.key, 0o600); err != nil {
					t.Fatalf("write key: %v", err)
				}
			}

			_, err := NewCertificateAuthority(certPath, keyPath)
			if err == nil {
				t.Fatalf("Expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}
