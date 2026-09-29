package certificate

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestCertificateAuthority(t *testing.T) {
	// Paths to the real CA files in the workspace (we are running tests from internal/domain/certificate)
	certPath := filepath.Join("..", "..", "..", "..", "..", "certs", "root-ca.pem")
	keyPath := filepath.Join("..", "..", "..", "..", "..", "certs", "root-ca.key")

	// certs/ is gitignored, so these are absent in CI and on fresh clones. Skip
	// rather than fail: the assertions below only make sense against a real CA.
	if _, err := os.Stat(certPath); err != nil {
		t.Skipf("CA cert not available at %s (run sdk_tests/ci/gen_certs.sh to generate): %v", certPath, err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Skipf("CA key not available at %s (run sdk_tests/ci/gen_certs.sh to generate): %v", keyPath, err)
	}

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

	_, err = x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key as EC: %v", err)
	}
}
