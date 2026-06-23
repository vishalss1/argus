#ifndef WIFI_CLIENT_SECURE_H
#define WIFI_CLIENT_SECURE_H

#include "WiFiClient.h"
#include <openssl/ssl.h>

class WiFiClientSecure : public WiFiClient {
protected:
    SSL_CTX* _sslCtx;
    SSL* _ssl;
    const char* _CA_cert;
    const char* _cert;
    const char* _private_key;

    int connectSSL(const char* host, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key);
public:
    WiFiClientSecure();
    virtual ~WiFiClientSecure();

    void setCACert(const char* ca) { _CA_cert = ca; }
    void setCertificate(const char* cert) { _cert = cert; }
    void setPrivateKey(const char* key) { _private_key = key; }

    virtual int connect(IPAddress ip, uint16_t port) override;
    virtual int connect(const char *host, uint16_t port) override;
    
    // Additional overloads needed
    virtual int connect(const char* host, uint16_t port, int32_t timeout);
    virtual int connect(IPAddress ip, uint16_t port, int32_t timeout);
    int connect(IPAddress ip, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key);
    int connect(const char* host, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key);

    virtual size_t write(uint8_t c) override { return write(&c, 1); }
    virtual size_t write(const uint8_t *buf, size_t size) override;
    virtual int available() override;
    virtual int read() override { uint8_t c; return (read(&c, 1) > 0) ? c : -1; }
    virtual int read(uint8_t *buf, size_t size) override;
    virtual int peek() override;
    virtual void stop() override;
    virtual uint8_t connected() override;

    bool getFingerprintSHA256(uint8_t out[32]);
    void lastError(char* buf, size_t len);

    using Print::write;
};

#endif // WIFI_CLIENT_SECURE_H
