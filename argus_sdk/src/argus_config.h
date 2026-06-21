#pragma once

#include <Arduino.h>

// ---------------------------------------------------------------------------
// Device-specific configuration — defined in the generated firmware_<deviceID>.ino
// ---------------------------------------------------------------------------

extern const char ARGUS_FW_VERSION[];
extern const char ARGUS_DEVICE_ID[];
extern const char ARGUS_API_KEY[];
extern const char ARGUS_SERVER_HOST[];
extern const char ARGUS_MQTT_HOST[];
extern const uint16_t ARGUS_HTTP_PORT;
extern const uint16_t ARGUS_MQTT_PORT;
extern const char WIFI_SSID[];
extern const char WIFI_PASSWORD[];
extern const char ARGUS_OTA_KEY_ID[];
extern const char ARGUS_OTA_PUBLIC_KEY_B64[];
extern const char ARGUS_ROOT_CA[];
extern const char ARGUS_DEVICE_CERT[];
extern const char ARGUS_DEVICE_PRIVATE_KEY[];

// ---------------------------------------------------------------------------
// MQTT transport type — matches monolithic firmware conditional compilation
// ---------------------------------------------------------------------------

#if defined(ARGUS_MQTT_SECURE)
#include <WiFiClientSecure.h>
#else
#include <WiFiClient.h>
#endif
