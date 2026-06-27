// argus_nvs.cpp
// ---------------------------------------------------------------------------
// Runtime NVS backing for all extern symbols declared in argus_config.h.
//
// At boot, argusNVSLoad() opens the "argus_cfg" NVS namespace (written once
// by the per-device provisioning sketch), copies every value into the static
// char arrays and scalar variables defined here, then closes the namespace.
// The rest of the SDK reads the populated buffers through the extern const
// declarations in argus_config.h — transparently, as if the values had been
// baked into a device-specific firmware.ino at compile time.
//
// Design note on const and writability
// --------------------------------------
// argus_config.h declares all 14 symbols as non-const externs.  This is a
// deliberate choice that supports two definition paths:
//
//   1. NVS loader path (this file, fleet OTA binary): argusNVSLoad() writes
//      into the buffers at boot after reading from the "argus_cfg" NVS
//      namespace.  Non-const storage is required for those direct assignments.
//
//   2. Monolithic baked-in path (firmware.ino.tmpl): the sketch defines the
//      same symbols with compile-time string/integer initialisers.  The
//      linker resolves extern references to whichever TU provides the storage.
//
// In C++ a const-qualified definition is NOT compatible with a non-const
// extern declaration (they are different types), so removing const from the
// header declarations is the only correct fix.  Code that must treat these
// values as read-only should use const pointers/references at call sites.
//
// Native-host guard
// -----------------
// ARGUS_NATIVE_BUILD is defined by the sdk_tests/ CMake toolchain.  That
// build satisfies the externs via config_shim.cpp (env-var values) and must
// not compile this file.
// ---------------------------------------------------------------------------

#ifndef ARGUS_NATIVE_BUILD

#include "argus_config.h"
#include <Preferences.h>
#include <Arduino.h>

// ---------------------------------------------------------------------------
// Buffer sizes
// ---------------------------------------------------------------------------

static constexpr size_t PEM_BUF = 4096;   // root_ca, dev_cert, dev_key
static constexpr size_t STR_BUF = 256;    // every other string field

// ---------------------------------------------------------------------------
// Extern symbol definitions.
//
// These are the actual storage objects for all 14 extern declarations in
// argus_config.h.  They are intentionally defined WITHOUT `const` so that
// argusNVSLoad() can write into them safely.  The const-qualified extern
// re-declarations in the header are compatible: they express a read-only
// view to all other translation units.
//
// All arrays are zero-initialised at program start (BSS).
// Port scalars carry their default values (8080 / 1883) so that code which
// runs before argusNVSLoad() — or in environments without NVS — gets a
// defined, non-zero value.
// ---------------------------------------------------------------------------

// ---- String fields (256 bytes each) ----
char ARGUS_FW_VERSION        [STR_BUF] = {};
char ARGUS_DEVICE_ID         [STR_BUF] = {};
char ARGUS_API_KEY           [STR_BUF] = {};
char ARGUS_SERVER_HOST       [STR_BUF] = {};
char ARGUS_MQTT_HOST         [STR_BUF] = {};
char WIFI_SSID               [STR_BUF] = {};
char WIFI_PASSWORD           [STR_BUF] = {};
char ARGUS_OTA_KEY_ID        [STR_BUF] = {};
char ARGUS_OTA_PUBLIC_KEY_B64[STR_BUF] = {};

// ---- PEM fields (4 096 bytes each) ----
char ARGUS_ROOT_CA           [PEM_BUF] = {};
char ARGUS_DEVICE_CERT       [PEM_BUF] = {};
char ARGUS_DEVICE_PRIVATE_KEY[PEM_BUF] = {};

// ---- Port scalars ----
// argus_config.h declares these as `extern const uint16_t`.  We define the
// storage as non-const uint16_t; the const re-declaration is compatible and
// lets us assign in argusNVSLoad() without a cast.
uint16_t ARGUS_HTTP_PORT = 8080;
uint16_t ARGUS_MQTT_PORT = 1883;

// ---------------------------------------------------------------------------
// PEM newline expansion.
//
// The provisioning sketch stores PEM data with literal two-character '\''n'
// sequences in place of real newline bytes.  This is necessary because NVS
// string entries are null-terminated C strings; some ESP-IDF versions corrupt
// or truncate values that contain raw '\n' bytes.  We convert them back to
// real newlines when loading.
// ---------------------------------------------------------------------------

static void expandNewlines(char* buf, size_t bufSize) {
    // Single-pass in-place expansion.  Output is always <= input length,
    // so we never overflow the buffer.
    size_t r = 0;
    size_t w = 0;
    while (buf[r] != '\0' && w + 1 < bufSize) {
        if (buf[r] == '\\' && buf[r + 1] == 'n') {
            buf[w++] = '\n';
            r += 2;
        } else {
            buf[w++] = buf[r++];
        }
    }
    buf[w] = '\0';
}

