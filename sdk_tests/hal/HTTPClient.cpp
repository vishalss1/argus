#include "HTTPClient.h"
#include <sstream>
#include <iostream>

HTTPClient::HTTPClient() : _client(nullptr), _port(80), _contentLength(-1), _timeout(5000) {
}

HTTPClient::~HTTPClient() {
    end();
}

void HTTPClient::parseUrl(const String& url) {
    std::string s = url.c_str();
    size_t pos = s.find("://");
    if (pos == std::string::npos) return;
    _scheme = s.substr(0, pos).c_str();
    size_t host_start = pos + 3;
    size_t path_start = s.find('/', host_start);
    std::string host_port;
    if (path_start == std::string::npos) {
        host_port = s.substr(host_start);
        _path = "/";
    } else {
        host_port = s.substr(host_start, path_start - host_start);
        _path = s.substr(path_start).c_str();
    }
    size_t colon = host_port.find(':');
    if (colon == std::string::npos) {
        _host = host_port.c_str();
        _port = (_scheme == "https") ? 443 : 80;
    } else {
        _host = host_port.substr(0, colon).c_str();
        try {
            _port = std::stoi(host_port.substr(colon + 1));
        } catch (...) {
            _port = (_scheme == "https") ? 443 : 80;
        }
    }
}

bool HTTPClient::begin(WiFiClient& client, String url) {
    _client = &client;
    parseUrl(url);
    return true;
}

bool HTTPClient::begin(WiFiClientSecure& client, String url) {
    _client = &client;
    parseUrl(url);
    return true;
}

void HTTPClient::addHeader(const String& name, const String& value) {
    _headers.push_back(std::make_pair(name, value));
}

void HTTPClient::setTimeout(uint16_t timeout) {
    _timeout = timeout;
    if (_client) {
        _client->setTimeout(timeout);
    }
}

int HTTPClient::GET() {
    return sendRequest("GET");
}

int HTTPClient::sendRequest(const char* method, const String& body) {
    if (!_client) return HTTPC_ERROR_NOT_CONNECTED;
    
    if (!_client->connected()) {
        if (!_client->connect(_host.c_str(), _port)) {
            return HTTPC_ERROR_CONNECTION_REFUSED;
        }
    }

    // Build request
    std::ostringstream ss;
    ss << method << " " << _path.c_str() << " HTTP/1.1\r\n";
    ss << "Host: " << _host.c_str();
    if (_port != 80 && _port != 443) {
        ss << ":" << _port;
    }
    ss << "\r\n";

    for (const auto& h : _headers) {
        ss << h.first.c_str() << ": " << h.second.c_str() << "\r\n";
    }

    if (body.length() > 0) {
        ss << "Content-Length: " << body.length() << "\r\n";
    }
    ss << "\r\n";

    if (body.length() > 0) {
        ss << body.c_str();
    }

    std::string requestStr = ss.str();
    if (_client->write((const uint8_t*)requestStr.data(), requestStr.size()) != requestStr.size()) {
        return HTTPC_ERROR_SEND_HEADER_FAILED;
    }

    return readResponse();
}

int HTTPClient::readResponse() {
    _responseBody = "";
    _contentLength = -1;

    // Read status line
    String statusLine = _client->readStringUntil('\n');
    statusLine.trim();
    if (statusLine.length() == 0) {
        return HTTPC_ERROR_CONNECTION_LOST;
    }

    int statusCode = -1;
    int firstSpace = statusLine.indexOf(' ');
    if (firstSpace != -1) {
        int secondSpace = statusLine.indexOf(' ', firstSpace + 1);
        if (secondSpace != -1) {
            statusCode = statusLine.substring(firstSpace + 1, secondSpace).toInt();
        } else {
            statusCode = statusLine.substring(firstSpace + 1).toInt();
        }
    }

    // Read headers
    while (true) {
        String line = _client->readStringUntil('\n');
        line.trim();
        if (line.length() == 0) {
            break; // Empty line signifies end of headers
        }

        int colon = line.indexOf(':');
        if (colon != -1) {
            String name = line.substring(0, colon);
            name.trim();
            String val = line.substring(colon + 1);
            val.trim();

            if (name.equalsIgnoreCase("Content-Length")) {
                _contentLength = val.toInt();
            }
        }
    }

    return statusCode;
}

String HTTPClient::getString() {
    if (_responseBody.length() > 0) {
        return _responseBody;
    }
    if (!_client) return "";

    if (_contentLength > 0) {
        std::vector<char> buf(_contentLength + 1, 0);
        size_t read_bytes = _client->readBytes((uint8_t*)buf.data(), _contentLength);
        _responseBody = String(std::string(buf.data(), read_bytes));
    } else if (_contentLength == 0) {
        _responseBody = "";
    } else {
        // Read until EOF
        while (_client->connected()) {
            char c = _client->read();
            if (c < 0) break;
            _responseBody += c;
        }
    }
    return _responseBody;
}

String HTTPClient::errorToString(int error) {
    switch (error) {
        case HTTPC_ERROR_CONNECTION_REFUSED: return "connection refused";
        case HTTPC_ERROR_SEND_HEADER_FAILED: return "send header failed";
        case HTTPC_ERROR_SEND_PAYLOAD_FAILED: return "send payload failed";
        case HTTPC_ERROR_NOT_CONNECTED: return "not connected";
        case HTTPC_ERROR_CONNECTION_LOST: return "connection lost";
        case HTTPC_ERROR_NO_STREAM: return "no stream";
        case HTTPC_ERROR_NO_HTTP_SERVER: return "no http server";
        default: return "unknown error";
    }
}
