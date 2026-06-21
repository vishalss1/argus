#include "argus_mqtt.h"
#include "argus_config.h"
#include "argus_state_machine.h"
#include "argus_diag.h"
#include "argus_security.h"
#include "argus_version.h"
#include "argus_ota.h"
#include <WiFi.h>
#include <ArduinoJson.h>

namespace argus_sdk {

constexpr int LED_PIN = 18;
constexpr unsigned long MQTT_RECONNECT_MS = 5000UL;
constexpr unsigned long MQTT_RECONNECT_MAX_MS = 60000UL;

unsigned long mqttReconnectBackoffMs = MQTT_RECONNECT_MS;
uint16_t mqttReconnectFailures = 0;
unsigned long lastTelemetryMs = 0;

String telemetryTopic;
String stateTopic;
String commandTopic;
String resultTopic;
String otaStatusTopic;

void onMqttMessage(char* topic, byte* payload, unsigned int length) {
  String msg;
  msg.reserve(length + 1);
  for (unsigned int i = 0; i < length; i++) msg += (char)payload[i];

  if (String(topic) != commandTopic) {
    Serial.printf("[MQTT] Ignoring message on unexpected topic %s\n", topic);
    return;
  }

  Serial.printf("[MQTT] Incoming command: %s\n", msg.c_str());

  DynamicJsonDocument doc(1024);
  DeserializationError err = deserializeJson(doc, msg);
  if (err) {
    Serial.printf("[MQTT] Failed to parse command JSON: %s\n", err.c_str());
    return;
  }

  String cmdId = doc["id"] | "";
  String type = doc["type"] | "unknown";
  JsonObject p = doc["payload"];

  if (cmdId.length() == 0) {
    Serial.println("[COMMAND] Missing command id; cannot report result");
    return;
  }

  if (type == "ping") {
    publishCommandResult(cmdId, "ack", "Pong from hardware");
  } else if (type == "led_on") {
    digitalWrite(LED_PIN, HIGH);
    publishCommandResult(cmdId, "ack", "GPIO18 set to HIGH");
  } else if (type == "led_off") {
    digitalWrite(LED_PIN, LOW);
    publishCommandResult(cmdId, "ack", "GPIO18 set to LOW");
  } else if (type == "led_blink") {
    int count = p["count"] | 5;
    int delayMs = p["delayMs"] | 250;

    Serial.printf("[ACTION] Blinking LED %d times with %d ms delay\n", count, delayMs);
    for (int i = 0; i < count; i++) {
      digitalWrite(LED_PIN, HIGH);
      delay(delayMs);
      digitalWrite(LED_PIN, LOW);
      delay(delayMs);
      mqtt.loop();
      yield();
    }
    publishCommandResult(cmdId, "ack", "Blink sequence finished");
  } else {
    publishCommandResult(cmdId, "nack", "Unknown command type");
  }
}

void connectMQTT() {
  initMqttClient();
  logNetState("before MQTT reconnect");
  Serial.printf("[MQTT] Pre-connect client.connected=%d state=%d\n",
                (wifiClient && wifiClient->connected()) ? 1 : 0, mqtt.state());
  if (!mqtt.connected()) {
    mqtt.disconnect();
    if (wifiClient) wifiClient->stop();
    delay(25);
    yield();
    logNetState("after MQTT stale socket cleanup");
  }

  if (ARGUS_MQTT_PORT == 8883 && wifiClient != nullptr) {
    configureTLS(*(WiFiClientSecure*)wifiClient);
  }

  mqtt.setServer(ARGUS_MQTT_HOST, ARGUS_MQTT_PORT);
  mqtt.setCallback(onMqttMessage);
  mqtt.setKeepAlive(15);
  mqtt.setSocketTimeout(10);

  StaticJsonDocument<192> lwt;
  lwt["device_id"] = ARGUS_DEVICE_ID;
  lwt["status"] = "offline";
  lwt["firmware_version"] = ARGUS_FW_VERSION;

  String lwtMsg;
  serializeJson(lwt, lwtMsg);

  String mqttClientId = String("argus-esp32-") + ARGUS_DEVICE_ID;

  Serial.println("[MQTT] Attempting connection...");
  bool connected = mqtt.connect(mqttClientId.c_str(), nullptr, nullptr, stateTopic.c_str(), 1, true, lwtMsg.c_str());
  if (!connected) {
    Serial.printf("[MQTT] Connection failed, rc=%d\n", mqtt.state());
    if (wifiClient) wifiClient->stop();
    mqttReconnectFailures++;
    uint16_t exponent = mqttReconnectFailures > 4 ? 4 : mqttReconnectFailures;
    unsigned long nextBackoff = MQTT_RECONNECT_MS * (1UL << exponent);
    mqttReconnectBackoffMs = nextBackoff > MQTT_RECONNECT_MAX_MS ? MQTT_RECONNECT_MAX_MS : nextBackoff;
    updateConnectivityState("mqtt reconnect failed");
    Serial.printf("[MQTT] Reconnect backoff now %lu ms failures=%u\n",
                  mqttReconnectBackoffMs, mqttReconnectFailures);
    logNetState("after MQTT reconnect failure");
    return;
  }

  mqttReconnectFailures = 0;
  mqttReconnectBackoffMs = MQTT_RECONNECT_MS;
  updateConnectivityState("mqtt connected");
  Serial.println("[MQTT] Connected to broker");

  if (mqtt.subscribe(commandTopic.c_str(), 1)) {
    Serial.printf("[MQTT] Subscribed to %s\n", commandTopic.c_str());
  } else {
    Serial.printf("[MQTT] Failed to subscribe to %s\n", commandTopic.c_str());
  }

  DynamicJsonDocument conn(256);
  conn["device_id"] = ARGUS_DEVICE_ID;
  conn["status"] = "online";
  conn["firmware_version"] = ARGUS_FW_VERSION;

  String connMsg;
  serializeJson(conn, connMsg);
  bool ok = mqtt.publish(stateTopic.c_str(), connMsg.c_str(), true);
  Serial.printf("[MQTT] Online presence publish %s: %s\n", ok ? "ok" : "failed", connMsg.c_str());

  processPendingOTAACK();
  logNetState("after MQTT reconnect");
}

void publishCommandResult(const String& commandId, const String& status, const String& message) {
  if (!mqtt.connected()) {
    Serial.printf("[COMMAND] MQTT disconnected; could not publish %s for command %s\n",
                  status.c_str(), commandId.c_str());
    return;
  }

  DynamicJsonDocument doc(512);
  doc["command_id"] = commandId;
  doc["status"] = status;
  doc["message"] = message;

  String payload;
  serializeJson(doc, payload);

  bool ok = mqtt.publish(resultTopic.c_str(), payload.c_str(), false);
  Serial.printf("[COMMAND] Result publish %s [%s] for ID %s: %s\n",
                ok ? "ok" : "failed", status.c_str(), commandId.c_str(), message.c_str());
}

bool publishOTAResult(const String& deploymentId, const String& status, const String& message) {
  if (!mqtt.connected()) {
    Serial.printf("[OTA] MQTT disconnected; could not publish %s for deployment %s\n",
                  status.c_str(), deploymentId.c_str());
    return false;
  }

  DynamicJsonDocument doc(512);
  doc["deployment_id"] = deploymentId;
  doc["status"] = status;
  doc["message"] = message;

  String payload;
  serializeJson(doc, payload);

  bool ok = mqtt.publish(resultTopic.c_str(), payload.c_str(), false);
  mqtt.loop();
  Serial.printf("[OTA] Result publish %s [%s] for deployment %s: %s\n",
                ok ? "ok" : "failed", status.c_str(), deploymentId.c_str(), message.c_str());
  Serial.printf("[OTA] Result payload: %s\n", payload.c_str());
  return ok;
}

bool publishOTAACK(const String& deploymentId, const String& version) {
  return publishOTAResult(deploymentId, "ack", "Flashed " + version + " successfully");
}

void publishOTAStatus(const String& deploymentId, const String& status, int progress, const String& message) {
  if (!mqtt.connected()) {
    Serial.printf("[OTA] MQTT disconnected; status %s for deployment %s not published\n",
                  status.c_str(), deploymentId.c_str());
    return;
  }

  DynamicJsonDocument doc(384);
  doc["deployment_id"] = deploymentId;
  doc["status"] = status;
  if (progress >= 0) doc["progress"] = progress;
  if (message.length() > 0) doc["message"] = message;

  String payload;
  serializeJson(doc, payload);
  bool ok = mqtt.publish(otaStatusTopic.c_str(), payload.c_str(), false);
  mqtt.loop();
  Serial.printf("[OTA] Status publish %s: %s\n", ok ? "ok" : "failed", payload.c_str());
}

void publishOTANack(const String& deploymentId, const String& message) {
  publishOTAResult(deploymentId, "nack", message);
}

void processPendingOTAACK() {
  PendingOTAResult pending = loadPendingOTAACK();
  if (!pending.needsAck) return;

  if (!mqtt.connected()) {
    Serial.println("[OTA] Pending ACK retry waiting for MQTT connection");
    return;
  }

  if (publishOTAACK(pending.deploymentId, pending.version)) {
    clearPendingOTAACK();
  } else {
    Serial.println("[OTA] Pending ACK publish failed; will retry");
  }
}

}
