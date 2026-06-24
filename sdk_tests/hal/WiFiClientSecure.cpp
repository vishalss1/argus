#include "WiFiClientSecure.h"
#include <openssl/err.h>
#include <openssl/x509v3.h>
#include <iostream>
#include <cstring>

WiFiClientSecure::WiFiClientSecure() 
    : WiFiClient(), _sslCtx(nullptr), _ssl(nullptr), _CA_cert(nullptr), _cert(nullptr), _private_key(nullptr) {
}

WiFiClientSecure::~WiFiClientSecure() {
    stop();
}

int WiFiClientSecure::connect(IPAddress ip, uint16_t port) {
    return connect(ip.toString().c_str(), port);
}

int WiFiClientSecure::connect(const char *host, uint16_t port) {
    return connect(host, port, host, _CA_cert, _cert, _private_key);
}

int WiFiClientSecure::connect(const char* host, uint16_t port, int32_t timeout) {
    _timeout = timeout;
    return connect(host, port);
}

int WiFiClientSecure::connect(IPAddress ip, uint16_t port, int32_t timeout) {
    _timeout = timeout;
    return connect(ip, port);
}

int WiFiClientSecure::connect(IPAddress ip, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key) {
    return connect(ip.toString().c_str(), port, verifyHost, ca, cert, key);
}

int WiFiClientSecure::connect(const char* host, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key) {
    return connectSSL(host, port, verifyHost, ca, cert, key);
}

int WiFiClientSecure::connectSSL(const char* host, uint16_t port, const char* verifyHost, const char* ca, const char* cert, const char* key) {
    stop();

    // 1. Establish TCP connection
    if (!WiFiClient::connect(host, port)) {
        return 0;
    }

    // 2. Initialize SSL Context
    const SSL_METHOD* method = TLS_client_method();
    _sslCtx = SSL_CTX_new(method);
    if (!_sslCtx) {
        WiFiClient::stop();
        return 0;
    }

    // Disable SSLv2/SSLv3/TLSv1/TLSv1.1 if needed, TLS_client_method defaults to modern TLS negotiations

    // 3. Load CA cert
    if (ca) {
        BIO* bio = BIO_new_mem_buf((void*)ca, -1);
        if (bio) {
            X509* x509 = PEM_read_bio_X509(bio, nullptr, nullptr, nullptr);
            if (x509) {
                X509_STORE* store = SSL_CTX_get_cert_store(_sslCtx);
                if (store) {
                    X509_STORE_add_cert(store, x509);
                }
                X509_free(x509);
            }
            BIO_free(bio);
        }
        SSL_CTX_set_verify(_sslCtx, SSL_VERIFY_PEER, nullptr);
    } else {
        SSL_CTX_set_verify(_sslCtx, SSL_VERIFY_NONE, nullptr);
    }

    // 4. Load client cert
    if (cert) {
        BIO* bio = BIO_new_mem_buf((void*)cert, -1);
        if (bio) {
            X509* x509 = PEM_read_bio_X509(bio, nullptr, nullptr, nullptr);
            if (x509) {
                SSL_CTX_use_certificate(_sslCtx, x509);
                X509_free(x509);
            }
            BIO_free(bio);
        }
    }

    // 5. Load client key
    if (key) {
        BIO* bio = BIO_new_mem_buf((void*)key, -1);
        if (bio) {
            EVP_PKEY* pkey = PEM_read_bio_PrivateKey(bio, nullptr, nullptr, nullptr);
            if (pkey) {
                SSL_CTX_use_PrivateKey(_sslCtx, pkey);
                EVP_PKEY_free(pkey);
            }
            BIO_free(bio);
        }
    }

    _ssl = SSL_new(_sslCtx);
    if (!_ssl) {
        stop();
        return 0;
    }

    SSL_set_fd(_ssl, _fd);

    // 6. Hostname verification
    if (verifyHost && std::strlen(verifyHost) > 0) {
        SSL_set_tlsext_host_name(_ssl, verifyHost);
        SSL_set1_host(_ssl, verifyHost);
    }

    // 7. Perform SSL Handshake
    if (SSL_connect(_ssl) <= 0) {
        stop();
        return 0;
    }

    return 1;
}

size_t WiFiClientSecure::write(const uint8_t *buf, size_t size) {
    if (!_ssl) return 0;
    size_t total = 0;
    while (total < size) {
        int written = SSL_write(_ssl, buf + total, (int)(size - total));
        if (written <= 0) break;
        total += (size_t)written;
    }
    return total;
}

int WiFiClientSecure::available() {
    if (!_ssl) return 0;
    int pending = SSL_pending(_ssl);
    if (pending > 0) return pending;
    return WiFiClient::available();
}

int WiFiClientSecure::read(uint8_t *buf, size_t size) {
    if (!_ssl) return -1;
    int read_bytes = SSL_read(_ssl, buf, size);
    if (read_bytes <= 0) {
        int err = SSL_get_error(_ssl, read_bytes);
        if (err == SSL_ERROR_WANT_READ || err == SSL_ERROR_WANT_WRITE) {
            return 0;
        }
        return -1;
    }
    return read_bytes;
}

int WiFiClientSecure::peek() {
    if (!_ssl) return -1;
    uint8_t c;
    int res = SSL_peek(_ssl, &c, 1);
    if (res > 0) return c;
    return -1;
}

void WiFiClientSecure::stop() {
    if (_ssl) {
        SSL_shutdown(_ssl);
        SSL_free(_ssl);
        _ssl = nullptr;
    }
    if (_sslCtx) {
        SSL_CTX_free(_sslCtx);
        _sslCtx = nullptr;
    }
    WiFiClient::stop();
}

uint8_t WiFiClientSecure::connected() {
    if (!_ssl) return 0;
    
    // Check if SSL is still connected.
    // Try to peek one byte.
    uint8_t c;
    int res = SSL_peek(_ssl, &c, 1);
    if (res <= 0) {
        int err = SSL_get_error(_ssl, res);
        if (err == SSL_ERROR_WANT_READ || err == SSL_ERROR_WANT_WRITE) {
            return 1;
        }
        stop();
        return 0;
    }
    return 1;
}

bool WiFiClientSecure::getFingerprintSHA256(uint8_t out[32]) {
    if (!_ssl) return false;
    X509* cert = SSL_get_peer_certificate(_ssl);
    if (!cert) return false;

    unsigned int len = 32;
    int res = X509_digest(cert, EVP_sha256(), out, &len);
    X509_free(cert);
    return res != 0;
}

void WiFiClientSecure::lastError(char* buf, size_t len) {
    unsigned long err = ERR_get_error();
    if (err) {
        ERR_error_string_n(err, buf, len);
    } else {
        std::strncpy(buf, "No error", len);
    }
}
