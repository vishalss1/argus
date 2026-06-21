#include "argus_http.h"
#include "argus_version.h"
#include "argus_config.h"
#include "argus_security.h"
#include "argus_diag.h"
#include "argus_state_machine.h"
#include <WiFi.h>
#include <ArduinoJson.h>
#include <PubSubClient.h>
#include <Preferences.h>

namespace argus_sdk {

constexpr int BACKEND_HTTP_TIMEOUT = 5000;
extern uint16_t mqttReconnectFailures;
extern PubSubClient mqtt;
extern bool timeSynced;

unsigned long lastHeartbeatMs = 0;
String deviceApiKey = ARGUS_API_KEY;

bool isHttpsUrl(const String& url) {
  return url.startsWith("https://");
}

bool isHttpUrl(const String& url) {
  return url.startsWith("http://");
}

static String argusApiBase() {
  return String("https://") + ARGUS_SERVER_HOST + ":" + String(ARGUS_HTTP_PORT);
}

void addArgusHeaders(HTTPClient& http) {
  http.addHeader("Accept", "application/json");
  http.addHeader("Connection", "close");

  if (deviceApiKey.length() > 0) {
    http.addHeader("X-Device-API-Key", deviceApiKey);
  }
}

void loadDeviceAPIKey() {
  Preferences prefs;
  prefs.begin("argus_device", true);
  String key = prefs.getString("api_key", "");
  prefs.end();
  if (key.length() > 0) {
    deviceApiKey = key;
  } else {
    deviceApiKey = ARGUS_API_KEY;
  }

  if (deviceApiKey.length() == 0) {
    Serial.println("[AUTH] Device API key not set; device auth will fail");
  } else {
    String prefix = deviceApiKey.length() >= 8 ? deviceApiKey.substring(0, 8) : deviceApiKey;
    Serial.printf("[AUTH] Device API key loaded. Length: %u, Prefix: %s\n",
                  (unsigned int)deviceApiKey.length(), prefix.c_str());
  }
}

void provisionDeviceAPIKey(const String& key) {
  Preferences prefs;
  if (prefs.begin("argus_device", false)) {
    prefs.putString("api_key", key);
    prefs.end();
    Serial.println("[AUTH] Device API key provisioned to NVS successfully");
  } else {
    Serial.println("[AUTH] Failed to open NVS namespace argus_device for provisioning");
  }
}

String httpGet(const String& path, int& httpCode) {
  logNetState("before HTTP GET");
  HTTPClient http;
  WiFiClient client;
  ArgusWiFiClientSecure secureClient;
  String url = argusApiBase() + path;

  Serial.printf("[HTTP] GET %s\n", url.c_str());
  bool https = isHttpsUrl(url);
  bool beginOk = false;
  if (https) {
    if (!timeSynced) {
      httpCode = -1;
      Serial.println("[HTTP] HTTPS rejected: time not synced via NTP");
      logNetState("after HTTP GET begin failure");
      return "";
    }
    if (!hasConfiguredRootCA()) {
      httpCode = -1;
      Serial.println("[HTTP] HTTPS rejected: no root CA configured for ARGUS API");
      logNetState("after HTTP GET begin failure");
      return "";
    }
    configureTLS(secureClient);
    beginOk = http.begin(secureClient, url);
  } else {
    beginOk = http.begin(client, url);
  }
  if (!beginOk) {
    httpCode = -1;
    Serial.println("[HTTP] Failed to initialize GET request");
    client.stop();
    secureClient.stop();
    logNetState("after HTTP GET begin failure");
    return "";
  }

  http.setTimeout(BACKEND_HTTP_TIMEOUT);
  http.setReuse(false);
  addArgusHeaders(http);
  httpCode = http.GET();
  if (https && httpCode > 0 && !verifyCertificatePin(secureClient, "ARGUS API")) {
    httpCode = -1;
    Serial.println("[HTTP] TLS certificate pin verification failed");
  }
  String response = (httpCode > 0 && httpCode != HTTP_CODE_NO_CONTENT) ? http.getString() : "";
  if (httpCode < 0) {
    logHTTPError(http, httpCode, secureClient, "GET");
  }
  http.end();

  Serial.printf("[HTTP] GET completed with status %d\n", httpCode);
  logNetState("after HTTP GET");
  return response;
}

String httpSendJson(const String& method, const String& path, const String& body, int* statusCode) {
  logNetState("before HTTP JSON");
  HTTPClient http;
  WiFiClient client;
  ArgusWiFiClientSecure secureClient;
  String url = argusApiBase() + path;

  Serial.printf("[HTTP] %s %s\n", method.c_str(), url.c_str());
  bool https = isHttpsUrl(url);
  bool beginOk = false;
  if (https) {
    if (!timeSynced) {
      if (statusCode) *statusCode = -1;
      Serial.println("[HTTP] HTTPS rejected: time not synced via NTP");
      logNetState("after HTTP JSON begin failure");
      return "";
    }
    if (!hasConfiguredRootCA()) {
      if (statusCode) *statusCode = -1;
      Serial.println("[HTTP] HTTPS rejected: no root CA configured for ARGUS API");
      logNetState("after HTTP JSON begin failure");
      return "";
    }
    configureTLS(secureClient);
    beginOk = http.begin(secureClient, url);
  } else {
    beginOk = http.begin(client, url);
  }
  if (!beginOk) {
    if (statusCode) *statusCode = -1;
    Serial.println("[HTTP] Failed to initialize JSON request");
    logNetState("after HTTP JSON begin failure");
    return "";
  }

  http.setTimeout(BACKEND_HTTP_TIMEOUT);
  http.setReuse(false);
  addArgusHeaders(http);
  http.addHeader("Content-Type", "application/json");

  Serial.printf("[HTTP] Payload length: %u\n", body.length());
  Serial.println("[HTTP] Payload body:");
  Serial.println(body);

  int code = http.sendRequest(method.c_str(), body);
  if (https && code > 0 && !verifyCertificatePin(secureClient, "ARGUS API")) {
    code = -1;
    Serial.println("[HTTP] TLS certificate pin verification failed");
  }
  if (statusCode) *statusCode = code;

  String response = (code > 0) ? http.getString() : "";
  if (code < 0) {
    logHTTPError(http, code, secureClient, method.c_str());
  } else if (code >= 400) {
    Serial.printf("[HTTP] Error code: %d\n", code);
    Serial.printf("[HTTP] Response length: %u\n", response.length());
    Serial.println("[HTTP] Response body:");
    Serial.println(response);
  }

  http.end();

  Serial.printf("[HTTP] %s completed with status %d\n", method.c_str(), code);
  logNetState("after HTTP JSON");
  return response;
}

String httpPost(const String& path, const String& body, int* statusCode) {
  return httpSendJson("POST", path, body, statusCode);
}

String httpPut(const String& path, const String& body, int* statusCode) {
  return httpSendJson("PUT", path, body, statusCode);
}

void updateShadow() {
  if (WiFi.status() != WL_CONNECTED) return;
  logNetState("before shadow update");

  DynamicJsonDocument doc(512);
  JsonObject reported = doc.createNestedObject("state");
  reported["fw"] = ARGUS_FW_VERSION;
  reported["firmware_version"] = ARGUS_FW_VERSION;
  reported["ip"] = WiFi.localIP().toString();
  reported["rssi"] = WiFi.RSSI();
  reported["free_heap"] = ESP.getFreeHeap();
  reported["largest_free_block"] = ESP.getMaxAllocHeap();
  reported["uptime_s"] = millis() / 1000;
  reported["mqtt_reconnects"] = mqttReconnectFailures;
  reported["mqtt_state"] = mqtt.state();

  String payload;
  serializeJson(doc, payload);

  Serial.println("[SHADOW] Sending payload:");
  Serial.println(payload);

  int code = 0;
  String response = httpPut(String("/api/devices/") + ARGUS_DEVICE_ID + "/shadow", payload, &code);
  Serial.printf("[SHADOW] Reported state update HTTP %d\n", code);
  if (code != 200 && response.length() > 0) {
    Serial.printf("[SHADOW] Error response: %s\n", response.c_str());
  }
  logNetState("after shadow update");
}

void sendHeartbeat() {
  if (WiFi.status() != WL_CONNECTED) return;
  logNetState("before heartbeat");

  DynamicJsonDocument h(192);
  h["device_id"] = ARGUS_DEVICE_ID;
  h["status"] = "online";
  h["firmware_version"] = ARGUS_FW_VERSION;

  String out;
  serializeJson(h, out);

  int code = 0;
  httpPost("/api/devices/heartbeat", out, &code);
  Serial.printf("[HEARTBEAT] HTTP %d\n", code);
  logNetState("after heartbeat");
}

}
