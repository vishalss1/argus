#include "argus_security.h"
#include "argus_config.h"
#include <mbedtls/base64.h>

namespace argus_sdk {

bool hasConfiguredRootCA() {
  String ca = String(ARGUS_ROOT_CA);
  ca.trim();
  return ca.length() > 0;
}

bool decodeBase64Strict(const String& encoded, uint8_t* out, size_t outSize, size_t& outLen) {
  outLen = 0;
  String clean = encoded;
  clean.trim();
  if (clean.length() == 0) return false;
  int rc = mbedtls_base64_decode(out, outSize, &outLen, (const unsigned char*)clean.c_str(), clean.length());
  return rc == 0;
}

String normalizedHex(String value) {
  value.toLowerCase();
  value.replace(":", "");
  value.replace(" ", "");
  value.trim();
  return value;
}

String bytesToHex(const uint8_t* bytes, size_t len) {
  char* out = (char*)malloc((len * 2) + 1);
  if (out == nullptr) return "";
  for (size_t i = 0; i < len; i++) {
    sprintf(&out[i * 2], "%02x", bytes[i]);
  }
  out[len * 2] = '\0';
  String result(out);
  free(out);
  return result;
}

bool verifyCertificatePin(WiFiClientSecure& client, const char* phase) {
  String expected = normalizedHex(String(ARGUS_PINNED_CERT_SHA256));
  String expectedNext = normalizedHex(String(ARGUS_PINNED_CERT_SHA256_NEXT));
  
  if (expected.length() == 0) {
    Serial.printf("[TLS] %s certificate pin not configured; CA validation only\n", phase);
    return true;
  }
  if (expected.length() != 64) {
    Serial.printf("[TLS] %s primary certificate pin invalid length=%u\n", phase, (unsigned int)expected.length());
    return false;
  }

  uint8_t fingerprint[32];
  if (!client.getFingerprintSHA256(fingerprint)) {
    Serial.printf("[TLS] %s could not read peer certificate fingerprint\n", phase);
    return false;
  }

  String actual = bytesToHex(fingerprint, sizeof(fingerprint));
  bool match = actual.equalsIgnoreCase(expected) || (expectedNext.length() == 64 && actual.equalsIgnoreCase(expectedNext));
  
  if (match) {
    Serial.printf("[TLS] %s certificate pin matched (actual=%s)\n", phase, actual.c_str());
  } else {
    Serial.printf("[TLS] %s certificate pin mismatch (actual=%s, expected1=%s, expected2=%s)\n",
                  phase, actual.c_str(), expected.c_str(), expectedNext.c_str());
  }
  return match;
}

void configureTLS(WiFiClientSecure& client) {
  client.setCACert(ARGUS_ROOT_CA);
  if (hasDeviceCredentials()) {
    client.setCertificate(ARGUS_DEVICE_CERT);
    client.setPrivateKey(ARGUS_DEVICE_PRIVATE_KEY);
  }
}

bool hasDeviceCredentials() {
  bool hasCert = (String(ARGUS_DEVICE_CERT).indexOf("-----BEGIN CERTIFICATE-----") >= 0);
  bool hasKey = (String(ARGUS_DEVICE_PRIVATE_KEY).indexOf("-----BEGIN PRIVATE KEY-----") >= 0 ||
                 String(ARGUS_DEVICE_PRIVATE_KEY).indexOf("-----BEGIN EC PRIVATE KEY-----") >= 0);
  return hasCert && hasKey;
}

}
