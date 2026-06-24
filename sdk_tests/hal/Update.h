#ifndef UPDATE_H
#define UPDATE_H

#include "Arduino.h"

#define U_FLASH 0

#include <vector>
#include <cstring>

extern std::vector<uint8_t> g_otaReceivedBytes;
extern bool g_otaFlashAttempted;

class UpdateClass {
public:
    bool begin(size_t size, int command = U_FLASH) {
        g_otaReceivedBytes.clear();
        g_otaReceivedBytes.reserve(size);
        g_otaFlashAttempted = true;
        return true;
    }
    size_t write(uint8_t* data, size_t len) {
        g_otaReceivedBytes.insert(g_otaReceivedBytes.end(), data, data + len);
        return len;
    }
    bool end(bool evenIfRemaining = false) { return true; }
    int getError() { return 0; }
    bool isFinished() { return true; }
    void abort() {}
};

extern UpdateClass Update;

#endif // UPDATE_H
