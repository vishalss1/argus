package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/vishalss1/argus/core/internal/domain/ota"
)

// drainingObjectStore consumes the uploaded stream, which is what the real store
// does. fakeObjectStore discards it, which would starve the version scanner and
// make these tests pass for the wrong reason.
type drainingObjectStore struct {
	stored   []byte
	removed  []string
	putCalls int
}

func (s *drainingObjectStore) PutFirmware(ctx context.Context, objectKey string, reader io.Reader, sizeBytes int64, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.stored = data
	s.putCalls++
	return nil
}

func (s *drainingObjectStore) FirmwareURL(ctx context.Context, objectKey string, filename string, expires time.Duration) (string, error) {
	return "http://minio.local/" + objectKey, nil
}

func (s *drainingObjectStore) GetFirmware(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.stored)), nil
}

func (s *drainingObjectStore) RemoveFirmware(ctx context.Context, objectKey string) error {
	s.removed = append(s.removed, objectKey)
	s.stored = nil
	return nil
}

func firmwareImage(version string) []byte {
	marker := append([]byte("ARGUSVER:\x00"), append([]byte(version), 0)...)
	body := make([]byte, 512)
	copy(body, []byte("\x7fELF\x02\x01\x01\x00"))
	copy(body[256:], marker)
	return body
}

func newUploadRequest(t *testing.T, image []byte, extraFields map[string]string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("firmware", "fleet_firmware.bin")
	if err != nil {
		t.Fatalf("failed to create firmware form file: %v", err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatalf("failed to write firmware bytes: %v", err)
	}

	for name, value := range extraFields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatalf("failed to write field %s: %v", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ota/firmware", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// The upload modal no longer sends a version field; the handler must derive the
// version from the binary and return it.
func TestUploadFirmwareDerivesVersionFromBinary(t *testing.T) {
	repo := newFakeOTARepository()
	store := &drainingObjectStore{}
	handler := NewOTAHandler(ota.NewService(repo, store))

	rr := httptest.NewRecorder()
	handler.UploadFirmware(rr, newUploadRequest(t, firmwareImage("1.4.0"), nil))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var artifact ota.FirmwareArtifact
	if err := json.Unmarshal(rr.Body.Bytes(), &artifact); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rr.Body.String())
	}
	if artifact.Version != "1.4.0" {
		t.Errorf("response version = %q, want %q", artifact.Version, "1.4.0")
	}

	var created *ota.FirmwareArtifact
	for id, stored := range repo.artifacts {
		if stored.ID == artifact.ID {
			c := stored
			created = &c
		}
		_ = id
	}
	if created == nil {
		t.Fatal("expected the artifact to be persisted")
	}
	if created.Version != "1.4.0" {
		t.Errorf("persisted version = %q, want %q", created.Version, "1.4.0")
	}
	if len(store.stored) == 0 {
		t.Error("expected the binary to reach the object store")
	}
}

// A stray version field must not be able to override what the binary says. The
// handler no longer reads it, and the service no longer accepts it.
func TestUploadFirmwareIgnoresSubmittedVersionField(t *testing.T) {
	repo := newFakeOTARepository()
	store := &drainingObjectStore{}
	handler := NewOTAHandler(ota.NewService(repo, store))

	rr := httptest.NewRecorder()
	handler.UploadFirmware(rr, newUploadRequest(t, firmwareImage("1.4.0"), map[string]string{
		"version": "9.9.9",
	}))

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rr.Code, rr.Body.String())
	}

	var artifact ota.FirmwareArtifact
	if err := json.Unmarshal(rr.Body.Bytes(), &artifact); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if artifact.Version != "1.4.0" {
		t.Errorf("version = %q, want the binary's 1.4.0 rather than the submitted 9.9.9", artifact.Version)
	}
}

// A binary with no usable marker is rejected with a 400 that explains why, so an
// operator uploading the wrong file is not left guessing.
func TestUploadFirmwareRejectsBinaryWithoutVersionMarker(t *testing.T) {
	tests := []struct {
		name        string
		image       []byte
		wantMessage string
	}{
		{
			name:        "no marker",
			image:       []byte("\x7fELF opaque firmware image"),
			wantMessage: "ARGUSVER:",
		},
		{
			name:        "malformed version",
			image:       []byte("ARGUSVER:\x00not-a-version\x00"),
			wantMessage: "ARGUSVER:",
		},
		{
			name:        "not semver",
			image:       []byte("ARGUSVER:\x001.2.x\x00"),
			wantMessage: "ARGUSVER:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeOTARepository()
			store := &drainingObjectStore{}
			handler := NewOTAHandler(ota.NewService(repo, store))

			rr := httptest.NewRecorder()
			handler.UploadFirmware(rr, newUploadRequest(t, tt.image, nil))

			if rr.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
			}
			if !bytes.Contains(rr.Body.Bytes(), []byte(tt.wantMessage)) {
				t.Errorf("expected the error to mention %q, got %s", tt.wantMessage, rr.Body.String())
			}
			if len(store.removed) == 0 {
				t.Error("the rejected object should have been removed from the store")
			}
			// Only the pre-seeded fixture artifact should remain.
			if len(repo.artifacts) != 1 {
				t.Errorf("expected no new artifact rows, got %d total", len(repo.artifacts))
			}
		})
	}
}

// The file field is still mandatory; dropping it must not silently succeed.
func TestUploadFirmwareRequiresFile(t *testing.T) {
	repo := newFakeOTARepository()
	store := &drainingObjectStore{}
	handler := NewOTAHandler(ota.NewService(repo, store))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.WriteField("version", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/ota/firmware", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handler.UploadFirmware(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
	if store.putCalls != 0 {
		t.Error("a request without a file must not reach the object store")
	}
}

var _ ota.ObjectStore = (*drainingObjectStore)(nil)
