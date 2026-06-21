#include "argus_ota.h"
#include "argus_config.h"
#include "argus_state_machine.h"
#include "argus_diag.h"
#include "argus_security.h"
#include "argus_time.h"
#include "argus_mqtt.h"
#include "argus_http.h"

#include <WiFi.h>
#include <HTTPClient.h>
#include <Update.h>
#include <ArduinoJson.h>
#include <mbedtls/md.h>
#include <sodium.h>
#include <esp_ota_ops.h>
#include <esp_partition.h>

#define FW_VERSION       "v1.0.0"
#define OTA_POLL_MS        60000UL
#define OTA_ACK_RETRY_MS   10000UL
#define OTA_CHUNK_BYTES    2048
#define OTA_HTTP_TIMEOUT   30000
#define OTA_MAX_REDIRECTS  5
#define ARGUS_REQUIRE_FIRMWARE_SIGNATURES true

Preferences otaPrefs;
bool otaInProgress = false;
bool otaPartitionCapable = false;
unsigned long lastOtaPollMs = 0;
unsigned long lastOtaAckRetryMs = 0;

// ---------------------------------------------------------------------------
// Partition helpers
// ---------------------------------------------------------------------------

String partitionName(const esp_partition_t* p) {
  if (p == nullptr) return "(none)";
  return String(p->label);
}

String partitionSubtypeName(const esp_partition_t* p) {
  if (p == nullptr) return "(none)";
  if (p->subtype == ESP_PARTITION_SUBTYPE_APP_FACTORY) return "factory";
  if (p->subtype >= ESP_PARTITION_SUBTYPE_APP_OTA_0 && p->subtype <= ESP_PARTITION_SUBTYPE_APP_OTA_15) {
    return "ota_" + String(p->subtype - ESP_PARTITION_SUBTYPE_APP_OTA_0);
  }
  if (p->subtype == ESP_PARTITION_SUBTYPE_APP_TEST) return "test";
  return "unknown";
}

void logPartition(const char* label, const esp_partition_t* p) {
  if (p == nullptr) {
    Serial.printf("[OTA] %s partition: none\n", label);
    return;
  }

  Serial.printf("[OTA] %s partition: label=%s subtype=%s address=0x%08x size=%u\n",
                label,
                p->label,
                partitionSubtypeName(p).c_str(),
                (unsigned int)p->address,
                (unsigned int)p->size);
}

bool validateOTAPartitions() {
  const esp_partition_t* running = esp_ota_get_running_partition();
  const esp_partition_t* next = esp_ota_get_next_update_partition(nullptr);

  Serial.println("[OTA] Validating OTA partition layout...");
  logPartition("Current", running);
  logPartition("Next update", next);
  Serial.printf("[OTA] Sketch size: %u bytes\n", (unsigned int)ESP.getSketchSize());
  Serial.printf("[OTA] Free sketch space reported by Arduino: %u bytes\n", (unsigned int)ESP.getFreeSketchSpace());

  if (running == nullptr) {
    Serial.println("[OTA] OTA disabled: running partition could not be detected");
    return false;
  }
  if (next == nullptr) {
    Serial.println("[OTA] OTA disabled: no next update partition available");
    return false;
  }
  if (next == running) {
    Serial.println("[OTA] OTA disabled: next update partition equals running partition");
    return false;
  }
  if (!(next->subtype >= ESP_PARTITION_SUBTYPE_APP_OTA_0 && next->subtype <= ESP_PARTITION_SUBTYPE_APP_OTA_15)) {
    Serial.println("[OTA] OTA disabled: next partition is not an OTA app partition");
    return false;
  }

  Serial.printf("[OTA] OTA free partition space: %u bytes\n", (unsigned int)next->size);
  Serial.println("[OTA] OTA partition layout is valid");
  return true;
}

// ---------------------------------------------------------------------------
// Version parsing
// ---------------------------------------------------------------------------

struct Version {
  int major;
  int minor;
  int patch;
  bool valid;
};

Version parseVersion(const String& input) {
  String s = input;
  s.trim();
  if (s.startsWith("v") || s.startsWith("V")) {
    s.remove(0, 1);
  }

  Version v = {0, 0, 0, false};
  int first = s.indexOf('.');
  int second = s.indexOf('.', first + 1);
  if (first <= 0 || second <= first + 1 || second >= (int)s.length() - 1) {
    return v;
  }

  String majorStr = s.substring(0, first);
  String minorStr = s.substring(first + 1, second);
  String patchStr = s.substring(second + 1);

  for (unsigned int i = 0; i < majorStr.length(); i++) if (!isDigit(majorStr[i])) return v;
  for (unsigned int i = 0; i < minorStr.length(); i++) if (!isDigit(minorStr[i])) return v;
  for (unsigned int i = 0; i < patchStr.length(); i++) if (!isDigit(patchStr[i])) return v;

  v.major = majorStr.toInt();
  v.minor = minorStr.toInt();
  v.patch = patchStr.toInt();
  v.valid = true;
  return v;
}

