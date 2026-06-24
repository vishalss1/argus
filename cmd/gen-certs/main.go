package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	// 1. Generate Root CA (RSA 2048 for maximum compatibility)
	caPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	// NotBefore set to 1 hour ago to handle clock drift
	notBefore := time.Now().Add(-1 * time.Hour)

	caTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Argus Root CA",
			Organization: []string{"Argus IoT"},
		},
		NotBefore:             notBefore,
		NotAfter:              notBefore.Add(10 * 365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caPriv.PublicKey, caPriv)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Generate Server Certificate
	serverPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			CommonName:   "192.168.29.222",
			Organization: []string{"Argus IoT"},
		},
		NotBefore:   notBefore,
		NotAfter:    notBefore.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.29.222")},
		DNSNames:    []string{"localhost"},
	}

	serverBytes, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverPriv.PublicKey, caPriv)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Save files
	if err := os.MkdirAll("certs", 0o755); err != nil {
		log.Fatal(err)
	}
	savePEM("certs/ca.pem", "CERTIFICATE", caBytes)
	
	f, err := os.Create("certs/fullchain.pem")
	if err != nil {
		log.Fatal(err)
	}
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: serverBytes})
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	f.Close()
	
	savePEM("certs/privkey.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverPriv))

	log.Println("Generated RSA Root CA and Server Cert with 1h clock-drift buffer.")
}

func savePEM(filename, blockType string, bytes []byte) {
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: blockType, Bytes: bytes})
}
