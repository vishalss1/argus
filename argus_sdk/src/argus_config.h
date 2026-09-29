#pragma once

#include <Arduino.h>

// ---------------------------------------------------------------------------
// Device-specific configuration.
// Defined in argus_nvs.cpp and populated at boot by argusNVSLoad() from the
// "argus_cfg" NVS namespace. The per-device provisioning sketch
// (config_<deviceID>.ino) writes that namespace; the fleet firmware binary that
// runs afterwards is identical for every device and contains no baked-in
// identity.
// Declared non-const so argusNVSLoad() can populate the buffers at runtime.
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
// MQTT transport type. ARGUS_MQTT_SECURE is set by the provisioning sketch
// when the broker port is 8883.
// ---------------------------------------------------------------------------

#if defined(ARGUS_MQTT_SECURE)
#include <WiFiClientSecure.h>
#else
#include <WiFiClient.h>
#endif
