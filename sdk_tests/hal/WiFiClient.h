#ifndef WIFI_CLIENT_H
#define WIFI_CLIENT_H

#include "Client.h"

class WiFiClient : public Client {
protected:
    int _fd;
public:
    WiFiClient();
    virtual ~WiFiClient();

    virtual int connect(IPAddress ip, uint16_t port) override;
    virtual int connect(const char *host, uint16_t port) override;
    virtual size_t write(uint8_t) override;
    virtual size_t write(const uint8_t *buf, size_t size) override;
    virtual int available() override;
    virtual int read() override;
    virtual int read(uint8_t *buf, size_t size) override;
    virtual int peek() override;
    virtual void flush() override;
    virtual void stop() override;
    virtual uint8_t connected() override;

    using Print::write;
};

#endif // WIFI_CLIENT_H
