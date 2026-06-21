#include "argus_network.h"
#include "argus_config.h"
#include "argus_state_machine.h"
#include "argus_diag.h"
#include "argus_mqtt.h"
#include "argus_security.h"
#include <WiFi.h>
#include <PubSubClient.h>

namespace argus_sdk {

extern bool timeSynced;
extern void syncNTP();

Client* wifiClient = nullptr;
PubSubClient mqtt;

void initMqttClient() {
  if (wifiClient != nullptr) return;

  if (ARGUS_MQTT_PORT == 8883) {
    static ArgusWiFiClientSecure secureClient;
    wifiClient = &secureClient;
  } else {
    static WiFiClient plainClient;
    wifiClient = &plainClient;
  }
  mqtt.setClient(*wifiClient);
}

constexpr unsigned long MQTT_RECONNECT_MS = 5000UL;
constexpr unsigned long NETWORK_RESTART_AFTER_MS = 1800000UL;

unsigned long lastReconnectAttempt = 0;
unsigned long networkDegradedSinceMs = 0;

void connectWifi() {
  initMqttClient();
  if (WiFi.status() == WL_CONNECTED && timeSynced) return;

  if (WiFi.status() != WL_CONNECTED) {
    logNetState("before WiFi reconnect");
    Serial.printf("[WiFi] Connecting to %s", ::WIFI_SSID);
    WiFi.mode(WIFI_STA);
    WiFi.begin(::WIFI_SSID, ::WIFI_PASSWORD);

    unsigned long start = millis();
    while (WiFi.status() != WL_CONNECTED) {
      delay(500);
      Serial.print(".");
      if (millis() - start > 20000UL) {
        Serial.println("\n[WiFi] Connection timeout; normal loop will retry");
        updateConnectivityState("wifi reconnect timeout");
        logNetState("after WiFi reconnect failure");
        return;
      }
      yield();
    }

    Serial.println("\n[WiFi] Connected");
    Serial.print("[WiFi] IP: ");
    Serial.println(WiFi.localIP());
  }

  if (!timeSynced) {
    syncNTP();
    if (!timeSynced) {
      WiFi.disconnect();
      updateConnectivityState("ntp sync timeout");
      return;
    }
  }

  updateConnectivityState("wifi connected");
  logNetState("after WiFi reconnect");
}

void handleNetworkRecovery() {
  updateConnectivityState("recovery loop");
  logPeriodicNetworkDiagnostics();

  unsigned long now = millis();
  if (connectivityState != ARGUS_MQTT_CONNECTED) {
    if (networkDegradedSinceMs == 0) {
      networkDegradedSinceMs = now;
    } else if (now - networkDegradedSinceMs > NETWORK_RESTART_AFTER_MS) {
      Serial.println("[NET] Network degraded too long; restarting device");
      logNetState("before network recovery restart");
      delay(100);
      ESP.restart();
    }
  } else {
    networkDegradedSinceMs = 0;
  }

  if (connectivityState == ARGUS_WIFI_DOWN) {
    if (now - lastReconnectAttempt >= MQTT_RECONNECT_MS) {
      lastReconnectAttempt = now;
      connectWifi();
    }
    return;
  }

  if (connectivityState == ARGUS_WIFI_CONNECTED) {
    if (now - lastReconnectAttempt >= mqttReconnectBackoffMs) {
      lastReconnectAttempt = now;
      connectMQTT();
    }
    return;
  }

  mqtt.loop();
}

}