// ---------------------------------------------------------------------------
// NVS read helpers
// ---------------------------------------------------------------------------

// Read a string key from prefs into buf (up to bufSize bytes including '\0').
// Returns true if the key existed and was non-empty.
static bool nvs_read_str(Preferences& prefs, const char* key,
                         char* buf, size_t bufSize) {
    String val = prefs.getString(key, "");
    if (val.length() == 0) return false;
    strlcpy(buf, val.c_str(), bufSize);
    return true;
}

// Read a PEM key and expand literal \n sequences to real newlines on success.
static bool nvs_read_pem(Preferences& prefs, const char* key,
                         char* buf, size_t bufSize) {
    if (!nvs_read_str(prefs, key, buf, bufSize)) return false;
    expandNewlines(buf, bufSize);
    return true;
}

// ---------------------------------------------------------------------------
// argusNVSLoad()
//
// Opens "argus_cfg" read-only, populates all extern buffers, closes the
// namespace, and returns whether all required keys were present.
//
// Required keys  → returns false if any is absent or empty:
//   device_id, api_key, root_ca
//
// Optional keys  → missing/empty leaves the buffer at its zero default:
//   fw_version, server_host, mqtt_host, wifi_ssid, wifi_pass,
//   ota_key_id, ota_pub_key, dev_cert, dev_key, http_port, mqtt_port
// ---------------------------------------------------------------------------

bool argusNVSLoad() {
    Preferences prefs;

    // Open read-only: provisioning data must never be mutated by firmware.
    if (!prefs.begin("argus_cfg", /*readOnly=*/true)) {
        Serial.println("[NVS] Failed to open namespace argus_cfg");
        return false;
    }

    bool ok = true;

    // ---- Required keys ----

    if (!nvs_read_str(prefs, "device_id", ARGUS_DEVICE_ID, STR_BUF)) {
        Serial.println("[NVS] Missing required key: device_id");
        ok = false;
    }
    if (!nvs_read_str(prefs, "api_key", ARGUS_API_KEY, STR_BUF)) {
        Serial.println("[NVS] Missing required key: api_key");
        ok = false;
    }
    if (!nvs_read_pem(prefs, "root_ca", ARGUS_ROOT_CA, PEM_BUF)) {
        Serial.println("[NVS] Missing required key: root_ca");
        ok = false;
    }

    // ---- Optional keys ----

    nvs_read_str(prefs, "fw_version",  ARGUS_FW_VERSION,         STR_BUF);
    nvs_read_str(prefs, "server_host", ARGUS_SERVER_HOST,        STR_BUF);
    nvs_read_str(prefs, "mqtt_host",   ARGUS_MQTT_HOST,          STR_BUF);
    nvs_read_str(prefs, "wifi_ssid",   WIFI_SSID,                STR_BUF);
    nvs_read_str(prefs, "wifi_pass",   WIFI_PASSWORD,            STR_BUF);
    nvs_read_str(prefs, "ota_key_id",  ARGUS_OTA_KEY_ID,         STR_BUF);
    nvs_read_str(prefs, "ota_pub_key", ARGUS_OTA_PUBLIC_KEY_B64, STR_BUF);
    nvs_read_pem(prefs, "dev_cert",    ARGUS_DEVICE_CERT,        PEM_BUF);
    nvs_read_pem(prefs, "dev_key",     ARGUS_DEVICE_PRIVATE_KEY, PEM_BUF);

    // ---- Port scalars (getUInt returns uint32_t; safe to truncate) ----
    ARGUS_HTTP_PORT = static_cast<uint16_t>(prefs.getUInt("http_port", 8080));
    ARGUS_MQTT_PORT = static_cast<uint16_t>(prefs.getUInt("mqtt_port", 1883));

    prefs.end();

    if (ok) {
        Serial.printf("[NVS] Provisioned: device_id=%s http_port=%u mqtt_port=%u\n",
                      ARGUS_DEVICE_ID, ARGUS_HTTP_PORT, ARGUS_MQTT_PORT);
    }
    return ok;
}

// ---------------------------------------------------------------------------
// argusIsProvisioned()
//
// Lightweight probe: opens argus_cfg and checks whether "device_id" is
// present and non-empty.  Does NOT populate the runtime extern buffers.
// Safe to call before argusNVSLoad().
// ---------------------------------------------------------------------------

bool argusIsProvisioned() {
    Preferences prefs;
    if (!prefs.begin("argus_cfg", /*readOnly=*/true)) return false;
    String id = prefs.getString("device_id", "");
    prefs.end();
    return id.length() > 0;
}

#endif // ARGUS_NATIVE_BUILD
