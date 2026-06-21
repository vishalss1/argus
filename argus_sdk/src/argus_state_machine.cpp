#include "argus_state_machine.h"
#include <WiFi.h>
#include <PubSubClient.h>

namespace argus_sdk {

extern PubSubClient mqtt;

ConnectivityState connectivityState = ARGUS_WIFI_DOWN;

const char* connectivityStateName(ConnectivityState state) {
  switch (state) {
    case ARGUS_WIFI_DOWN: return "WIFI_DOWN";
    case ARGUS_WIFI_CONNECTED: return "WIFI_CONNECTED";
    case ARGUS_MQTT_CONNECTED: return "MQTT_CONNECTED";
    default: return "UNKNOWN";
  }
}

ConnectivityState computeConnectivityState() {
  if (WiFi.status() != WL_CONNECTED) return ARGUS_WIFI_DOWN;
  if (!mqtt.connected()) return ARGUS_WIFI_CONNECTED;
  return ARGUS_MQTT_CONNECTED;
}

void updateConnectivityState(const char* reason) {
  ConnectivityState next = computeConnectivityState();
  if (next != connectivityState) {
    Serial.printf("[NET] state %s -> %s reason=%s\n",
                  connectivityStateName(connectivityState),
                  connectivityStateName(next),
                  reason);
    connectivityState = next;
  }
}

}
