package firmware

import (
	"bytes"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/firmware.ino.tmpl templates/provision.ino.tmpl templates/fleet_firmware.ino.tmpl
var templatesFS embed.FS

type GeneratorConfig struct {
	ServerHost             string
	HTTPPort               int
	MQTTPort               int
	RootCAPEM              string
	WiFiSSID               string
	WiFiPassword           string
	OTASigningKeyID        string
	OTASigningPublicKeyB64 string
	DefaultFirmwareVersion string
}

type Generator struct {
	config GeneratorConfig
	tmpl   *template.Template
}

type TemplateData struct {
	DeviceID               string
	WorkspaceID            string
	APIKey                 string
	ServerHost             string
	HTTPPort               int
	MQTTPort               int
	WiFiSSID               string
	WiFiPassword           string
	OTASigningKeyID        string
	OTASigningPublicKeyB64 string
	FirmwareVersion        string
	RootCAPEM              string
	DeviceCertPEM          string
	DevicePrivateKeyPEM    string
	UserCode               string
}

type GenerateOptions struct {
	DeviceID        string
	WorkspaceID     string
	APIKey          string
	FirmwareVersion string
	CertPEM         string
	PrivKeyPEM      string
	WiFiSSID        string
	WiFiPassword    string
	UserCode        string
}

func validatePEM(name, pemStr, expectedType string) error {
	if pemStr == "" {
		return fmt.Errorf("%s is empty", name)
	}

	block, rest := pem.Decode([]byte(pemStr))
	if block == nil {
		return fmt.Errorf("%s: failed to decode PEM block", name)
	}
	
	if expectedType == "PRIVATE KEY" {
		if block.Type != "PRIVATE KEY" && block.Type != "EC PRIVATE KEY" {
			return fmt.Errorf("%s: expected PEM type \"PRIVATE KEY\" or \"EC PRIVATE KEY\", got %q", name, block.Type)
		}
	} else if block.Type != expectedType {
		return fmt.Errorf("%s: expected PEM type %q, got %q", name, expectedType, block.Type)
	}

	if len(bytes.TrimSpace(rest)) != 0 {
		return fmt.Errorf("%s: unexpected data after PEM block", name)
	}

	if block.Type == "PRIVATE KEY" {
		_, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("%s: failed to parse PKCS8 private key: %w", name, err)
		}
	} else if block.Type == "EC PRIVATE KEY" {
		_, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("%s: failed to parse EC private key: %w", name, err)
		}
	} else {
		_, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("%s: failed to parse X509 certificate: %w", name, err)
		}
	}
	return nil
}

func preparePEM(name, pemStr, expectedType string) (string, error) {
	if err := validatePEM(name, pemStr, expectedType); err != nil {
		return "", err
	}
	if strings.Contains(pemStr, ")EOF\"") {
		return "", fmt.Errorf("%s: contains reserved C++ raw-string delimiter", name)
	}
	if !strings.HasSuffix(pemStr, "\n") {
		pemStr += "\n"
	}
	return pemStr, nil
}

func cppString(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\r", `\r`,
		"\n", `\n`,
		"\t", `\t`,
	)
	return replacer.Replace(value)
}

// nvsPEM prepares a PEM string for storage in ESP32 NVS.
//
// NVS string entries are null-terminated C strings; raw '\n' bytes can cause
// truncation on some ESP-IDF versions.  We replace each newline with the
// two-character literal sequence '\''n' so the value survives the round-trip.
// The argus_nvs.cpp loader reverses this substitution when reading at runtime.
func nvsPEM(pem string) string {
	return strings.ReplaceAll(pem, "\n", `\n`)
}

func validateRenderedPEM(sketch, symbol, expected string) error {
	prefix := `const char ` + symbol + `[] PROGMEM = R"EOF(` + "\n"
	start := strings.Index(sketch, prefix)
	if start < 0 {
		return fmt.Errorf("%s: generated declaration not found", symbol)
	}
	start += len(prefix)
	end := strings.Index(sketch[start:], `)EOF";`)
	if end < 0 {
		return fmt.Errorf("%s: generated raw string is unterminated", symbol)
	}
	actual := sketch[start : start+end]
	if actual != expected {
		return fmt.Errorf("%s: generated PEM differs from validated input", symbol)
	}
	return nil
}

