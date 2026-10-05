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

// ---------------------------------------------------------------------------
// Version comparison (exposed for host tests)
// ---------------------------------------------------------------------------
//
// parseVersion implements the same grammar the backend enforces at upload time
// (services/core-service/internal/version): an optional leading 'v' or 'V',
// then major.minor.patch with every component non-empty and all-numeric.
//
// versionAllowed decides whether a manifest's version may be flashed. It
// compares against ARGUS_FW_VERSION, which is the compiled-in binary version,
// so a device flashed over OTA evaluates the next manifest against the code it
// is actually running.
struct Version {
  int major;
  int minor;
  int patch;
  bool valid;
};

Version parseVersion(const String& input);
int compareVersions(const Version& a, const Version& b);
bool versionAllowed(const OTAManifest& manifest);

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
