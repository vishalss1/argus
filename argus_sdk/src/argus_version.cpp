#include "argus_version.h"

// The version marker, emitted exactly once per binary.
//
// The layout is a fixed contract with the backend's upload scanner:
//
//     ARGUSVER:\0<major.minor.patch>\0
//
// Adjacent string-literal concatenation produces the "ARGUSVER:" + NUL prefix,
// and the trailing NUL comes from the version literal's own terminator.
//
// This lives in its own translation unit on purpose. A `static` definition in
// argus_version.h would be emitted once per translation unit that includes it,
// which the ESP-IDF build compiles as six or more units — several independent
// copies in one image.
//
// `__attribute__((used))` on its own is not enough either. It prevents the
// compiler from dropping the symbol, but the ESP32 link runs with
// --gc-sections, which collects the unreferenced .rodata section regardless; the
// symbol then appears in the .map file but not in the ELF, and every OTA upload
// of that binary is rejected for a missing marker. argusBegin() calls
// argusFirmwareVersionFromMarker(), which is the reference that keeps this
// section alive.
__attribute__((used)) const char ARGUS_FIRMWARE_VERSION_MARKER[] =
    "ARGUSVER:\0" ARGUS_FIRMWARE_VERSION;