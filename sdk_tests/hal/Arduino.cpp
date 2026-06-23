#include "Arduino.h"
#include <algorithm>
#include <thread>
#include <sstream>
#include <iomanip>
#include <cstdarg>

std::ostringstream g_serialCapture;
HardwareSerial Serial;
EspClass ESP;

// String numeric constructors
String::String(int num) {
    val = std::to_string(num);
}
String::String(unsigned int num) {
    val = std::to_string(num);
}
String::String(long num) {
    val = std::to_string(num);
}
String::String(unsigned long num) {
    val = std::to_string(num);
}
String::String(float num) {
    val = std::to_string(num);
}
String::String(double num) {
    val = std::to_string(num);
}

void String::trim() {
    val.erase(val.begin(), std::find_if(val.begin(), val.end(), [](unsigned char ch) {
        return !std::isspace(ch);
    }));
    val.erase(std::find_if(val.rbegin(), val.rend(), [](unsigned char ch) {
        return !std::isspace(ch);
    }).base(), val.end());
}

void String::toLowerCase() {
    std::transform(val.begin(), val.end(), val.begin(), [](unsigned char c) {
        return std::tolower(c);
    });
}

void String::replace(char find, char replace) {
    std::replace(val.begin(), val.end(), find, replace);
}

void String::replace(const String& find, const String& replace) {
    if (find.val.empty()) return;
    size_t pos = 0;
    while ((pos = val.find(find.val, pos)) != std::string::npos) {
        val.replace(pos, find.val.length(), replace.val);
        pos += replace.val.length();
    }
}

bool String::startsWith(const String& prefix) const {
    if (prefix.val.length() > val.length()) return false;
    return val.compare(0, prefix.val.length(), prefix.val) == 0;
}

int String::indexOf(char c, unsigned int fromIndex) const {
    if (fromIndex >= val.length()) return -1;
    size_t pos = val.find(c, fromIndex);
    return (pos == std::string::npos) ? -1 : static_cast<int>(pos);
}

int String::indexOf(const String& str, unsigned int fromIndex) const {
    if (fromIndex >= val.length()) return -1;
    size_t pos = val.find(str.val, fromIndex);
    return (pos == std::string::npos) ? -1 : static_cast<int>(pos);
}

int String::lastIndexOf(char c) const {
    size_t pos = val.rfind(c);
    return (pos == std::string::npos) ? -1 : static_cast<int>(pos);
}

String String::substring(unsigned int left, unsigned int right) const {
    if (left > val.length()) return String("");
    if (right > val.length()) right = val.length();
    if (left > right) return String("");
    return String(val.substr(left, right - left));
}

long String::toInt() const {
    try {
        return std::stol(val);
    } catch (...) {
        return 0;
    }
}

bool String::equalsIgnoreCase(const String& s2) const {
    if (val.length() != s2.val.length()) return false;
    return std::equal(val.begin(), val.end(), s2.val.begin(), [](unsigned char a, unsigned char b) {
        return std::tolower(a) == std::tolower(b);
    });
}

void String::remove(unsigned int index, unsigned int count) {
    if (index >= val.length()) return;
    val.erase(index, count);
}

// Print class
size_t Print::write(const uint8_t *buffer, size_t size) {
    size_t n = 0;
    while (size--) {
        if (write(*buffer++)) n++;
        else break;
    }
    return n;
}

size_t Print::print(const String &s) {
    return write((const uint8_t*)s.c_str(), s.length());
}

size_t Print::print(const char str[]) {
    return write((const uint8_t*)str, strlen(str));
}

size_t Print::print(char c) {
    return write(c);
}

size_t Print::print(int n, int base) {
    return print(static_cast<long>(n), base);
}

size_t Print::print(unsigned int n, int base) {
    return print(static_cast<unsigned long>(n), base);
}

size_t Print::print(long n, int base) {
    if (base == 10) {
        return print(String(n));
    } else {
        std::ostringstream ss;
        ss << std::hex << n;
        return print(String(ss.str()));
    }
}

size_t Print::print(unsigned long n, int base) {
    if (base == 10) {
        return print(String(n));
    } else {
        std::ostringstream ss;
        ss << std::hex << n;
        return print(String(ss.str()));
    }
}

size_t Print::print(double n, int digits) {
    std::ostringstream ss;
    ss << std::fixed << std::setprecision(digits) << n;
    return print(String(ss.str()));
}

size_t Print::println(const String &s) {
    size_t n = print(s);
    n += println();
    return n;
}

size_t Print::println(const char str[]) {
    size_t n = print(str);
    n += println();
    return n;
}

