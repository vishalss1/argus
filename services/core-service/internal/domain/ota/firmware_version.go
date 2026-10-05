package ota

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/vishalss1/argus/core/internal/version"
)

// versionMarker is the fixed contract between the firmware build and this
// parser. argus_version.h emits, as a referenced and used symbol so the linker
// cannot discard it:
//
//	ARGUSVER:\0<major.minor.patch>\0
//
// The nine literal bytes "ARGUSVER:" followed by a NUL, then the version string,
// then a NUL from the literal terminator. We scan for the marker and read
// forward to the next NUL.
var versionMarker = []byte("ARGUSVER:")

// markerTailBytes is how many bytes past the marker we will read looking for the
// terminating NUL. It bounds the work a malformed image can force while staying
// comfortably above any plausible version string.
const markerTailBytes = 64

var (
	// ErrNoVersionMarker means the uploaded bytes contain no ARGUSVER: marker.
	ErrNoVersionMarker = errors.New("firmware binary does not contain an ARGUSVER: version marker")
	// ErrMalformedVersionMarker means the marker was present but the bytes
	// following it are not a usable version.
	ErrMalformedVersionMarker = errors.New("firmware binary contains a malformed ARGUSVER: version marker")
)

// VersionScanner extracts the firmware version from a compiled binary by
// scanning its bytes for the ARGUSVER: marker.
//
// It is a streaming scanner rather than a whole-file reader: firmware images are
// up to 256 MB and are streamed straight to object storage, so buffering one
// to find a few dozen bytes is not acceptable. Attach it with io.TeeReader so
// the same bytes reach the scanner and the store in a single pass, then call
// Version once the upload completes.
//
// Only the first marker is honoured. The firmware emits exactly one by
// construction, so a second occurrence means the image is not one we produced;
// the first match is still the authoritative one.
type VersionScanner struct {
	found bool
	// carry holds bytes from the end of the previous window that could be the
	// start of a marker split across the chunk boundary.
	carry []byte
	// reading holds bytes collected after the marker's separator NUL, waiting
	// for the NUL that terminates the version.
	reading []byte
	// state tracks how far into the marker contract we are:
	//   seekMarker  → looking for "ARGUSVER:"
	//   skipSep    → the marker matched; the next byte is its separator NUL
	//   readVersion → collecting the version until NUL
	state scanState
}

type scanState int

const (
	seekMarker scanState = iota
	skipSep
	readVersion
)

// NewVersionScanner returns a scanner ready to observe a firmware binary.
func NewVersionScanner() *VersionScanner {
	return &VersionScanner{}
}

// Write implements io.Writer so the scanner can sit in an io.TeeReader.
// It never returns an error: a missing or malformed marker is reported by
// Version, not by aborting the upload stream.
func (s *VersionScanner) Write(p []byte) (int, error) {
	n := len(p)
	if s.found {
		return n, nil
	}

	// Resume from where the previous chunk left off, then treat this chunk and
	// the carry-over as one contiguous window so a marker split across a chunk
	// boundary is still found.
	buf := p
	if len(s.carry) > 0 {
		buf = append(append(make([]byte, 0, len(s.carry)+len(p)), s.carry...), p...)
		s.carry = nil
	}

	consumed := 0
	for consumed < len(buf) {
		switch s.state {
		case seekMarker:
			idx := bytes.Index(buf[consumed:], versionMarker)
			if idx < 0 {
				// Keep a tail so a marker straddling the boundary survives.
				keep := len(versionMarker) - 1
				remaining := len(buf) - consumed
				if remaining > keep {
					s.carry = append(s.carry[:0], buf[len(buf)-keep:]...)
				} else {
					s.carry = append(s.carry[:0], buf[consumed:]...)
				}
				return n, nil
			}
			consumed += idx + len(versionMarker)
			s.state = skipSep

		case skipSep:
			// The contract puts a NUL immediately after "ARGUSVER:". That
			// separator must not be mistaken for the end of the version.
			if consumed >= len(buf) {
				return n, nil
			}
			if buf[consumed] != 0 {
				// Not the layout we emit; abandon this marker.
				s.state = seekMarker
				s.reading = nil
				continue
			}
			consumed++
			s.state = readVersion

		case readVersion:
			for consumed < len(buf) {
				b := buf[consumed]
				if b == 0 {
					s.finish()
					return n, nil
				}
				if len(s.reading) >= markerTailBytes {
					// No terminator within the bound. Abandon the marker rather
					// than reading unboundedly far into the image.
					s.state = seekMarker
					s.reading = nil
					break
				}
				s.reading = append(s.reading, b)
				consumed++
			}
			if consumed >= len(buf) {
				return n, nil
			}
		}
	}

	return n, nil
}

// finish latches the collected bytes and stops further scanning.
func (s *VersionScanner) finish() {
	s.found = true
	s.state = readVersion
	s.carry = nil
}

// Version returns the version derived from the scanned binary.
//
// It fails closed: a binary with no marker, or with a marker whose payload is
// malformed or not semver, is an error rather than a defaulted version.
func (s *VersionScanner) Version() (string, error) {
	if !s.found {
		return "", ErrNoVersionMarker
	}

	raw := string(s.reading)
	if raw == "" {
		return "", fmt.Errorf("%w: marker is not followed by a version string", ErrMalformedVersionMarker)
	}
	parsed, err := version.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrMalformedVersionMarker, err)
	}
	return parsed, nil
}

// ReadVersion scans r to completion and returns the firmware version it
// contains. It is the convenience form for callers that already hold the whole
// image; the upload path uses VersionScanner directly to avoid buffering.
func ReadVersion(r io.Reader) (string, error) {
	scanner := NewVersionScanner()
	if _, err := io.Copy(scanner, r); err != nil {
		return "", fmt.Errorf("failed to scan firmware binary: %w", err)
	}
	return scanner.Version()
}
