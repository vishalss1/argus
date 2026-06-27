#pragma once

#include <Arduino.h>

// ---------------------------------------------------------------------------
// Device-specific configuration.
// Defined either in argus_nvs.cpp (NVS loader path, fleet OTA binary) or in
// the generated per-device firmware sketch (monolithic baked-in path).
// Declared non-const so both definition sites can write/initialize the storage
// and argusNVSLoad() can populate the buffers at runtime without a cast.
// ---------------------------------------------------------------------------

extern char ARGUS_FW_VERSION[];
extern char ARGUS_DEVICE_ID[];
extern char ARGUS_API_KEY[];
extern char ARGUS_SERVER_HOST[];
extern char ARGUS_MQTT_HOST[];
extern uint16_t ARGUS_HTTP_PORT;
extern uint16_t ARGUS_MQTT_PORT;
extern char WIFI_SSID[];
extern char WIFI_PASSWORD[];
extern char ARGUS_OTA_KEY_ID[];
extern char ARGUS_OTA_PUBLIC_KEY_B64[];
extern char ARGUS_ROOT_CA[];
extern char ARGUS_DEVICE_CERT[];
extern char ARGUS_DEVICE_PRIVATE_KEY[];

// ---------------------------------------------------------------------------
// MQTT transport type — matches monolithic firmware conditional compilation
// ---------------------------------------------------------------------------

#if defined(ARGUS_MQTT_SECURE)
#include <WiFiClientSecure.h>
#else
#include <WiFiClient.h>
#endif
