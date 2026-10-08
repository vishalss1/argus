#pragma once

#include <Client.h>
#include <WiFiClientSecure.h>
#include <PubSubClient.h>

namespace argus_sdk {

extern Client* wifiClient;
extern PubSubClient mqtt;

void connectWifi();
void handleNetworkRecovery();
void initMqttClient();

}