size_t Print::println(char c) {
    size_t n = print(c);
    n += println();
    return n;
}

size_t Print::println(int n, int base) {
    size_t n_chars = print(n, base);
    n_chars += println();
    return n_chars;
}

size_t Print::println(unsigned int n, int base) {
    size_t n_chars = print(n, base);
    n_chars += println();
    return n_chars;
}

size_t Print::println(long n, int base) {
    size_t n_chars = print(n, base);
    n_chars += println();
    return n_chars;
}

size_t Print::println(unsigned long n, int base) {
    size_t n_chars = print(n, base);
    n_chars += println();
    return n_chars;
}

size_t Print::println(double n, int digits) {
    size_t n_chars = print(n, digits);
    n_chars += println();
    return n_chars;
}

size_t Print::println(void) {
    return write('\n');
}

size_t Print::printf(const char * format, ...) {
    char loc_buf[256];
    char * temp = loc_buf;
    va_list arg;
    va_list copy;
    va_start(arg, format);
    va_copy(copy, arg);
    int len = vsnprintf(temp, sizeof(loc_buf), format, temp == loc_buf ? arg : copy);
    va_end(copy);
    if (len < 0) {
        va_end(arg);
        return 0;
    }
    if (len >= static_cast<int>(sizeof(loc_buf))) {
        temp = (char*)malloc(len + 1);
        if (temp == nullptr) {
            va_end(arg);
            return 0;
        }
        len = vsnprintf(temp, len + 1, format, arg);
    }
    va_end(arg);
    size_t n = write((const uint8_t*)temp, len);
    if (temp != loc_buf) {
        free(temp);
    }
    return n;
}

// Stream class
int Stream::timedRead() {
    int c;
    _startMillis = millis();
    do {
        c = read();
        if (c >= 0) return c;
    } while (millis() - _startMillis < _timeout);
    return -1; // timeout
}

size_t Stream::readBytes(char *buffer, size_t length) {
    size_t count = 0;
    while (count < length) {
        int c = timedRead();
        if (c < 0) break;
        *buffer++ = (char)c;
        count++;
    }
    return count;
}

String Stream::readStringUntil(char terminator) {
    String ret;
    while (true) {
        int c = timedRead();
        if (c < 0) break;
        if (c == terminator) break;
        ret += (char)c;
    }
    return ret;
}

// IPAddress class
IPAddress::IPAddress() {
    std::memset(_address, 0, sizeof(_address));
}

IPAddress::IPAddress(uint8_t first_octet, uint8_t second_octet, uint8_t third_octet, uint8_t fourth_octet) {
    _address[0] = first_octet;
    _address[1] = second_octet;
    _address[2] = third_octet;
    _address[3] = fourth_octet;
}

IPAddress::IPAddress(uint32_t address) {
    _address[0] = (address >> 24) & 0xFF;
    _address[1] = (address >> 16) & 0xFF;
    _address[2] = (address >> 8) & 0xFF;
    _address[3] = address & 0xFF;
}

IPAddress::IPAddress(const uint8_t *address) {
    std::memcpy(_address, address, sizeof(_address));
}

bool IPAddress::fromString(const char *address) {
    int a, b, c, d;
    if (sscanf(address, "%d.%d.%d.%d", &a, &b, &c, &d) == 4) {
        if (a >= 0 && a <= 255 && b >= 0 && b <= 255 && c >= 0 && c <= 255 && d >= 0 && d <= 255) {
            _address[0] = a;
            _address[1] = b;
            _address[2] = c;
            _address[3] = d;
            return true;
        }
    }
    return false;
}

String IPAddress::toString() const {
    std::ostringstream ss;
    ss << (int)_address[0] << "." << (int)_address[1] << "." << (int)_address[2] << "." << (int)_address[3];
    return String(ss.str());
}

// HardwareSerial
size_t HardwareSerial::write(uint8_t c) {
    std::cout.put(c);
    g_serialCapture.put(c);
    return 1;
}

size_t HardwareSerial::write(const uint8_t *buffer, size_t size) {
    std::cout.write((const char*)buffer, size);
    g_serialCapture.write((const char*)buffer, size);
    return size;
}

// EspClass restart
void EspClass::restart() {
    std::exit(0);
}

// Global millisecond clock
unsigned long millis() {
    static const auto start = std::chrono::steady_clock::now();
    auto now = std::chrono::steady_clock::now();
    return std::chrono::duration_cast<std::chrono::milliseconds>(now - start).count();
}

void delay(unsigned long ms) {
    std::this_thread::sleep_for(std::chrono::milliseconds(ms));
}
