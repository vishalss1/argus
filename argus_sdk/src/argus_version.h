#pragma once

// ARGUS_FW_VERSION is declared in argus_config.h and defined (storage) in
// argus_nvs.cpp. Its value comes from ARGUS_FIRMWARE_VERSION below, which is
// compiled into the binary, not read from NVS at boot.

#include <stdint.h>

// ---------------------------------------------------------------------------
// Compiled-in firmware version — the single source of truth for the version of
// the running binary.
// ---------------------------------------------------------------------------
// The fleet firmware template defines ARGUS_FIRMWARE_VERSION before including
// <argus.h>, so the value that ships is whatever the sketch says. The default
// here keeps the header self-contained for other build configurations.
//
// To change the version, edit the definition in the firmware source. Nothing
// else — no UI field, no database override — can change it.
//
// ARGUS_FIRMWARE_VERSION_MARKER is the machine-readable form the backend reads
// when firmware is uploaded for OTA. The layout is a fixed contract:
//
//     ARGUSVER:\0<major.minor.patch>\0
//
// The backend scans uploaded binaries for "ARGUSVER:" and reads forward to the
// next NUL; an upload whose marker is missing, malformed or not semver is
// rejected rather than defaulted.
//
// Two details are load-bearing, both learned the hard way:
//
//   - It is *defined* in exactly one translation unit (argus_version.cpp) and
//     declared here, rather than being a `static` in this header. A header-local
//     static would be emitted once per including TU, producing several
//     independent copies in the same image.
//   - `__attribute__((used))` is NOT sufficient to keep it. That stops the
//     compiler discarding it, but the ESP32 link runs with --gc-sections, which
//     garbage-collects the unreferenced .rodata section anyway — the symbol ends
//     up in the map file but not in the ELF. argusBegin() therefore reads the
//     marker at boot, which is a genuine reference from retained code.
#ifndef ARGUS_FIRMWARE_VERSION
#define ARGUS_FIRMWARE_VERSION "1.0.0"
#endif

#define ARGUS_FIRMWARE_VERSION_MARKER_PREFIX "ARGUSVER:"
#define ARGUS_FIRMWARE_VERSION_MARKER_PREFIX_LEN 10  // 9 literal bytes + the NUL

extern const char ARGUS_FIRMWARE_VERSION_MARKER[];

// Returns the version string embedded in the marker, i.e. the bytes after the
// "ARGUSVER:\0" prefix. Reading it this way keeps the marker referenced by
// retained code, which is what stops --gc-sections from dropping it.
inline const char* argusFirmwareVersionFromMarker() {
  return ARGUS_FIRMWARE_VERSION_MARKER + ARGUS_FIRMWARE_VERSION_MARKER_PREFIX_LEN;
}

namespace argus_sdk
{
    constexpr char SDK_VERSION[] = "1.0.0";
    constexpr uint32_t SDK_VERSION_MAJOR = 1;
    constexpr uint32_t SDK_VERSION_MINOR = 0;
    constexpr uint32_t SDK_VERSION_PATCH = 0;
}

inline const char* getArgusSdkVersion() {
  return argus_sdk::SDK_VERSION;
}

inline uint32_t getArgusSdkVersionMajor() {
  return argus_sdk::SDK_VERSION_MAJOR;
}

inline uint32_t getArgusSdkVersionMinor() {
  return argus_sdk::SDK_VERSION_MINOR;
}

inline uint32_t getArgusSdkVersionPatch() {
  return argus_sdk::SDK_VERSION_PATCH;
}