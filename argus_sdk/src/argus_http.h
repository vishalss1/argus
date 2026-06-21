#pragma once
#include <Arduino.h>
#include <HTTPClient.h>

namespace argus_sdk {

extern unsigned long lastHeartbeatMs;

bool isHttpsUrl(const String& url);
bool isHttpUrl(const String& url);
void addArgusHeaders(HTTPClient& http);
void loadDeviceAPIKey();
void provisionDeviceAPIKey(const String& key);
String httpGet(const String& path, int& httpCode);
String httpSendJson(const String& method, const String& path, const String& body, int* statusCode = nullptr);
String httpPost(const String& path, const String& body, int* statusCode = nullptr);
String httpPut(const String& path, const String& body, int* statusCode = nullptr);
void updateShadow();
void sendHeartbeat();

}