int compareVersions(const Version& a, const Version& b) {
  if (a.major != b.major) return a.major < b.major ? -1 : 1;
  if (a.minor != b.minor) return a.minor < b.minor ? -1 : 1;
  if (a.patch != b.patch) return a.patch < b.patch ? -1 : 1;
  return 0;
}

// ---------------------------------------------------------------------------
// Hash / signature helpers
// ---------------------------------------------------------------------------

void updateHash(mbedtls_md_context_t& ctx, const uint8_t* data, size_t len) {
  mbedtls_md_update(&ctx, data, len);
}

String finishHashHex(mbedtls_md_context_t& ctx) {
  uint8_t digest[32];
  mbedtls_md_finish(&ctx, digest);

  char hex[65];
  for (int i = 0; i < 32; i++) {
    sprintf(&hex[i * 2], "%02x", digest[i]);
  }
  hex[64] = '\0';
  return String(hex);
}

bool verifyFirmwareAuthenticity(const OTAManifest& manifest, const String& calculatedSha256) {
  if (!calculatedSha256.equalsIgnoreCase(manifest.checksumSha256)) {
    Serial.println("[OTA] SHA256 verification failed");
    return false;
  }

  if (!ARGUS_REQUIRE_FIRMWARE_SIGNATURES && manifest.signature.length() == 0) {
    Serial.println("[OTA] SHA256 passed; unsigned firmware accepted because signature requirement is disabled");
    return true;
  }

  if (manifest.signatureAlg != "ed25519" || manifest.signature.length() == 0) {
    Serial.println("[OTA] Firmware signature missing or unsupported");
    return false;
  }
  if (manifest.signingKeyId != String(ARGUS_OTA_KEY_ID)) {
    Serial.printf("[OTA] Firmware signing key mismatch got=%s expected=%s\n",
                  manifest.signingKeyId.c_str(), ARGUS_OTA_KEY_ID);
    return false;
  }

  uint8_t publicKey[crypto_sign_PUBLICKEYBYTES];
  uint8_t signature[crypto_sign_BYTES];
  size_t publicKeyLen = 0;
  size_t signatureLen = 0;
  if (!decodeBase64Strict(String(ARGUS_OTA_PUBLIC_KEY_B64), publicKey, sizeof(publicKey), publicKeyLen) ||
      publicKeyLen != crypto_sign_PUBLICKEYBYTES) {
    Serial.println("[OTA] Ed25519 public key is missing or invalid");
    return false;
  }
  if (!decodeBase64Strict(manifest.signature, signature, sizeof(signature), signatureLen) ||
      signatureLen != crypto_sign_BYTES) {
    Serial.println("[OTA] Firmware signature base64 is invalid");
    return false;
  }

  String checksum = calculatedSha256;
  checksum.toLowerCase();
  int verify = crypto_sign_verify_detached(
    signature,
    (const unsigned char*)checksum.c_str(),
    checksum.length(),
    publicKey
  );
  if (verify != 0) {
    Serial.println("[OTA] Ed25519 signature verification failed");
    return false;
  }

  Serial.println("[OTA] SHA256 and Ed25519 signature verification passed");
  return true;
}

// ---------------------------------------------------------------------------
// NVS durable OTA result state
// ---------------------------------------------------------------------------

void openOTAPreferences() {
  if (!otaPrefs.begin("argus_ota", false)) {
    Serial.println("[OTA] Failed to open NVS namespace argus_ota");
  }
}

void persistPendingOTAACK(const String& deploymentId, const String& version) {
  otaPrefs.putString("deployment_id", deploymentId);
  otaPrefs.putString("version", version);
  otaPrefs.putBool("needs_ack", true);
  Serial.printf("[OTA] Persisted pending ACK in NVS: deployment_id=%s version=%s\n",
                deploymentId.c_str(), version.c_str());
}

PendingOTAResult loadPendingOTAACK() {
  PendingOTAResult pending;
  pending.deploymentId = otaPrefs.getString("deployment_id", "");
  pending.version = otaPrefs.getString("version", "");
  pending.needsAck = otaPrefs.getBool("needs_ack", false) && pending.deploymentId.length() > 0;

  if (pending.needsAck) {
    Serial.printf("[OTA] Pending ACK loaded from NVS: deployment_id=%s version=%s\n",
                  pending.deploymentId.c_str(), pending.version.c_str());
  }
  return pending;
}

