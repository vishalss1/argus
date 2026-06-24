#ifndef UPDATE_H
#define UPDATE_H

#include "Arduino.h"

#define U_FLASH 0

class UpdateClass {
public:
    bool begin(size_t size, int command = U_FLASH) { return true; }
    size_t write(uint8_t* data, size_t len) { return len; }
    bool end(bool evenIfRemaining = false) { return true; }
    int getError() { return 0; }
    bool isFinished() { return true; }
    void abort() {}
};

extern UpdateClass Update;

#endif // UPDATE_H
