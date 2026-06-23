#ifndef ARDUINO_H
#define ARDUINO_H

#include <string>
#include <iostream>
#include <sstream>
#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <cstring>
#include <cctype>

#include <algorithm>
#include <type_traits>

#ifdef _WIN32
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <winsock2.h>
#include <windows.h>
#include <ctime>
inline struct tm* gmtime_r(const time_t* timep, struct tm* result) {
    if (gmtime_s(result, timep) == 0) {
        return result;
    }
    return nullptr;
}
#endif

// Basic types
typedef uint8_t byte;
#define PROGMEM
#define F(x) x
#define pgm_read_byte_near(addr) (*(const uint8_t*)(addr))
#define pgm_read_byte(addr) (*(const uint8_t*)(addr))

#ifndef min
template<typename T, typename U>
inline auto min(T a, U b) -> typename std::common_type<T, U>::type {
    return (a < b) ? a : b;
}
#endif

#ifndef max
template<typename T, typename U>
inline auto max(T a, U b) -> typename std::common_type<T, U>::type {
    return (a > b) ? a : b;
}
#endif

inline bool isDigit(char c) {
    return std::isdigit(static_cast<unsigned char>(c)) != 0;
}

// Arduino String class wrapping std::string
class String {
private:
    std::string val;
    mutable size_t read_pos = 0;
public:
    String() : val("") {}
    String(const char *str) : val(str ? str : "") {}
    String(const std::string &str) : val(str) {}
    String(char c) : val(1, c) {}
    String(unsigned char c) : val(1, static_cast<char>(c)) {}
    String(int num);
    String(unsigned int num);
    String(long num);
    String(unsigned long num);
    String(float num);
    String(double num);

    const char* c_str() const { return val.c_str(); }
    unsigned int length() const { return val.length(); }
    
    void trim();
    void toLowerCase();
    void replace(char find, char replace);
    void replace(const String& find, const String& replace);
    bool startsWith(const String& prefix) const;
    int indexOf(char c, unsigned int fromIndex = 0) const;
    int indexOf(const String& str, unsigned int fromIndex = 0) const;
    int lastIndexOf(char c) const;
    String substring(unsigned int left, unsigned int right = -1) const;
    long toInt() const;
    bool equalsIgnoreCase(const String& s2) const;
    void remove(unsigned int index, unsigned int count = -1);
    void reserve(unsigned int size) { val.reserve(size); }
    size_t write(uint8_t c) {
        val.push_back(static_cast<char>(c));
        return 1;
    }
    size_t write(const uint8_t *s, size_t n) {
        val.append(reinterpret_cast<const char*>(s), n);
        return n;
    }
    int read() const {
        if (read_pos < val.length()) {
            return static_cast<unsigned char>(val[read_pos++]);
        }
        return -1;
    }

    String& operator+=(const String& rhs) { val += rhs.val; return *this; }
    String& operator+=(const char* rhs) { val += rhs; return *this; }
    String& operator+=(char c) { val += c; return *this; }
    
    friend String operator+(String lhs, const String& rhs) { lhs.val += rhs.val; return lhs; }
    friend String operator+(String lhs, const char* rhs) { lhs.val += rhs; return lhs; }
    friend String operator+(String lhs, char c) { lhs.val += c; return lhs; }

    bool operator==(const String& rhs) const { return val == rhs.val; }
    bool operator==(const char* rhs) const { return val == rhs; }
    bool operator!=(const String& rhs) const { return val != rhs.val; }
    bool operator!=(const char* rhs) const { return val != rhs; }
    bool operator<(const String& rhs) const { return val < rhs.val; }
    bool operator>(const String& rhs) const { return val > rhs.val; }

    char operator[](unsigned int index) const {
        if (index < val.length()) return val[index];
        return 0;
    }
    char& operator[](unsigned int index) {
        static char dummy = 0;
        if (index < val.length()) return val[index];
        return dummy;
    }
};

