package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	hostFlag := flag.String("host", "127.0.0.1", "comma-separated IPs/hostnames for the server certificate SANs")
	flag.Parse()

	var ips []net.IP
	dnsNames := []string{"localhost"}
	commonName := ""
	for _, h := range strings.Split(*hostFlag, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if commonName == "" {
			commonName = h
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		} else if h != "localhost" {
			dnsNames = append(dnsNames, h)
		}
	}
	if commonName == "" {
		log.Fatal("-host must contain at least one IP or hostname")
	}

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
			CommonName:   commonName,
			Organization: []string{"Argus IoT"},
		},
		NotBefore:   notBefore,
		NotAfter:    notBefore.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: ips,
		DNSNames:    dnsNames,
	}

	serverBytes, err := x509.CreateCertificate(rand.Reader, &serverTemplate, &caTemplate, &serverPriv.PublicKey, caPriv)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Save files
	if err := os.MkdirAll("certs", 0o755); err != nil {
		log.Fatal(err)
	}
	savePEM("certs/ca.pem", "CERTIFICATE", caBytes, 0o644)
	
	f, err := os.Create("certs/fullchain.pem")
	if err != nil {
		log.Fatal(err)
	}
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: serverBytes})
	pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	f.Close()
	
	savePEM("certs/privkey.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverPriv), 0o600)

	log.Println("Generated RSA Root CA and Server Cert with 1h clock-drift buffer.")
}

func savePEM(filename, blockType string, bytes []byte, perm os.FileMode) {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	// OpenFile only applies perm on creation; tighten a pre-existing file too.
	if err := os.Chmod(filename, perm); err != nil {
		log.Fatal(err)
	}
	pem.Encode(f, &pem.Block{Type: blockType, Bytes: bytes})
}
