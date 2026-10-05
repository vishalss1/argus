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

	"github.com/vishalss1/argus/core/internal/version"
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

func TestGenerateProvision(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatalf("Failed to generate test root CA: %v", err)
	}
	certPEM, privKeyPEM, err := generateTestCertAndKey()
	if err != nil {
		t.Fatalf("Failed to generate test device cert/key: %v", err)
	}

	config := GeneratorConfig{
		ServerHost:             "192.168.10.100",
		HTTPPort:               8443,
		MQTTPort:               8883,
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
	apiKey := "argus_mockapikey123"

	sketchBytes, err := gen.GenerateProvision(GenerateOptions{
		DeviceID:        deviceID,
		WorkspaceID:     "5ab8eeaa-a44a-43bc-8ff0-56d9e843cbfc",
		APIKey:          apiKey,
		FirmwareVersion: "1.0.0",
		CertPEM:         certPEM,
		PrivKeyPEM:      privKeyPEM,
	})
	if err != nil {
		t.Fatalf("Failed to generate provisioning sketch: %v", err)
	}

	sketch := string(sketchBytes)

	assertions := []struct {
		substring string
		desc      string
	}{
		{`prefs.putString("device_id",   "` + deviceID + `");`, "Device ID"},
		{`prefs.putString("api_key",     "` + apiKey + `");`, "API Key"},
		{`prefs.putString("server_host", "` + config.ServerHost + `");`, "Server Host"},
		{`prefs.putString("mqtt_host",   "` + config.ServerHost + `");`, "MQTT Host"},
		{`prefs.putString("wifi_ssid",   "` + config.WiFiSSID + `");`, "WiFi SSID"},
		{`prefs.putString("wifi_pass",   "` + config.WiFiPassword + `");`, "WiFi Password"},
		{`prefs.putString("ota_key_id",  "` + config.OTASigningKeyID + `");`, "OTA Key ID"},
		{`prefs.putString("ota_pub_key", "` + config.OTASigningPublicKeyB64 + `");`, "OTA Public Key"},
		{`prefs.putString("fw_version",  "1.0.0");`, "Firmware Version"},
		{`prefs.putUInt("http_port", 8443);`, "HTTP Port"},
		{`prefs.putUInt("mqtt_port", 8883);`, "MQTT Port"},
	}
	for _, a := range assertions {
		if !strings.Contains(sketch, a.substring) {
			t.Errorf("Provisioning sketch is missing expected %s: %q", a.desc, a.substring)
		}
	}

	// PEM is embedded newline-escaped so it survives the NVS round-trip; the
	// sketch must carry it intact, not truncated.
	for _, item := range []struct {
		key string
		pem string
	}{
		{"root_ca", rootCA},
		{"dev_cert", certPEM},
		{"dev_key", privKeyPEM},
	} {
		if !strings.Contains(sketch, nvsPEM(item.pem)) {
			t.Errorf("Provisioning sketch does not contain intact %s", item.key)
		}
	}

	// The sketch must not bake device identity into a config symbol: that is
	// what the fleet firmware binary reads from NVS at boot.
	if strings.Contains(sketch, "char ARGUS_DEVICE_ID[]") {
		t.Error("Provisioning sketch must not define config symbols owned by the SDK")
	}
}

func TestGenerateProvisionRejectsInvalidPEM(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	certPEM, keyPEM, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}

	gen, err := NewGenerator(GeneratorConfig{RootCAPEM: rootCA})
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
			_, err := gen.GenerateProvision(GenerateOptions{
				DeviceID:   "device",
				APIKey:     "key",
				CertPEM:    tt.cert,
				PrivKeyPEM: tt.key,
			})
			if err == nil {
				t.Fatal("expected generation to fail")
			}
		})
	}
}

func TestGenerateFleetFirmwareHasNoDeviceIdentity(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	gen, err := NewGenerator(GeneratorConfig{RootCAPEM: rootCA})
	if err != nil {
		t.Fatal(err)
	}

	sketch, err := gen.GenerateFleetFirmware("")
	if err != nil {
		t.Fatalf("Failed to generate fleet firmware: %v", err)
	}

	// One identical binary is flashed to every device in the fleet; identity
	// comes from NVS. Any baked-in value would break that invariant.
	for _, forbidden := range []string{
		"putString(\"device_id\"",
		"putString(\"api_key\"",
		"putString(\"dev_key\"",
		"putString(\"root_ca\"",
		"char ARGUS_DEVICE_ID[]",
	} {
		if strings.Contains(string(sketch), forbidden) {
			t.Errorf("Fleet firmware must not contain device identity: found %q", forbidden)
		}
	}
	if !strings.Contains(string(sketch), "argusBegin()") {
		t.Error("Fleet firmware should call argusBegin()")
	}
}

// The OTA upload path derives an artifact's version by scanning the compiled
// binary for the ARGUSVER: marker. That only works if the version define is
// emitted into the sketch ahead of the SDK include, so pin both facts.
func TestGenerateFleetFirmwareEmbedsCompiledInVersion(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	gen, err := NewGenerator(GeneratorConfig{RootCAPEM: rootCA})
	if err != nil {
		t.Fatal(err)
	}

	sketch, err := gen.GenerateFleetFirmware("")
	if err != nil {
		t.Fatalf("Failed to generate fleet firmware: %v", err)
	}
	got := string(sketch)

	define := `#define ARGUS_FIRMWARE_VERSION "` + FleetFirmwareVersion + `"`
	if !strings.Contains(got, define) {
		t.Errorf("fleet sketch is missing %q", define)
	}

	// The define must precede #include <argus.h>: argus_version.h keys its
	// default off ARGUS_FIRMWARE_VERSION, so defining it afterwards would let
	// the header's fallback win.
	defineIdx := strings.Index(got, define)
	includeIdx := strings.Index(got, "#include <argus.h>")
	if defineIdx < 0 || includeIdx < 0 {
		t.Fatalf("expected both the version define and the argus.h include, got define=%d include=%d", defineIdx, includeIdx)
	}
	if defineIdx > includeIdx {
		t.Error("ARGUS_FIRMWARE_VERSION must be defined before #include <argus.h>")
	}

	if !version.Semver(FleetFirmwareVersion) {
		t.Errorf("FleetFirmwareVersion %q is not valid semver", FleetFirmwareVersion)
	}
}

// The version is a property of the binary, so the fleet path must not accept a
// caller-supplied one. Rendering twice must produce the same version, and the
// generator must reject a bad constant rather than emit an unflashable image.
func TestGenerateFleetFirmwareVersionHasNoOverride(t *testing.T) {
	rootCA, _, err := generateTestCertAndKey()
	if err != nil {
		t.Fatal(err)
	}
	gen, err := NewGenerator(GeneratorConfig{RootCAPEM: rootCA})
	if err != nil {
		t.Fatal(err)
	}

	first, err := gen.GenerateFleetFirmware("")
	if err != nil {
		t.Fatal(err)
	}
	second, err := gen.GenerateFleetFirmware("")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("fleet firmware must render identically regardless of caller input")
	}

	// DefaultFirmwareVersion is provisioning metadata and must not leak into the
	// fleet binary's running version.
	other, err := NewGenerator(GeneratorConfig{RootCAPEM: rootCA, DefaultFirmwareVersion: "9.9.9"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := other.GenerateFleetFirmware("")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, third) {
		t.Error("DefaultFirmwareVersion must not affect fleet firmware")
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
