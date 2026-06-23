#pragma once
#include <WiFiClientSecure.h>

namespace argus_sdk {

#define ARGUS_PINNED_CERT_SHA256 "219E6C1E7F26D9FE3905A6F6DE237BC1DE3CED80ACD56D6C7A04AC9200C99091"
#define ARGUS_PINNED_CERT_SHA256_NEXT ""
#define ARGUS_REQUIRE_FIRMWARE_SIGNATURES true

bool hasConfiguredRootCA();
bool decodeBase64Strict(const String& encoded, uint8_t* out, size_t outSize, size_t& outLen);
String normalizedHex(String value);
String bytesToHex(const uint8_t* bytes, size_t len);
bool verifyCertificatePin(WiFiClientSecure& client, const char* phase);
void configureTLS(WiFiClientSecure& client);
bool hasDeviceCredentials();

class ArgusWiFiClientSecure : public WiFiClientSecure {
private:
  const char* getVerifyHost(const char* host) {
    IPAddress ip;
    if (ip.fromString(host)) {
      return "localhost";
    }
    return host;
  }

public:
  int connect(IPAddress ip, uint16_t port) override {
    return WiFiClientSecure::connect(ip, port, "localhost", _CA_cert, _cert, _private_key);
  }
  
  int connect(IPAddress ip, uint16_t port, int32_t timeout) override {
    _timeout = timeout;
    return WiFiClientSecure::connect(ip, port, "localhost", _CA_cert, _cert, _private_key);
  }
  
  int connect(const char *host, uint16_t port) override {
    IPAddress ip;
    const char* verifyHost = getVerifyHost(host);
    if (ip.fromString(host)) {
      return WiFiClientSecure::connect(ip, port, verifyHost, _CA_cert, _cert, _private_key);
    }
    return WiFiClientSecure::connect(host, port, verifyHost, _CA_cert, _cert, _private_key);
  }
  
  int connect(const char *host, uint16_t port, int32_t timeout) override {
    _timeout = timeout;
    IPAddress ip;
    const char* verifyHost = getVerifyHost(host);
    if (ip.fromString(host)) {
      return WiFiClientSecure::connect(ip, port, verifyHost, _CA_cert, _cert, _private_key);
    }
    return WiFiClientSecure::connect(host, port, verifyHost, _CA_cert, _cert, _private_key);
  }
};

}
