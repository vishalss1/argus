#ifndef HTTP_CLIENT_H
#define HTTP_CLIENT_H

#include "WiFiClientSecure.h"
#include <vector>
#include <utility>

class HTTPClient {
protected:
    WiFiClient* _client;
    String _scheme;
    String _host;
    uint16_t _port;
    String _path;
    std::vector<std::pair<String, String>> _headers;
    int _contentLength;
    String _responseBody;
    int32_t _timeout;

    void parseUrl(const String& url);
    int readResponse();
public:
    HTTPClient();
    virtual ~HTTPClient();

    bool begin(WiFiClient& client, String url);
    bool begin(WiFiClientSecure& client, String url);

    void addHeader(const String& name, const String& value);
    void setTimeout(uint16_t timeout);
    void setReuse(bool reuse) {}
    void setFollowRedirects(int policy) {}
    void setRedirectLimit(int limit) {}
    bool connected() { return _client != nullptr && _client->connected() != 0; }

    int GET();
    int sendRequest(const char* method, const String& body = "");

    int getSize() { return _contentLength; }
    String getString();
    WiFiClient* getStreamPtr() { return _client; }
    void end() { _headers.clear(); }

    static String errorToString(int error);
};

// Constants
#define HTTP_CODE_OK 200
#define HTTP_CODE_NO_CONTENT 204
#define HTTP_CODE_NOT_FOUND 404
#define HTTP_CODE_FORBIDDEN 403

#define HTTPC_ERROR_CONNECTION_REFUSED -1
#define HTTPC_ERROR_SEND_HEADER_FAILED -2
#define HTTPC_ERROR_SEND_PAYLOAD_FAILED -3
#define HTTPC_ERROR_NOT_CONNECTED -4
#define HTTPC_ERROR_CONNECTION_LOST -5
#define HTTPC_ERROR_NO_STREAM -6
#define HTTPC_ERROR_NO_HTTP_SERVER -7
#define HTTPC_ERROR_TOO_LESS_RAM -8
#define HTTPC_ERROR_ENCODING -9
#define HTTPC_ERROR_STREAM_WRITE -10
#define HTTPC_ERROR_READ_TIMEOUT -11
#define HTTPC_STRICT_FOLLOW_REDIRECTS 2

#endif // HTTP_CLIENT_H
