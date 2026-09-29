package handler

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/certificate"
	"github.com/vishalss1/argus/core/internal/domain/device"
	"github.com/vishalss1/argus/core/internal/firmware"
)

// createDeviceRepo embeds the interface so unimplemented methods panic loudly
// if this handler ever starts depending on them.
type createDeviceRepo struct {
	device.Repository
	created device.Device
}

func (r *createDeviceRepo) Create(_ context.Context, d device.Device) (*device.Device, error) {
	r.created = d
	return &d, nil
}

func newHandlerTestCA(t *testing.T) *certificate.CertificateAuthority {
	t.Helper()

	dir := t.TempDir()
	certPath := filepath.Join(dir, "ca.pem")
	keyPath := filepath.Join(dir, "ca.key")

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate CA serial: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Handler Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
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

	ca, err := certificate.NewCertificateAuthority(certPath, keyPath)
	if err != nil {
		t.Fatalf("create CA: %v", err)
	}
	return ca
}

func newCreateDeviceHandler(t *testing.T) (*DeviceHandler, *createDeviceRepo, *firmware.Generator) {
	t.Helper()

	ca := newHandlerTestCA(t)
	rootCAPEM := newTestRootCA(t)

	gen, err := firmware.NewGenerator(firmware.GeneratorConfig{
		ServerHost:             "192.168.10.100",
		HTTPPort:               8443,
		MQTTPort:               8883,
		RootCAPEM:              rootCAPEM,
		OTASigningKeyID:        "ota-test-v1",
		OTASigningPublicKeyB64: "OTAPUBLICKEYMOCK==",
	})
	if err != nil {
		t.Fatalf("create generator: %v", err)
	}

	repo := &createDeviceRepo{}
	return NewDeviceHandler(device.NewService(repo), nil, ca, gen), repo, gen
}

// newTestRootCA returns a self-signed root CA PEM for the generator config.
func newTestRootCA(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Argus Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// TestCreateDeviceReturnsProvisioningSketch pins the endpoint's contract: it
// must hand back the per-device NVS provisioning sketch, not a monolithic
// baked-in binary. The fleet firmware is a single shared binary, so a sketch
// that defined the config symbols would both duplicate argus_nvs.cpp's
// definitions at link time and break the "one binary per fleet" invariant.
func TestCreateDeviceReturnsProvisioningSketch(t *testing.T) {
	handler, _, _ := newCreateDeviceHandler(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/devices/",
		strings.NewReader(`{"name":"probe","type":"esp32","firmware_version":"1.0.0"}`))
	handler.CreateDevice(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	disposition := rr.Header().Get("Content-Disposition")
	if !strings.Contains(disposition, "config_") || !strings.Contains(disposition, ".ino") {
		t.Errorf("expected a config_<id>.ino attachment, got %q", disposition)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `prefs.putString("device_id"`) {
		t.Error("response is not a provisioning sketch: missing device_id write")
	}
	for _, forbidden := range []string{
		"char ARGUS_DEVICE_ID[]",
		"char ARGUS_API_KEY[]",
		"char ARGUS_DEVICE_PRIVATE_KEY[]",
	} {
		if strings.Contains(body, forbidden) {
			t.Errorf("provisioning sketch must not define SDK config symbols, found %q", forbidden)
		}
	}

	// The issued certificate must actually be embedded, not just referenced.
	if !strings.Contains(body, "putPEM(prefs, \"dev_cert\"") {
		t.Error("provisioning sketch does not write the issued device certificate")
	}
}

func TestCreateDeviceRejectsInvalidBody(t *testing.T) {
	handler, repo, _ := newCreateDeviceHandler(t)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/devices/", bytes.NewReader([]byte("{not json")))
	handler.CreateDevice(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	if repo.created.ID != "" {
		t.Error("no device should be created for an invalid body")
	}
}
