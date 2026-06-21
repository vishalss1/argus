#include "argus_diag.h"
#include "argus_state_machine.h"
#include "argus_config.h"
#include "argus_security.h"
#include <WiFi.h>
#include <PubSubClient.h>
#include <time.h>

namespace argus_sdk {

extern PubSubClient mqtt;
extern bool timeSynced;

constexpr unsigned long NET_DIAG_MS = 60000UL;
unsigned long lastNetDiagMs = 0;

void logNetState(const char* phase) {
  Serial.printf("[NET] %s heap=%u largest=%u wifi=%d rssi=%d mqtt_connected=%d mqtt_state=%d state=%s uptime=%lu local_ip=%s\n",
                phase,
                ESP.getFreeHeap(),
                ESP.getMaxAllocHeap(),
                WiFi.status(),
                WiFi.RSSI(),
                mqtt.connected() ? 1 : 0,
                mqtt.state(),
                connectivityStateName(connectivityState),
                millis() / 1000UL,
                WiFi.localIP().toString().c_str());
}

void logPeriodicNetworkDiagnostics() {
  unsigned long now = millis();
  if (now - lastNetDiagMs < NET_DIAG_MS) return;
  lastNetDiagMs = now;
  updateConnectivityState("periodic diagnostics");
  logNetState("periodic diagnostics");
}

void logTLSDiagnostics(WiFiClientSecure& client, const char* url) {
  time_t now = time(nullptr);
  struct tm timeinfo;
  gmtime_r(&now, &timeinfo);
  char timeStr[64];
  strftime(timeStr, sizeof(timeStr), "%Y-%m-%d %H:%M:%S UTC", &timeinfo);

  Serial.println("--- [TLS DIAGNOSTICS] ---");
  Serial.printf("Target: %s\n", url);
  Serial.printf("Epoch:  %ld\n", (long)now);
  Serial.printf("Time:   %s\n", timeStr);
  Serial.printf("CA Configured:  %s\n", hasConfiguredRootCA() ? "YES" : "NO");
  String pin = normalizedHex(String(ARGUS_PINNED_CERT_SHA256));
  Serial.printf("Pin Configured: %s\n", pin.length() > 0 ? "YES" : "NO");
  Serial.println("-------------------------");
}

void logHTTPError(HTTPClient& http, int code, WiFiClientSecure& secureClient, const char* method) {
  Serial.printf("[HTTP] %s failed code=%d (%s)\n", method, code, http.errorToString(code).c_str());

  if (code < 0) {
     char buf[128];
     secureClient.lastError(buf, sizeof(buf));
     Serial.printf("[TLS] Last SSL Error: %s\n", buf);

     if (strstr(buf, "BADCERT_EXPIRED")) {
       Serial.println("[TLS] Cause: Certificate expired (Check ESP32 time!)");
     } else if (strstr(buf, "BADCERT_FUTURE")) {
       Serial.println("[TLS] Cause: Certificate not yet valid (Check ESP32 time!)");
     } else if (strstr(buf, "BADCERT_NOT_TRUSTED")) {
       Serial.println("[TLS] Cause: Unknown CA (Check ARGUS_ROOT_CA_PEM!)");
     } else if (strstr(buf, "BADCERT_CN_MISMATCH")) {
       Serial.println("[TLS] Cause: SAN/CN mismatch (Check ARGUS_HOST vs Cert!)");
     } else if (code == HTTPC_ERROR_CONNECTION_REFUSED) {
       Serial.println("[TLS] Cause: Connection Refused (TCP layer or TLS handshake abort)");
     }
  }
}

void logOTANetworkState(const char* phase) {
  Serial.printf("[OTA] %s wifi status=%d free heap=%u largest=%u local IP=%s\n",
                phase, WiFi.status(), ESP.getFreeHeap(), ESP.getMaxAllocHeap(), WiFi.localIP().toString().c_str());
}

}