void clearPendingOTAACK() {
  otaPrefs.remove("deployment_id");
  otaPrefs.remove("version");
  otaPrefs.putBool("needs_ack", false);
  Serial.println("[OTA] Cleared pending ACK from NVS");
}

// ---------------------------------------------------------------------------
// OTA manifest and flashing
// ---------------------------------------------------------------------------

bool parseOTAManifest(const String& response, OTAManifest& manifest) {
  DynamicJsonDocument doc(4096);
  DeserializationError err = deserializeJson(doc, response);
  if (err) {
    Serial.printf("[OTA] Manifest parse failed: %s\n", err.c_str());
    return false;
  }

  manifest.deploymentId = doc["deployment_id"] | "";
  manifest.deviceId = doc["device_id"] | "";
  manifest.firmwareId = doc["firmware_id"] | "";
  manifest.version = doc["version"] | "";
  manifest.filename = doc["filename"] | "";
  manifest.contentType = doc["content_type"] | "";
  manifest.sizeBytes = doc["size_bytes"] | 0;
  manifest.checksumSha256 = doc["checksum_sha256"] | "";
  manifest.signatureAlg = doc["signature_alg"] | "";
  manifest.signature = doc["signature"] | "";
  manifest.signingKeyId = doc["signing_key_id"] | "";
  manifest.downloadUrl = doc["download_url"] | "";
  manifest.expiresAt = doc["expires_at"] | "";
  manifest.allowDowngrade = doc["allow_downgrade"] | false;

  Serial.printf("[OTA] Manifest parsed: deployment_id=%s version=%s size=%u expires_at=%s allow_downgrade=%s\n",
                manifest.deploymentId.c_str(),
                manifest.version.c_str(),
                (unsigned int)manifest.sizeBytes,
                manifest.expiresAt.c_str(),
                manifest.allowDowngrade ? "true" : "false");
  Serial.printf("[OTA] Manifest download_url=%s\n", manifest.downloadUrl.c_str());
  Serial.printf("[OTA] Manifest checksum_sha256=%s\n", manifest.checksumSha256.c_str());
  Serial.printf("[OTA] Manifest signature_alg=%s signing_key_id=%s signature_len=%u\n",
                manifest.signatureAlg.c_str(), manifest.signingKeyId.c_str(), (unsigned int)manifest.signature.length());

  if (manifest.deploymentId.length() == 0) {
    Serial.println("[OTA] Manifest missing deployment_id");
    return false;
  }
  if (manifest.version.length() == 0) {
    Serial.println("[OTA] Manifest missing version");
    return false;
  }
  if (manifest.checksumSha256.length() != 64) {
    Serial.println("[OTA] Manifest missing or invalid checksum_sha256");
    return false;
  }
  if (manifest.downloadUrl.length() == 0 || (!isHttpUrl(manifest.downloadUrl) && !isHttpsUrl(manifest.downloadUrl))) {
    Serial.println("[OTA] Manifest missing or invalid download_url");
    return false;
  }
  if (manifest.downloadUrl.indexOf("localhost") >= 0 || manifest.downloadUrl.indexOf("127.0.0.1") >= 0) {
    Serial.println("[OTA] Manifest download_url points to localhost/127.0.0.1, which ESP32 cannot reach");
    return false;
  }
  if (ARGUS_REQUIRE_FIRMWARE_SIGNATURES) {
    if (manifest.signatureAlg != "ed25519") {
      Serial.println("[OTA] Manifest missing required signature_alg=ed25519");
      return false;
    }
    if (manifest.signature.length() == 0) {
      Serial.println("[OTA] Manifest missing required firmware signature");
      return false;
    }
    if (manifest.signingKeyId != String(ARGUS_OTA_KEY_ID)) {
      Serial.printf("[OTA] Unexpected signing_key_id=%s expected=%s\n",
                    manifest.signingKeyId.c_str(), ARGUS_OTA_KEY_ID);
      return false;
    }
  }

  return true;
}

