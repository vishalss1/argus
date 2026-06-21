#pragma once

#include <Client.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>

namespace argus_sdk {

extern Client* wifiClient;
extern PubSubClient mqtt;
extern unsigned long lastReconnectAttempt;

void connectWifi();
void handleNetworkRecovery();
void initMqttClient();

}