// Print base class
class Print {
public:
    virtual size_t write(uint8_t) = 0;
    virtual size_t write(const uint8_t *buffer, size_t size);
    size_t print(const String &);
    size_t print(const char[]);
    size_t print(char);
    size_t print(int, int = 10);
    size_t print(unsigned int, int = 10);
    size_t print(long, int = 10);
    size_t print(unsigned long, int = 10);
    size_t print(double, int = 2);
    size_t println(const String &s);
    size_t println(const char[]);
    size_t println(char);
    size_t println(int, int = 10);
    size_t println(unsigned int, int = 10);
    size_t println(long, int = 10);
    size_t println(unsigned long, int = 10);
    size_t println(double, int = 2);
    size_t println(void);
    size_t printf(const char * format, ...) __attribute__ ((format (printf, 2, 3)));
};

// Stream base class
class Stream : public Print {
protected:
    unsigned long _timeout = 1000;
    unsigned long _startMillis = 0;
    int timedRead();
public:
    virtual int available() = 0;
    virtual int read() = 0;
    virtual int peek() = 0;
    virtual void flush() = 0;
    void setTimeout(unsigned long timeout) { _timeout = timeout; }
    size_t readBytes(char *buffer, size_t length);
    size_t readBytes(uint8_t *buffer, size_t length) { return readBytes((char*)buffer, length); }
    String readStringUntil(char terminator);
};

// IPAddress class
class IPAddress {
private:
    uint8_t _address[4];
public:
    IPAddress();
    IPAddress(uint8_t first_octet, uint8_t second_octet, uint8_t third_octet, uint8_t fourth_octet);
    IPAddress(uint32_t address);
    IPAddress(const uint8_t *address);
    bool fromString(const char *address);
    bool fromString(const String &address) { return fromString(address.c_str()); }
    String toString() const;
    uint8_t operator[](int index) const { return _address[index]; }
    uint8_t& operator[](int index) { return _address[index]; }
    operator uint32_t() const {
        return ((uint32_t)_address[0] << 24) | ((uint32_t)_address[1] << 16) | ((uint32_t)_address[2] << 8) | _address[3];
    }
};

// HardwareSerial class
extern std::ostringstream g_serialCapture;

class HardwareSerial : public Stream {
public:
    void begin(unsigned long baud) {}
    size_t write(uint8_t c) override;
    size_t write(const uint8_t *buffer, size_t size) override;
    int available() override { return 0; }
    int read() override { return -1; }
    int peek() override { return -1; }
    void flush() override {}
};

extern HardwareSerial Serial;

// EspClass
class EspClass {
public:
    uint32_t getFreeHeap() { return 262144; }
    uint32_t getMaxAllocHeap() { return 131072; }
    uint32_t getSketchSize() { return 1048576; }
    uint32_t getFreeSketchSpace() { return 1048576; }
    void restart();
};

extern EspClass ESP;

// Time and Delay
unsigned long millis();
void delay(unsigned long ms);
inline void yield() {}

inline void pinMode(uint8_t pin, uint8_t mode) {}
inline void digitalWrite(uint8_t pin, uint8_t val) {}

// Constants
#define OUTPUT 1
#ifndef _WIN32
#define INPUT 0
#endif
#define HIGH 1
#define LOW 0
#define LED_BUILTIN 2
#define WIFI_STA 1
#define WL_CONNECTED 3
#define WL_DISCONNECTED 6

inline void configTime(long gmtOffset_sec, int daylightOffset_sec, const char* server1, const char* server2 = nullptr, const char* server3 = nullptr) {}

inline long random(long min_val, long max_val) {
    if (min_val >= max_val) return min_val;
    return min_val + (std::rand() % (max_val - min_val));
}
inline long random(long max_val) {
    return random(0, max_val);
}

#ifdef _WIN32
inline int setenv(const char *name, const char *value, int overwrite) {
    if (!overwrite) {
        if (std::getenv(name)) return 0;
    }
    return _putenv_s(name, value);
}
#endif

#endif // ARDUINO_H