bool versionAllowed(const OTAManifest& manifest) {
  Version current = parseVersion(FW_VERSION);
  Version target = parseVersion(manifest.version);

  if (!current.valid || !target.valid) {
    Serial.printf("[OTA] Invalid semantic version. current=%s target=%s\n", FW_VERSION, manifest.version.c_str());
    return false;
  }

  int cmp = compareVersions(target, current);
  if (cmp == 0) {
    Serial.println("[OTA] Target version matches current version; skipping flash");
    publishOTAACK(manifest.deploymentId, manifest.version);
    return false;
  }
  if (cmp < 0 && !manifest.allowDowngrade) {
    Serial.printf("[OTA] Rejecting downgrade from %s to %s\n", FW_VERSION, manifest.version.c_str());
    publishOTANack(manifest.deploymentId, "Downgrade rejected");
    return false;
  }

  return true;
}

bool beginFirmwareHTTP(HTTPClient& http, WiFiClient& plainClient, WiFiClientSecure& secureClient, const String& url) {
  http.setTimeout(OTA_HTTP_TIMEOUT);
  http.setReuse(false);
  http.setFollowRedirects(HTTPC_STRICT_FOLLOW_REDIRECTS);
  http.setRedirectLimit(OTA_MAX_REDIRECTS);

  if (isHttpsUrl(url)) {
    Serial.println("[OTA] HTTPS firmware URL detected");
    if (!timeSynced) {
      Serial.println("[OTA] HTTPS rejected: time not synced via NTP");
      return false;
    }
    if (!hasConfiguredRootCA()) {
      Serial.println("[OTA] HTTPS rejected: no root CA configured for certificate validation");
      return false;
    }

    logTLSDiagnostics(secureClient, url.c_str());
    configureTLSClient(secureClient);
    bool ok = http.begin(secureClient, url);
    if (ok) http.addHeader("Connection", "close");
    return ok;
  }

  Serial.println("[OTA] HTTP firmware URL detected");
  bool ok = http.begin(plainClient, url);
  if (ok) http.addHeader("Connection", "close");
  return ok;
}

bool parseFirmwareURL(const String& url, String& scheme, String& host, String& path, uint16_t& port) {
  scheme = "";
  host = "";
  path = "/";
  port = 0;

  int schemeEnd = url.indexOf("://");
  if (schemeEnd <= 0) return false;

  scheme = url.substring(0, schemeEnd);
  int authorityStart = schemeEnd + 3;
  int pathStart = url.indexOf('/', authorityStart);
  String authority = pathStart >= 0 ? url.substring(authorityStart, pathStart) : url.substring(authorityStart);
  path = pathStart >= 0 ? url.substring(pathStart) : "/";

  int at = authority.lastIndexOf('@');
  if (at >= 0) authority = authority.substring(at + 1);

  int colon = authority.lastIndexOf(':');
  if (colon > 0) {
    host = authority.substring(0, colon);
    int parsedPort = authority.substring(colon + 1).toInt();
    if (parsedPort <= 0 || parsedPort > 65535) return false;
    port = (uint16_t)parsedPort;
  } else {
    host = authority;
    port = scheme == "https" ? 443 : 80;
  }

  return host.length() > 0 && (scheme == "http" || scheme == "https");
}

bool connectTCPWithDiagnostics(WiFiClient& client, const String& host, uint16_t port, const char* label) {
  IPAddress ip;
  bool isIP = ip.fromString(host);

  Serial.printf("[OTA] %s host=%s\n", label, host.c_str());
  Serial.printf("[OTA] %s port=%d\n", label, port);
  logOTANetworkState("before TCP connect");

  bool ok = false;
  if (isIP) {
    ok = client.connect(ip, port);
  } else {
    ok = client.connect(host.c_str(), port);
  }

  Serial.printf("[OTA] %s result=%s available=%d connected=%d\n",
                label, ok ? "ok" : "failed", client.available(), client.connected());
  logOTANetworkState("after TCP connect");
  return ok;
}

bool failFirmwareDownload(HTTPClient& http, bool httpStarted, WiFiClient& plainClient, WiFiClientSecure& secureClient, bool updateStarted, const String& deploymentId, const String& message) {
  if (updateStarted) {
    Update.abort();
  }
  if (httpStarted) {
    http.end();
    plainClient.stop();
    secureClient.stop();
  }
  publishOTANack(deploymentId, message);
  logNetState("after OTA HTTPS download failure");
  return false;
}

bool failRawFirmwareDownload(WiFiClient& client, bool clientStarted, bool updateStarted, const String& deploymentId, const String& message) {
  if (updateStarted) {
    Update.abort();
  }
  if (clientStarted) {
    client.stop();
  }
  publishOTANack(deploymentId, message);
  logNetState("after OTA download failure");
  return false;
}

