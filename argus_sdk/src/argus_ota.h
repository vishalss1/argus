#pragma once
#include <Arduino.h>
#include <Preferences.h>

namespace argus_sdk {

struct OTAManifest {
  String deploymentId;
  String deviceId;
  String firmwareId;
  String version;
  String filename;
  String contentType;
  uint32_t sizeBytes;
  String checksumSha256;
  String signatureAlg;
  String signature;
  String signingKeyId;
  String downloadUrl;
  String expiresAt;
  bool allowDowngrade;
};

struct PendingOTAResult {
  String deploymentId;
  String version;
  bool needsAck;
};

extern Preferences otaPrefs;
extern bool otaInProgress;
extern bool otaPartitionCapable;
extern unsigned long lastOtaPollMs;
extern unsigned long lastOtaAckRetryMs;

void checkOTA();
bool validateOTAPartitions();
void openOTAPreferences();
void persistPendingOTAACK(const String& deploymentId, const String& version);
PendingOTAResult loadPendingOTAACK();
void clearPendingOTAACK();

}
