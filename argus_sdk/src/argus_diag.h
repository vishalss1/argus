#pragma once
#include <HTTPClient.h>
#include <WiFiClientSecure.h>

namespace argus_sdk {

extern unsigned long lastNetDiagMs;

void logNetState(const char* phase);
void logPeriodicNetworkDiagnostics();
void logTLSDiagnostics(WiFiClientSecure& client, const char* url);
void logHTTPError(HTTPClient& http, int code, WiFiClientSecure& secureClient, const char* method);
void logOTANetworkState(const char* phase);

}
