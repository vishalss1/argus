package ota

import (
	"bytes"
	"os"
	"testing"
)

// loadFirmwareFixture reads testdata/firmware_v1_2_0.bin, a small synthetic
// ESP32-style image that carries a real ARGUSVER: marker in the middle of
// otherwise opaque bytes.
//
// The fixture exists so the scan path is exercised against a file on disk rather
// than only against in-memory byte slices assembled by the test itself.
func loadFirmwareFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/firmware_v1_2_0.bin")
	if err != nil {
		t.Fatalf("failed to read firmware fixture: %v", err)
	}
	return data
}

func TestReadVersionFromFixtureBinary(t *testing.T) {
	got, err := ReadVersion(bytes.NewReader(loadFirmwareFixture(t)))
	if err != nil {
		t.Fatalf("ReadVersion returned error: %v", err)
	}
	if got != "1.2.0" {
		t.Errorf("ReadVersion = %q, want %q", got, "1.2.0")
	}
}

// The scanner consumes the image in whatever chunk sizes the object store
// chooses, and a marker can straddle a chunk boundary. Feed the same fixture in
// every chunk size from 1 upward to prove no boundary loses the marker.
func TestVersionScannerHandlesChunkBoundaries(t *testing.T) {
	image := loadFirmwareFixture(t)

	for chunkSize := 1; chunkSize <= len(image); chunkSize++ {
		scanner := NewVersionScanner()
		for offset := 0; offset < len(image); offset += chunkSize {
			end := offset + chunkSize
			if end > len(image) {
				end = len(image)
			}
			if _, err := scanner.Write(image[offset:end]); err != nil {
				t.Fatalf("chunkSize=%d: Write returned error: %v", chunkSize, err)
			}
		}
		got, err := scanner.Version()
		if err != nil {
			t.Fatalf("chunkSize=%d: Version returned error: %v", chunkSize, err)
		}
		if got != "1.2.0" {
			t.Fatalf("chunkSize=%d: Version = %q, want %q", chunkSize, got, "1.2.0")
		}
	}
}

func TestVersionScannerRejectsBadMarkers(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "no marker at all",
			body: []byte("\x7fELF\x02\x01\x01\x00opaque firmware bytes"),
		},
		{
			name: "marker with no version after it",
			body: []byte("prefix ARGUSVER:\x00 trailing bytes"),
		},
		{
			name: "marker not terminated",
			body: []byte("prefix ARGUSVER:\x00 1.2.0 and then unterminated garbage"),
		},
		{
			name: "version is not semver",
			body: []byte("prefix ARGUSVER:\x00not-a-version\x00"),
		},
		{
			name: "version missing patch",
			body: []byte("prefix ARGUSVER:\x001.2\x00"),
		},
		{
			name: "version has prerelease suffix",
			body: []byte("prefix ARGUSVER:\x001.2.0-rc1\x00"),
		},
		{
			name: "version is empty string",
			body: []byte("prefix ARGUSVER:\x00\x00"),
		},
		{
			name: "near miss marker text",
			body: []byte("prefix ARGUSVERS:\x001.2.0\x00"),
		},
		{
			name: "empty image",
			body: []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadVersion(bytes.NewReader(tt.body))
			if err == nil {
				t.Fatal("expected the scan to fail, got nil error")
			}
		})
	}
}

// Only the first marker is authoritative, matching the firmware's guarantee that
// exactly one is emitted.
func TestVersionScannerAcceptsFirstMarkerOnly(t *testing.T) {
	body := append([]byte("ARGUSVER:\x001.2.3\x00"), []byte("ARGUSVER:\x009.9.9\x00")...)
	got, err := ReadVersion(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("ReadVersion returned error: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("ReadVersion = %q, want %q", got, "1.2.3")
	}
}

// Once a version has been latched, later writes must not disturb it.
func TestVersionScannerIgnoresWritesAfterVersion(t *testing.T) {
	scanner := NewVersionScanner()
	if _, err := scanner.Write([]byte("ARGUSVER:\x001.2.0\x00")); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.Write([]byte("ARGUSVER:\x007.7.7\x00")); err != nil {
		t.Fatal(err)
	}
	got, err := scanner.Version()
	if err != nil {
		t.Fatal(err)
	}
	if got != "1.2.0" {
		t.Errorf("Version = %q, want %q", got, "1.2.0")
	}
}

// An unterminated marker must not make the scanner read unboundedly far into the
// image looking for a NUL.
func TestVersionScannerBoundsUnterminatedMarker(t *testing.T) {
	body := make([]byte, 4096)
	copy(body, []byte("ARGUSVER:\x00"))
	for i := 13; i < len(body); i++ {
		body[i] = 'x'
	}

	scanner := NewVersionScanner()
	if _, err := scanner.Write(body); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.Version(); err == nil {
		t.Fatal("expected an unterminated marker to be rejected")
	}
}

func TestVersionScannerIgnoresWriteErrors(t *testing.T) {
	// Write must never fail: a bad marker is reported by Version, and returning
	// an error here would abort the upload stream instead.
	scanner := NewVersionScanner()
	n, err := scanner.Write([]byte("no marker here"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != len("no marker here") {
		t.Errorf("Write consumed %d bytes, want %d", n, len("no marker here"))
	}
}
