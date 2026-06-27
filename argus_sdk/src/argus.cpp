/**
 * ARGUS ESP32 SDK — Core Implementation
 * --------------------------------------
 * Infrastructure logic is delegated to modules.
 */

#include "argus.h"
#include "argus_nvs.h"
#include "argus_version.h"
#include <sodium.h>
#include <WiFi.h>

namespace argus_sdk {
  constexpr unsigned long TELEMETRY_MS = 5000UL;
  constexpr unsigned long HEARTBEAT_MS = 30000UL;
  constexpr unsigned long OTA_POLL_MS = 60000UL;
  constexpr unsigned long OTA_ACK_RETRY_MS = 10000UL;
}

using namespace argus_sdk;

static float currentTemp = 26.5;
static float currentBattery = 85.0;
static float currentRamUsage = 42.0;
static float currentCpuLoad = 12.0;

void internalPublishTelemetry() {
  logNetState("before telemetry publish");

  currentTemp += (random(-2, 3) / 10.0);
  if (currentTemp < 24.0) currentTemp = 24.0;
  if (currentTemp > 32.0) currentTemp = 32.0;

  currentRamUsage += (random(-5, 6) / 10.0);
  if (currentRamUsage < 35.0) currentRamUsage = 35.0;
  if (currentRamUsage > 55.0) currentRamUsage = 55.0;

  currentCpuLoad += (random(-10, 11) / 10.0);
  if (currentCpuLoad < 5.0) currentCpuLoad = 5.0;
  if (currentCpuLoad > 30.0) currentCpuLoad = 30.0;

  currentBattery -= 0.01;
  if (currentBattery < 70.0) currentBattery = 90.0;

  if (!mqtt.connected()) {
    Serial.println("[TELEMETRY] MQTT disconnected; telemetry skipped");
    logNetState("after telemetry skipped");
    return;
  }

  DynamicJsonDocument t(768);
  JsonObject m = t.createNestedObject("metrics");
  m["temp_c"] = currentTemp;
  m["ram_usage"] = currentRamUsage;
  m["cpu_load"] = currentCpuLoad;
  m["battery_level"] = currentBattery;
  m["rssi_dbm"] = WiFi.RSSI();
  m["uptime_s"] = millis() / 1000;
  m["free_heap"] = ESP.getFreeHeap();
  m["firmware_version"] = ARGUS_FW_VERSION;

  String out;
  serializeJson(t, out);

  bool ok = mqtt.publish(telemetryTopic.c_str(), out.c_str(), false);
  Serial.printf("[TELEMETRY] Publish %s: %s\n", ok ? "ok" : "failed", out.c_str());
  if (!ok) {
    Serial.printf("[TELEMETRY] MQTT publish failed state=%d client_connected=%d\n",
                  mqtt.state(), (wifiClient && wifiClient->connected()) ? 1 : 0);
  }
  logNetState("after telemetry publish");
}

void argusBegin() {
  // NVS must be loaded before anything else so all extern symbols are
  // populated.  A missing device identity would cause silent misbehaviour
  // (empty device ID in MQTT topics, TLS without a CA, etc.).
  if (!argusNVSLoad()) {
    Serial.println("[BOOT] NVS not provisioned — halting");
    while (true) { delay(1000); }
  }

  delay(100);
  setenv("TZ", "UTC0", 1);
  tzset();

  Serial.println();
  Serial.println("[BOOT] ARGUS ESP32 firmware starting");
  Serial.printf("[BOOT] Device ID: %s\n", ARGUS_DEVICE_ID);
  Serial.printf("[BOOT] Firmware version: %s\n", ARGUS_FW_VERSION);
  if (sodium_init() < 0) {
    Serial.println("[BOOT] libsodium initialization failed; OTA signature verification unavailable");
  } else {
    Serial.println("[BOOT] libsodium initialized for Ed25519 OTA verification");
  }

  pinMode(18, OUTPUT);
  digitalWrite(18, LOW);

  telemetryTopic = String("argus/devices/") + ARGUS_DEVICE_ID + "/telemetry";
  stateTopic     = String("argus/devices/") + ARGUS_DEVICE_ID + "/state";
  commandTopic   = String("argus/devices/") + ARGUS_DEVICE_ID + "/commands";
  resultTopic    = String("argus/devices/") + ARGUS_DEVICE_ID + "/results";
  otaStatusTopic = String("argus/devices/") + ARGUS_DEVICE_ID + "/ota/status";

  Serial.printf("[BOOT] Telemetry topic: %s\n", telemetryTopic.c_str());
  Serial.printf("[BOOT] State topic: %s\n", stateTopic.c_str());
  Serial.printf("[BOOT] Command topic: %s\n", commandTopic.c_str());
  Serial.printf("[BOOT] Result topic: %s\n", resultTopic.c_str());
  Serial.printf("[BOOT] OTA status topic: %s\n", otaStatusTopic.c_str());

  openOTAPreferences();
  otaPartitionCapable = validateOTAPartitions();

  provisionDeviceAPIKey(ARGUS_API_KEY);
  loadDeviceAPIKey();

  connectWifi();
  connectMQTT();
  updateConnectivityState("setup complete");
  handleRollbackVerification();
  updateShadow();
  processPendingOTAACK();

  lastOtaPollMs = millis();
  checkOTA();
}

void argusLoop() {
  handleNetworkRecovery();
  unsigned long now = millis();

  if (now - lastOtaAckRetryMs >= OTA_ACK_RETRY_MS) {
    lastOtaAckRetryMs = now;
    processPendingOTAACK();
  }

  if (now - lastOtaPollMs >= OTA_POLL_MS) {
    lastOtaPollMs = now;
    if (connectivityState != ARGUS_WIFI_DOWN) {
      checkOTA();
    }
  }

  if (!otaInProgress && connectivityState == ARGUS_MQTT_CONNECTED && now - lastTelemetryMs >= TELEMETRY_MS) {
    lastTelemetryMs = now;
    internalPublishTelemetry();
  }

  if (!otaInProgress && connectivityState != ARGUS_WIFI_DOWN && now - lastHeartbeatMs >= HEARTBEAT_MS) {
    lastHeartbeatMs = now;
    sendHeartbeat();
  }
  delay(10);
}

void argusPublishTelemetry(JsonDocument& metrics) {
  if (!mqtt.connected()) {
    Serial.println("[TELEMETRY] MQTT disconnected; telemetry skipped");
    return;
  }

  String out;
  serializeJson(metrics, out);

  bool ok = mqtt.publish(telemetryTopic.c_str(), out.c_str(), false);
  Serial.printf("[TELEMETRY] Manual publish %s: %s\n", ok ? "ok" : "failed", out.c_str());
}

void argusPublishEvent(const String& type, JsonDocument& payload) {
  if (!mqtt.connected()) {
    Serial.println("[EVENT] MQTT disconnected; event skipped");
    return;
  }

  DynamicJsonDocument doc(1024);
  doc["type"] = type;
  doc["payload"] = payload;

  String out;
  serializeJson(doc, out);

  String eventTopic = String("argus/devices/") + ARGUS_DEVICE_ID + "/events";
  bool ok = mqtt.publish(eventTopic.c_str(), out.c_str(), false);
  Serial.printf("[EVENT] Publish %s [%s]: %s\n", ok ? "ok" : "failed", type.c_str(), out.c_str());
}
