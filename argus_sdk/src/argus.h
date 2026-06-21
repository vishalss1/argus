#pragma once

#include <ArduinoJson.h>
#include "argus_config.h"
#include "argus_state_machine.h"
#include "argus_diag.h"
#include "argus_network.h"
#include "argus_security.h"
#include "argus_http.h"
#include "argus_mqtt.h"
#include "argus_time.h"
#include "argus_ota.h"
#include "argus_rollback.h"

#include "argus_version.h"

void argusBegin();
void argusLoop();
void argusPublishTelemetry(JsonDocument& metrics);
void argusPublishEvent(const String& type, JsonDocument& payload);
