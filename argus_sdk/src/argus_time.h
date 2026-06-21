#pragma once
#include <Arduino.h>

namespace argus_sdk {

extern bool timeSynced;

void syncNTP();
bool isTimeAvailable();
bool parseRFC3339UTC(const String& value, time_t& parsed);
bool manifestExpired(const String& expiresAt);

}
