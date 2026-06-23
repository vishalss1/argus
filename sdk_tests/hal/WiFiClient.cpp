#include "WiFiClient.h"
#include <iostream>

#ifdef _WIN32
#include <winsock2.h>
#include <ws2tcpip.h>
#define ioctl ioctlsocket
#define close closesocket
typedef int socklen_t;
static void initWinsock() {
    static bool initialized = false;
    if (!initialized) {
        WSADATA wsa;
        WSAStartup(MAKEWORD(2,2), &wsa);
        initialized = true;
    }
}
#else
#include <sys/socket.h>
#include <netinet/in.h>
#include <arpa/inet.h>
#include <netdb.h>
#include <unistd.h>
#include <sys/ioctl.h>
#include <poll.h>
#define initWinsock()
#endif

WiFiClient::WiFiClient() : _fd(-1) {
    initWinsock();
}

WiFiClient::~WiFiClient() {
    stop();
}

int WiFiClient::connect(IPAddress ip, uint16_t port) {
    return connect(ip.toString().c_str(), port);
}

int WiFiClient::connect(const char *host, uint16_t port) {
    stop();

    struct addrinfo hints, *result = nullptr, *rp = nullptr;
    std::memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;

    String portStr(port);
    if (getaddrinfo(host, portStr.c_str(), &hints, &result) != 0) {
        return 0;
    }

    for (rp = result; rp != nullptr; rp = rp->ai_next) {
        _fd = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (_fd < 0) continue;

        if (::connect(_fd, rp->ai_addr, rp->ai_addrlen) == 0) {
            break;
        }

        close(_fd);
        _fd = -1;
    }

    freeaddrinfo(result);

    if (_fd < 0) {
        return 0;
    }

    // Set socket receive timeout
#ifdef _WIN32
    DWORD timeout = _timeout;
    setsockopt(_fd, SOL_SOCKET, SO_RCVTIMEO, (const char*)&timeout, sizeof(timeout));
#else
    struct timeval tv;
    tv.tv_sec = _timeout / 1000;
    tv.tv_usec = (_timeout % 1000) * 1000;
    setsockopt(_fd, SOL_SOCKET, SO_RCVTIMEO, (const void*)&tv, sizeof(tv));
#endif

    return 1;
}

size_t WiFiClient::write(uint8_t c) {
    return write(&c, 1);
}

size_t WiFiClient::write(const uint8_t *buf, size_t size) {
    if (_fd < 0) return 0;
    int sent = send(_fd, (const char*)buf, size, 0);
    return sent > 0 ? sent : 0;
}

int WiFiClient::available() {
    if (_fd < 0) return 0;
    unsigned long bytes = 0;
    if (ioctl(_fd, FIONREAD, &bytes) < 0) {
        return 0;
    }
    return bytes;
}

int WiFiClient::read() {
    uint8_t c;
    if (read(&c, 1) > 0) {
        return c;
    }
    return -1;
}

int WiFiClient::read(uint8_t *buf, size_t size) {
    if (_fd < 0) return -1;
    int received = recv(_fd, (char*)buf, size, 0);
    if (received < 0) {
#ifdef _WIN32
        int err = WSAGetLastError();
        if (err == WSAETIMEDOUT || err == WSAEWOULDBLOCK) return 0; // timeout
#else
        if (errno == EAGAIN || errno == EWOULDBLOCK) return 0; // timeout
#endif
        return -1;
    }
    return received;
}

int WiFiClient::peek() {
    if (_fd < 0) return -1;
    char c;
    int received = recv(_fd, &c, 1, MSG_PEEK);
    if (received > 0) return (uint8_t)c;
    return -1;
}

void WiFiClient::flush() {
    // TCP flush is a no-op
}

void WiFiClient::stop() {
    if (_fd >= 0) {
        close(_fd);
        _fd = -1;
    }
}

uint8_t WiFiClient::connected() {
    if (_fd < 0) return 0;
    
    // Check if socket is still open by doing a MSG_PEEK
    char c;
    int res = recv(_fd, &c, 1, MSG_PEEK);
    if (res == 0) {
        // EOF
        stop();
        return 0;
    } else if (res < 0) {
#ifdef _WIN32
        int err = WSAGetLastError();
        if (err != WSAEWOULDBLOCK && err != WSAETIMEDOUT) {
            stop();
            return 0;
        }
#else
        if (errno != EAGAIN && errno != EWOULDBLOCK) {
            stop();
            return 0;
        }
#endif
    }
    return 1;
}