// ValidateCAIssuesServerCert checks that the server CA PEM can verify the
// server certificate PEM. This catches CA/serving-cert mismatches at firmware
// generation time rather than at runtime on the device.
func ValidateCAIssuesServerCert(caPEM, serverCertPEM string) error {
	caBlock, _ := pem.Decode([]byte(caPEM))
	if caBlock == nil {
		return fmt.Errorf("server CA: failed to decode PEM block")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return fmt.Errorf("server CA: failed to parse X.509 certificate: %w", err)
	}

	serverBlock, _ := pem.Decode([]byte(serverCertPEM))
	if serverBlock == nil {
		return fmt.Errorf("server certificate: failed to decode PEM block")
	}
	serverCert, err := x509.ParseCertificate(serverBlock.Bytes)
	if err != nil {
		return fmt.Errorf("server certificate: failed to parse X.509 certificate: %w", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(caCert)
	if _, err := serverCert.Verify(x509.VerifyOptions{Roots: roots}); err != nil {
		return fmt.Errorf("FATAL: firmware server CA (%s) does not verify the server certificate chain: %w",
			caCert.Subject.CommonName, err)
	}
	return nil
}

func NewGenerator(config GeneratorConfig) (*Generator, error) {
	rootCA, err := preparePEM("root CA", config.RootCAPEM, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("invalid root CA in config: %w", err)
	}
	config.RootCAPEM = rootCA

	funcMap := template.FuncMap{
		"cppString": cppString,
		"nvsPEM":    nvsPEM,
	}

	// Parse all three templates from the embedded FS.
	tmplBytes, err := templatesFS.ReadFile("templates/firmware.ino.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read firmware template: %w", err)
	}
	tmpl, err := template.New("firmware.ino").Funcs(funcMap).Parse(string(tmplBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse firmware template: %w", err)
	}

	provBytes, err := templatesFS.ReadFile("templates/provision.ino.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read provision template: %w", err)
	}
	if _, err = tmpl.New("provision.ino").Funcs(funcMap).Parse(string(provBytes)); err != nil {
		return nil, fmt.Errorf("failed to parse provision template: %w", err)
	}

	fleetBytes, err := templatesFS.ReadFile("templates/fleet_firmware.ino.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to read fleet_firmware template: %w", err)
	}
	if _, err = tmpl.New("fleet_firmware.ino").Funcs(funcMap).Parse(string(fleetBytes)); err != nil {
		return nil, fmt.Errorf("failed to parse fleet_firmware template: %w", err)
	}

	return &Generator{
		config: config,
		tmpl:   tmpl,
	}, nil
}

func (g *Generator) Generate(deviceID, workspaceID, apiKey, firmwareVersion, certPEM, privKeyPEM string) ([]byte, error) {
	return g.GenerateWithOptions(GenerateOptions{
		DeviceID:        deviceID,
		WorkspaceID:     workspaceID,
		APIKey:          apiKey,
		FirmwareVersion: firmwareVersion,
		CertPEM:         certPEM,
		PrivKeyPEM:      privKeyPEM,
	})
}