int parseHTTPStatusCode(const String& statusLine) {
  int firstSpace = statusLine.indexOf(' ');
  if (firstSpace < 0 || firstSpace + 4 > statusLine.length()) return -1;
  return statusLine.substring(firstSpace + 1, firstSpace + 4).toInt();
}

bool downloadVerifyAndFlashPlainHTTP(const OTAManifest& manifest) {
  logNetState("before OTA download");
  String scheme;
  String host;
  String path;
  uint16_t port;
  if (!parseFirmwareURL(manifest.downloadUrl, scheme, host, path, port) || scheme != "http") {
    publishOTANack(manifest.deploymentId, "Invalid HTTP firmware URL");
    logNetState("after OTA download failure");
    return false;
  }
  if (ARGUS_REQUIRE_FIRMWARE_SIGNATURES) {
    publishOTANack(manifest.deploymentId, "HTTPS firmware URL required");
    logNetState("after OTA download failure");
    return false;
  }

  WiFiClient client;
  bool clientStarted = false;
  bool updateStarted = false;
  client.setTimeout(OTA_HTTP_TIMEOUT);
  clientStarted = true;

  Serial.printf("[OTA] raw HTTP connect host=%s port=%u\n", host.c_str(), port);
  if (!connectTCPWithDiagnostics(client, host, port, "raw firmware TCP connect")) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Firmware TCP connect failed");
  }

  Serial.printf("[OTA] raw HTTP GET path length=%u\n", (unsigned int)path.length());
  client.print("GET ");
  client.print(path);
  client.print(" HTTP/1.1\r\nHost: ");
  client.print(host);
  client.print(":");
  client.print(port);
  client.print("\r\nUser-Agent: argus-esp32\r\nAccept: application/octet-stream\r\nConnection: close\r\n\r\n");

  String statusLine = client.readStringUntil('\n');
  statusLine.trim();
  int statusCode = parseHTTPStatusCode(statusLine);
  Serial.printf("[OTA] GET code=%d\n", statusCode);
  Serial.printf("[OTA] HTTP status line=%s\n", statusLine.c_str());

  int contentLength = -1;
  String redirectLocation;
  while (client.connected()) {
    String line = client.readStringUntil('\n');
    line.trim();
    if (line.length() == 0) break;

    String lower = line;
    lower.toLowerCase();
    if (lower.startsWith("content-length:")) {
      contentLength = line.substring(line.indexOf(':') + 1).toInt();
    } else if (lower.startsWith("location:")) {
      redirectLocation = line.substring(line.indexOf(':') + 1);
      redirectLocation.trim();
    }
  }

  if (statusCode >= 300 && statusCode < 400) {
    Serial.printf("[OTA] Firmware URL returned redirect to %s\n", redirectLocation.c_str());
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Unexpected firmware redirect");
  }
  if (statusCode != 200) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Download failed with HTTP code " + String(statusCode));
  }

  Serial.printf("[OTA] content length=%d\n", contentLength);
  if (contentLength <= 0) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Missing Content-Length");
  }
  if ((uint32_t)contentLength != manifest.sizeBytes) {
    Serial.printf("[OTA] Content-Length mismatch: manifest=%u http=%d\n",
                  (unsigned int)manifest.sizeBytes, contentLength);
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Content-Length mismatch");
  }

  Serial.println("[OTA] Beginning OTA partition write");
  if (!Update.begin(contentLength, U_FLASH)) {
    String error = "Update.begin failed: " + String(Update.getError());
    Serial.printf("[OTA] %s\n", error.c_str());
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, error);
  }
  updateStarted = true;
  publishOTAStatus(manifest.deploymentId, "downloading", 0, "Firmware download started");

  mbedtls_md_context_t mdCtx;
  mbedtls_md_init(&mdCtx);
  const mbedtls_md_info_t* mdInfo = mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);
  mbedtls_md_setup(&mdCtx, mdInfo, 0);
  mbedtls_md_starts(&mdCtx);

  uint8_t buffer[OTA_CHUNK_BYTES];
  size_t bytesReadTotal = 0;
  size_t bytesWrittenTotal = 0;
  int lastLoggedPercent = -1;
  unsigned long lastActivity = millis();
  Serial.printf("[OTA] initial stream available=%d\n", client.available());

  while (bytesReadTotal < (size_t)contentLength) {
    if (WiFi.status() != WL_CONNECTED) {
      mbedtls_md_free(&mdCtx);
      return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "WiFi disconnected during OTA");
    }

    if (mqtt.connected()) {
      mqtt.loop();
    }

    int available = client.available();
    if (available <= 0) {
      if (!client.connected()) {
        mbedtls_md_free(&mdCtx);
        return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "HTTP connection reset during OTA");
      }
      if (millis() - lastActivity > OTA_HTTP_TIMEOUT) {
        mbedtls_md_free(&mdCtx);
        return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Download timeout during OTA");
      }
      delay(1);
      yield();
      continue;
    }

    size_t remaining = (size_t)contentLength - bytesReadTotal;
    size_t toRead = min((size_t)available, min((size_t)OTA_CHUNK_BYTES, remaining));
    int got = client.readBytes(buffer, toRead);
    if (got <= 0) {
      delay(1);
      yield();
      continue;
    }

    lastActivity = millis();
    updateHash(mdCtx, buffer, got);
    bytesReadTotal += got;

    size_t wrote = Update.write(buffer, got);
    if (wrote != (size_t)got) {
      mbedtls_md_free(&mdCtx);
      return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Flash write failed");
    }
    bytesWrittenTotal += wrote;

    int percent = (int)((bytesWrittenTotal * 100UL) / (uint32_t)contentLength);
    if (percent >= lastLoggedPercent + 10 || percent == 100) {
      lastLoggedPercent = percent;
      Serial.printf("[OTA] Download/write progress: %d%% (%u/%d bytes)\n",
                    percent, (unsigned int)bytesWrittenTotal, contentLength);
      publishOTAStatus(manifest.deploymentId, percent < 75 ? "downloading" : "flashing", percent);
    }
    yield();
  }

  String calculated = finishHashHex(mdCtx);
  mbedtls_md_free(&mdCtx);
  Serial.printf("[OTA] Bytes read=%u bytes written=%u expected=%d\n",
                (unsigned int)bytesReadTotal, (unsigned int)bytesWrittenTotal, contentLength);
  Serial.printf("[OTA] Calculated SHA256: %s\n", calculated.c_str());
  Serial.printf("[OTA] Expected SHA256:   %s\n", manifest.checksumSha256.c_str());

  if (bytesReadTotal != (size_t)contentLength || bytesWrittenTotal != (size_t)contentLength) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Downloaded byte count mismatch");
  }
  if (!verifyFirmwareAuthenticity(manifest, calculated)) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Checksum verification failed");
  }

  publishOTAStatus(manifest.deploymentId, "flashing", 95, "Firmware verified");
  Serial.println("[OTA] Completing update with Update.end(true)");
  if (!Update.end(true)) {
    String error = "Update.end failed: " + String(Update.getError());
    Serial.printf("[OTA] %s\n", error.c_str());
    return failRawFirmwareDownload(client, clientStarted, false, manifest.deploymentId, error);
  }
  updateStarted = false;

  if (!Update.isFinished()) {
    return failRawFirmwareDownload(client, clientStarted, updateStarted, manifest.deploymentId, "Update not finished properly");
  }

  client.stop();
  clientStarted = false;
  logNetState("after OTA download");
  persistPendingOTAACK(manifest.deploymentId, manifest.version);
  publishOTAStatus(manifest.deploymentId, "rebooting", 100, "Firmware flashed; rebooting");
  Serial.println("[OTA] Flash complete. Pending ACK stored. Rebooting now.");
  delay(250);
  esp_restart();
  return true;
}

