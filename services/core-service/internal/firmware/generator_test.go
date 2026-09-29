package firmware

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func generateTestCertAndKey() (string, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return "", "", err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	return string(certPEM), string(keyPEM), nil
}

func TestFirmwareGenerator(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatalf("Failed to generate test root CA: %v", err)
	}

	config := GeneratorConfig{
		ServerHost:             "192.168.10.100",
		HTTPPort:               8443,
		RootCAPEM:              rootCA,
		WiFiSSID:               "TestSSID",
		WiFiPassword:           "TestPassword",
		OTASigningKeyID:        "ota-test-v1",
		OTASigningPublicKeyB64: "OTAPUBLICKEYMOCK==",
	}

	gen, err := NewGenerator(config)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	deviceID := "75596200-a44a-43bc-8ff0-56d9e843cbfc"
	workspaceID := "5ab8eeaa-a44a-43bc-8ff0-56d9e843cbfc"
	apiKey := "argus_mockapikey123"

	certPEM, privKeyPEM, err := generateTestCertAndKey()
	if err != nil {
		t.Fatalf("Failed to generate test device cert/key: %v", err)
	}

	sketchBytes, err := gen.Generate(deviceID, workspaceID, apiKey, "1.0.0", certPEM, privKeyPEM)
	if err != nil {
		t.Fatalf("Failed to generate sketch: %v", err)
	}

	sketch := string(sketchBytes)

	// Verify that placeholders were replaced correctly
	assertions := []struct {
		substring string
		desc      string
	}{
		{`char ARGUS_DEVICE_ID[] = "` + deviceID + `"`, "Device ID"},
		{`char ARGUS_API_KEY[] = "` + apiKey + `"`, "API Key"},
		{`char ARGUS_SERVER_HOST[] = "` + config.ServerHost + `"`, "Server Host"},
		{`uint16_t ARGUS_HTTP_PORT = 8443;`, "HTTP Port"},
		{`char ARGUS_FW_VERSION[] = "1.0.0"`, "Firmware Version"},
		{`char ARGUS_OTA_KEY_ID[] = "` + config.OTASigningKeyID + `"`, "OTA Key ID"},
		{`char ARGUS_OTA_PUBLIC_KEY_B64[] = "` + config.OTASigningPublicKeyB64 + `"`, "OTA Public Key"},
		{`char WIFI_SSID[] = "` + config.WiFiSSID + `"`, "WiFi SSID"},
		{`char WIFI_PASSWORD[] = "` + config.WiFiPassword + `"`, "WiFi Password"},
		{config.RootCAPEM, "Root CA PEM"},
		{certPEM, "Device Cert PEM"},
		{privKeyPEM, "Device Private Key PEM"},
	}

	for _, a := range assertions {
		if !strings.Contains(sketch, a.substring) {
			t.Errorf("Sketch is missing expected %s: %q", a.desc, a.substring)
		}
	}

	for _, item := range []struct {
		symbol string
		pem    string
	}{
		{"ARGUS_ROOT_CA", rootCA},
		{"ARGUS_DEVICE_CERT", certPEM},
		{"ARGUS_DEVICE_PRIVATE_KEY", privKeyPEM},
	} {
		expected := item.pem
		if !strings.HasSuffix(expected, "\n") {
			expected += "\n"
		}
		if err := validateRenderedPEM(sketch, item.symbol, expected); err != nil {
			t.Errorf("%s was not embedded byte-for-byte: %v", item.symbol, err)
		}
	}
}

func TestFirmwareGeneratorRejectsInvalidPEM(t *testing.T) {
	certPEM, keyPEM, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}

	gen, err := NewGenerator(GeneratorConfig{RootCAPEM: certPEM})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		cert string
		key  string
	}{
		{"truncated certificate", strings.TrimSuffix(certPEM, "-----END CERTIFICATE-----\n"), keyPEM},
		{"truncated private key", certPEM, strings.TrimSuffix(keyPEM, "-----END PRIVATE KEY-----\n")},
		{"wrong private key block type", certPEM, certPEM},
		{"trailing private key data", certPEM, keyPEM + "corruption"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := gen.Generate("device", "workspace", "key", "1.0.0", tt.cert, tt.key); err == nil {
				t.Fatal("expected generation to fail")
			}
		})
	}
}

func TestPreparePEMPreservesExistingBytes(t *testing.T) {
	certPEM, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := preparePEM("certificate", certPEM, "CERTIFICATE")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal([]byte(prepared), []byte(certPEM)) {
		t.Fatal("PEM bytes changed")
	}
}