func (g *Generator) GenerateWithOptions(opts GenerateOptions) ([]byte, error) {
	certPEM, err := preparePEM("device certificate", opts.CertPEM, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("invalid device certificate: %w", err)
	}
	privKeyPEM, err := preparePEM("device private key", opts.PrivKeyPEM, "PRIVATE KEY")
	if err != nil {
		return nil, fmt.Errorf("invalid device private key: %w", err)
	}

	firmwareVersion := opts.FirmwareVersion
	if firmwareVersion == "" {
		firmwareVersion = g.config.DefaultFirmwareVersion
	}
	if firmwareVersion == "" {
		firmwareVersion = "0.0.0"
	}
	
	wifiSSID := opts.WiFiSSID
	if wifiSSID == "" {
		wifiSSID = g.config.WiFiSSID
	}
	
	wifiPassword := opts.WiFiPassword
	if wifiPassword == "" {
		wifiPassword = g.config.WiFiPassword
	}

	data := TemplateData{
		DeviceID:               opts.DeviceID,
		WorkspaceID:            opts.WorkspaceID,
		APIKey:                 opts.APIKey,
		ServerHost:             g.config.ServerHost,
		HTTPPort:               g.config.HTTPPort,
		MQTTPort:               g.config.MQTTPort,
		WiFiSSID:               wifiSSID,
		WiFiPassword:           wifiPassword,
		OTASigningKeyID:        g.config.OTASigningKeyID,
		OTASigningPublicKeyB64: g.config.OTASigningPublicKeyB64,
		FirmwareVersion:        firmwareVersion,
		RootCAPEM:              g.config.RootCAPEM,
		DeviceCertPEM:          certPEM,
		DevicePrivateKeyPEM:    privKeyPEM,
		UserCode:               opts.UserCode,
	}

	var buf bytes.Buffer
	if err := g.tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	sketch := buf.String()
	for _, item := range []struct {
		symbol string
		value  string
	}{
		{"ARGUS_ROOT_CA", g.config.RootCAPEM},
		{"ARGUS_DEVICE_CERT", certPEM},
		{"ARGUS_DEVICE_PRIVATE_KEY", privKeyPEM},
	} {
		if err := validateRenderedPEM(sketch, item.symbol, item.value); err != nil {
			return nil, fmt.Errorf("generated firmware validation failed: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// GenerateProvision renders the per-device provisioning sketch.
// The returned bytes should be saved as config_<deviceID>.ino.
func (g *Generator) GenerateProvision(opts GenerateOptions) ([]byte, error) {
	certPEM, err := preparePEM("device certificate", opts.CertPEM, "CERTIFICATE")
	if err != nil {
		return nil, fmt.Errorf("invalid device certificate: %w", err)
	}
	privKeyPEM, err := preparePEM("device private key", opts.PrivKeyPEM, "PRIVATE KEY")
	if err != nil {
		return nil, fmt.Errorf("invalid device private key: %w", err)
	}

	firmwareVersion := opts.FirmwareVersion
	if firmwareVersion == "" {
		firmwareVersion = g.config.DefaultFirmwareVersion
	}
	if firmwareVersion == "" {
		firmwareVersion = "0.0.0"
	}

	wifiSSID := opts.WiFiSSID
	if wifiSSID == "" {
		wifiSSID = g.config.WiFiSSID
	}
	wifiPassword := opts.WiFiPassword
	if wifiPassword == "" {
		wifiPassword = g.config.WiFiPassword
	}

	data := TemplateData{
		DeviceID:               opts.DeviceID,
		WorkspaceID:            opts.WorkspaceID,
		APIKey:                 opts.APIKey,
		ServerHost:             g.config.ServerHost,
		HTTPPort:               g.config.HTTPPort,
		MQTTPort:               g.config.MQTTPort,
		WiFiSSID:               wifiSSID,
		WiFiPassword:           wifiPassword,
		OTASigningKeyID:        g.config.OTASigningKeyID,
		OTASigningPublicKeyB64: g.config.OTASigningPublicKeyB64,
		FirmwareVersion:        firmwareVersion,
		RootCAPEM:              g.config.RootCAPEM,
		DeviceCertPEM:          certPEM,
		DevicePrivateKeyPEM:    privKeyPEM,
		UserCode:               opts.UserCode,
	}

	var buf bytes.Buffer
	if err := g.tmpl.ExecuteTemplate(&buf, "provision.ino", data); err != nil {
		return nil, fmt.Errorf("failed to execute provision template: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateFleetFirmware renders the fleet firmware sketch.
// The binary compiled from this sketch is identical for all fleet devices;
// device identity is loaded from NVS at boot via argusNVSLoad().
func (g *Generator) GenerateFleetFirmware(userCode string) ([]byte, error) {
	data := TemplateData{
		UserCode: userCode,
	}
	var buf bytes.Buffer
	if err := g.tmpl.ExecuteTemplate(&buf, "fleet_firmware.ino", data); err != nil {
		return nil, fmt.Errorf("failed to execute fleet_firmware template: %w", err)
	}
	return buf.Bytes(), nil
}