bool downloadVerifyAndFlash(const OTAManifest& manifest) {
  if (!otaPartitionCapable || !validateOTAPartitions()) {
    publishOTANack(manifest.deploymentId, "OTA partition unavailable");
    return false;
  }

  const esp_partition_t* next = esp_ota_get_next_update_partition(nullptr);
  if (next == nullptr || manifest.sizeBytes == 0 || manifest.sizeBytes > next->size) {
    Serial.printf("[OTA] Firmware size %u does not fit next OTA partition size %u\n",
                  (unsigned int)manifest.sizeBytes,
                  next == nullptr ? 0 : (unsigned int)next->size);
    publishOTANack(manifest.deploymentId, "Firmware does not fit OTA partition");
    return false;
  }

  if (isHttpUrl(manifest.downloadUrl)) {
    return downloadVerifyAndFlashPlainHTTP(manifest);
  }

  HTTPClient http;
  WiFiClient plainClient;
  WiFiClientSecure secureClient;
  bool httpStarted = false;
  bool updateStarted = false;

  Serial.printf("[OTA] Starting firmware download for deployment %s\n", manifest.deploymentId.c_str());
  Serial.printf("[OTA] begin firmware request url=%s\n", manifest.downloadUrl.c_str());

  if (!beginFirmwareHTTP(http, plainClient, secureClient, manifest.downloadUrl)) {
    Serial.println("[OTA] http.begin result=failed");
    plainClient.stop();
    secureClient.stop();
    logNetState("after OTA HTTPS begin failure");
    publishOTANack(manifest.deploymentId, "Failed to initialize firmware download");
    return false;
  }
  httpStarted = true;
  Serial.println("[OTA] http.begin result=ok");

  int httpCode = http.GET();
  Serial.printf("[OTA] GET code=%d\n", httpCode);
  Serial.printf("[OTA] HTTP error=%s\n", http.errorToString(httpCode).c_str());
  if (httpCode < 0) {
    logHTTPError(http, httpCode, secureClient, "OTA GET");
  }
  if (httpCode != HTTP_CODE_OK) {
    Serial.printf("[OTA] Firmware download failed with HTTP %d\n", httpCode);
    if (httpCode == HTTP_CODE_FORBIDDEN) {
      return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Download URL forbidden or expired");
    } else if (httpCode == HTTP_CODE_NOT_FOUND) {
      return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Firmware object not found");
    }
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Download failed with HTTP code " + String(httpCode));
  }

  int contentLength = http.getSize();
  Serial.printf("[OTA] content length=%d\n", contentLength);
  if (contentLength <= 0) {
    Serial.println("[OTA] Missing Content-Length; refusing OTA for safety");
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Missing Content-Length");
  }
  if ((uint32_t)contentLength != manifest.sizeBytes) {
    Serial.printf("[OTA] Content-Length mismatch: manifest=%u http=%d\n",
                  (unsigned int)manifest.sizeBytes, contentLength);
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Content-Length mismatch");
  }

  Serial.println("[OTA] Beginning OTA partition write");
  if (!Update.begin(contentLength, U_FLASH)) {
    String error = "Update.begin failed: " + String(Update.getError());
    Serial.printf("[OTA] %s\n", error.c_str());
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, error);
  }
  updateStarted = true;
  publishOTAStatus(manifest.deploymentId, "downloading", 0, "Firmware download started");

  mbedtls_md_context_t mdCtx;
  mbedtls_md_init(&mdCtx);
  const mbedtls_md_info_t* mdInfo = mbedtls_md_info_from_type(MBEDTLS_MD_SHA256);
  mbedtls_md_setup(&mdCtx, mdInfo, 0);
  mbedtls_md_starts(&mdCtx);

  WiFiClient* stream = http.getStreamPtr();
  if (stream == nullptr) {
    Serial.println("[OTA] HTTP stream pointer is null");
    mbedtls_md_free(&mdCtx);
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "HTTP stream unavailable");
  }
  Serial.printf("[OTA] initial stream available=%d\n", stream->available());

  uint8_t buffer[OTA_CHUNK_BYTES];
  size_t bytesReadTotal = 0;
  size_t bytesWrittenTotal = 0;
  int lastLoggedPercent = -1;
  unsigned long lastActivity = millis();

  while (bytesReadTotal < (size_t)contentLength) {
    if (WiFi.status() != WL_CONNECTED) {
      Serial.println("[OTA] WiFi disconnected during OTA");
      mbedtls_md_free(&mdCtx);
      return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "WiFi disconnected during OTA");
    }

    if (mqtt.connected()) {
      mqtt.loop();
    }

    int available = stream->available();
    if (available <= 0) {
      if (!http.connected()) {
        Serial.println("[OTA] HTTP connection reset during OTA");
        mbedtls_md_free(&mdCtx);
        return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "HTTP connection reset during OTA");
      }
      if (millis() - lastActivity > OTA_HTTP_TIMEOUT) {
        Serial.println("[OTA] Download timeout during OTA");
        mbedtls_md_free(&mdCtx);
        return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Download timeout during OTA");
      }
      delay(1);
      yield();
      continue;
    }

    size_t remaining = (size_t)contentLength - bytesReadTotal;
    size_t toRead = min((size_t)available, min((size_t)OTA_CHUNK_BYTES, remaining));
    int got = stream->readBytes(buffer, toRead);
    if (got <= 0) {
      delay(1);
      yield();
      continue;
    }

    lastActivity = millis();
    updateHash(mdCtx, buffer, got);
    bytesReadTotal += got;

    size_t wrote = Update.write(buffer, got);
    if (wrote != (size_t)got) {
      Serial.printf("[OTA] Flash write mismatch: read=%d wrote=%u\n", got, (unsigned int)wrote);
      mbedtls_md_free(&mdCtx);
      return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Flash write failed");
    }
    bytesWrittenTotal += wrote;

    int percent = (int)((bytesWrittenTotal * 100UL) / (uint32_t)contentLength);
    if (percent >= lastLoggedPercent + 10 || percent == 100) {
      lastLoggedPercent = percent;
      Serial.printf("[OTA] Download/write progress: %d%% (%u/%d bytes)\n",
                    percent, (unsigned int)bytesWrittenTotal, contentLength);
      if (percent < 75) {
        publishOTAStatus(manifest.deploymentId, "downloading", percent);
      } else {
        publishOTAStatus(manifest.deploymentId, "flashing", percent);
      }
    }

    yield();
  }

  String calculated = finishHashHex(mdCtx);
  mbedtls_md_free(&mdCtx);

  Serial.printf("[OTA] Bytes read=%u bytes written=%u expected=%d\n",
                (unsigned int)bytesReadTotal, (unsigned int)bytesWrittenTotal, contentLength);
  Serial.printf("[OTA] Calculated SHA256: %s\n", calculated.c_str());
  Serial.printf("[OTA] Expected SHA256:   %s\n", manifest.checksumSha256.c_str());

  if (bytesReadTotal != (size_t)contentLength || bytesWrittenTotal != (size_t)contentLength) {
    Serial.println("[OTA] Content-Length / byte count mismatch; aborting");
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Downloaded byte count mismatch");
  }

  if (!verifyFirmwareAuthenticity(manifest, calculated)) {
    Serial.println("[OTA] Authenticity verification failed; aborting before Update.end()");
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Checksum verification failed");
  }

  publishOTAStatus(manifest.deploymentId, "flashing", 95, "Firmware verified");
  Serial.println("[OTA] Completing update with Update.end(true)");
  if (!Update.end(true)) {
    String error = "Update.end failed: " + String(Update.getError());
    Serial.printf("[OTA] %s\n", error.c_str());
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, error);
  }
  updateStarted = false;

  if (!Update.isFinished()) {
    Serial.println("[OTA] Update did not finish cleanly");
    return failFirmwareDownload(http, httpStarted, plainClient, secureClient, updateStarted, manifest.deploymentId, "Update not finished properly");
  }

  http.end();
  plainClient.stop();
  secureClient.stop();
  httpStarted = false;
  persistPendingOTAACK(manifest.deploymentId, manifest.version);
  publishOTAStatus(manifest.deploymentId, "rebooting", 100, "Firmware flashed; rebooting");
  Serial.println("[OTA] Flash complete. Pending ACK stored. Rebooting now.");
  delay(250);
  esp_restart();
  return true;
}

