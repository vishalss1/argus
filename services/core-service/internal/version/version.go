// Package version implements the single semantic-version grammar ARGUS uses.
//
// The grammar is fixed by the device-side parser in argus_ota.cpp
// (parseVersion): an optional leading "v" or "V", then major.minor.patch where
// every component is non-empty and all-numeric. Nothing else is accepted — no
// pre-release, build or whitespace metadata.
//
// Two independent enforcement points depend on this being exactly right:
//
//   - the OTA upload path, which rejects a firmware binary whose ARGUSVER:
//     marker is missing, malformed or not semver; and
//   - the device, which refuses to flash when either its own compiled-in
//     version or the manifest version fails to parse.
package version

import (
	"fmt"
	"strings"
)

// maxLength bounds the marker payload accepted from an uploaded binary so a
// malformed or hostile image cannot force unbounded parsing work. It is
// comfortably above any plausible major.minor.patch.
const maxLength = 64

// Semver reports whether s satisfies the ARGUS semantic-version grammar.
func Semver(s string) bool {
	_, err := Parse(s)
	return err == nil
}

// Parse validates s and returns it in canonical form: the leading "v" or "V"
// removed. The components themselves are preserved exactly, including any
// leading zeros, because that is the value compiled into the binary and stored
// on the artifact.
//
// The returned error is written for a human reading an upload rejection.
func Parse(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("version is empty")
	}
	if len(s) > maxLength {
		return "", fmt.Errorf("version is too long (max %d characters)", maxLength)
	}
	candidate := s
	if candidate[0] == 'v' || candidate[0] == 'V' {
		candidate = candidate[1:]
	}

	parts := strings.Split(candidate, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("version %q is not major.minor.patch", s)
	}
	for _, part := range parts {
		if part == "" {
			return "", fmt.Errorf("version %q has an empty version component", s)
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return "", fmt.Errorf("version %q must be numeric", s)
			}
		}
	}

	return candidate, nil
}
