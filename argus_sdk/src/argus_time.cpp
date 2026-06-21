#include "argus_time.h"
#include <WiFi.h>
#include <time.h>

namespace argus_sdk {

constexpr long MIN_EPOCH = 1704067200L; // 2024-01-01 00:00:00 UTC

bool timeSynced = false;

void syncNTP() {
  if (timeSynced) return;
  Serial.println("[NTP] Syncing time...");
  configTime(0, 0, "pool.ntp.org", "time.nist.gov");

  int retry = 0;
  while (time(nullptr) < MIN_EPOCH && retry < 20) {
    delay(500);
    Serial.print(".");
    retry++;
  }

  if (time(nullptr) < MIN_EPOCH) {
    Serial.println("\n[NTP] ERROR: Failed to sync time. TLS operations blocked!");
    timeSynced = false;
  } else {
    time_t now = time(nullptr);
    Serial.printf("\n[NTP] Time synchronized: %ld UTC\n", (long)now);
    timeSynced = true;
  }
}

bool isTimeAvailable() {
  time_t now = time(nullptr);
  return now > 1700000000;
}

bool parseRFC3339UTC(const String& value, time_t& parsed) {
  int year, month, day, hour, minute, second;
  if (sscanf(value.c_str(), "%d-%d-%dT%d:%d:%d", &year, &month, &day, &hour, &minute, &second) != 6) {
    return false;
  }

  struct tm t;
  memset(&t, 0, sizeof(t));
  t.tm_year = year - 1900;
  t.tm_mon = month - 1;
  t.tm_mday = day;
  t.tm_hour = hour;
  t.tm_min = minute;
  t.tm_sec = second;
  t.tm_isdst = 0;
  parsed = mktime(&t);
  return parsed > 0;
}

bool manifestExpired(const String& expiresAt) {
  if (expiresAt.length() == 0) {
    Serial.println("[OTA] Manifest has no expires_at; backend compatibility mode continues");
    return false;
  }
  if (!isTimeAvailable()) {
    Serial.println("[OTA] Device time unavailable; cannot enforce expires_at, continuing");
    return false;
  }

  time_t expires;
  if (!parseRFC3339UTC(expiresAt, expires)) {
    Serial.println("[OTA] Could not parse expires_at; rejecting manifest");
    return true;
  }

  time_t now = time(nullptr);
  if (now >= expires) {
    Serial.printf("[OTA] Manifest expired at %s; now=%ld\n", expiresAt.c_str(), (long)now);
    return true;
  }

  return false;
}

}