void checkOTA() {
  logNetState("before OTA poll");
  if (otaInProgress) {
    Serial.println("[OTA] Poll skipped because OTA is already in progress");
    logNetState("after OTA poll skipped");
    return;
  }
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("[OTA] Poll skipped because WiFi is disconnected");
    logNetState("after OTA poll skipped");
    return;
  }
  if (loadPendingOTAACK().needsAck) {
    Serial.println("[OTA] Poll skipped while pending ACK is being retried");
    logNetState("after OTA poll skipped");
    return;
  }

  otaInProgress = true;
  Serial.printf("[OTA] Polling pending deployment for device %s\n", ARGUS_DEVICE_ID);

  int httpCode = 0;
  String response = httpGet(String("/api/devices/") + ARGUS_DEVICE_ID + "/ota/pending", httpCode);

  if (httpCode == HTTP_CODE_OK) {
    OTAManifest manifest;
    if (!parseOTAManifest(response, manifest)) {
      Serial.println("[OTA] Invalid manifest; continuing normal operation");
      otaInProgress = false;
      logNetState("after OTA poll invalid manifest");
      return;
    }
    if (manifestExpired(manifest)) {
      publishOTANack(manifest.deploymentId, "Manifest expired");
      otaInProgress = false;
      logNetState("after OTA poll expired manifest");
      return;
    }
    if (!versionAllowed(manifest)) {
      logNetState("after OTA manifest expired");
      return;
    }

    if (!versionAllowed(manifest.version, manifest.allowDowngrade)) {
      otaInProgress = false;
      logNetState("after OTA version rejected");
      return;
    }

    Serial.printf("[OTA] Initiating download for version %s\n", manifest.version.c_str());
    bool success = downloadVerifyAndFlash(manifest);
    if (!success) {
      Serial.println("[OTA] Download/Install phase failed; keeping running firmware");
    }
    otaInProgress = false;
    logNetState("after OTA poll");
    return;
  }

  Serial.printf("[OTA] Pending deployment request failed with HTTP %d; continuing normal operation\n", httpCode);
  otaInProgress = false;
  logNetState("after OTA poll failure");
}

}
