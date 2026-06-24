#ifndef WIFI_H
#define WIFI_H

#include "Arduino.h"

typedef int wifi_mode_t;

class WiFiClass {
public:
    int status() { return WL_CONNECTED; }
    IPAddress localIP() { return IPAddress(127, 0, 0, 1); }
    int RSSI() { return -60; }
    void mode(wifi_mode_t m) {}
    void begin(const char* ssid, const char* passphrase = nullptr) {}
    void disconnect() {}
};

extern WiFiClass WiFi;

#endif // WIFI_H
