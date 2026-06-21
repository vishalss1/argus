package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

type CertificateAuthority struct {
	caCert *x509.Certificate
	caKey  any
}

func NewCertificateAuthority(certPath, keyPath string) (*CertificateAuthority, error) {
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert file from %s: %w", certPath, err)
	}

	keyBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA key file from %s: %w", keyPath, err)
	}

	certBlock, _ := pem.Decode(certBytes)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("failed to decode PEM CA cert")
	}

	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CA cert: %w", err)
	}

	keyBlock, _ := pem.Decode(keyBytes)
	if keyBlock == nil {
		return nil, fmt.Errorf("failed to decode PEM CA private key")
	}

	var caKey any
	// Parse the key. Try PKCS#8 first, then EC, then PKCS#1
	if caKey, err = x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err != nil {
		if caKey, err = x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
			if caKey, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes); err != nil {
				return nil, fmt.Errorf("failed to parse CA private key: %w", err)
			}
		}
	}

	return &CertificateAuthority{
		caCert: caCert,
		caKey:  caKey,
	}, nil
}

func (ca *CertificateAuthority) IssueDeviceCertificate(deviceID string, workspaceID string) (*IssuedCertificate, error) {
	// Generate ECDSA P-256 keypair
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA key: %w", err)
	}

	// SerialNumber: Random 128-bit integer
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNum, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: serialNum,
		Subject: pkix.Name{
			CommonName:         fmt.Sprintf("device:%s", deviceID),
			OrganizationalUnit: []string{fmt.Sprintf("workspace:%s", workspaceID)},
			Organization:       []string{"Argus IoT"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10-year validity
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// Sign with Root CA
	derBytes, err := x509.CreateCertificate(rand.Reader, template, ca.caCert, &privKey.PublicKey, ca.caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// PEM encode certificate
	certPEMBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}
	certPEM := pem.EncodeToMemory(certPEMBlock)

	// SEC1 marshal private key
	privKeyDer, err := x509.MarshalECPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EC private key: %w", err)
	}

	privPEMBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: privKeyDer,
	}
	privPEM := pem.EncodeToMemory(privPEMBlock)

	return &IssuedCertificate{
		CertPEM:       string(certPEM),
		PrivateKeyPEM: string(privPEM),
	}, nil
}
