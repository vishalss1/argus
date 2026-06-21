#pragma once
#include <Arduino.h>

#include "argus_network.h"

namespace argus_sdk {

extern unsigned long mqttReconnectBackoffMs;
extern uint16_t mqttReconnectFailures;
extern unsigned long lastTelemetryMs;

extern String telemetryTopic;
extern String stateTopic;
extern String commandTopic;
extern String resultTopic;
extern String otaStatusTopic;

void connectMQTT();
void publishCommandResult(const String& commandId, const String& status, const String& message);
bool publishOTAResult(const String& deploymentId, const String& status, const String& message);
bool publishOTAACK(const String& deploymentId, const String& version);
void publishOTAStatus(const String& deploymentId, const String& status, int progress = -1, const String& message = "");
void publishOTANack(const String& deploymentId, const String& message);
void processPendingOTAACK();

}
