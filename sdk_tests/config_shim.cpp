#include <cstdlib>
#include <cstring>
#include <string>
#include <iostream>
#include <cstdint>

char ARGUS_FW_VERSION[256] = {0};
char ARGUS_DEVICE_ID[256] = {0};
char ARGUS_API_KEY[256] = {0};
char ARGUS_SERVER_HOST[256] = {0};
char ARGUS_MQTT_HOST[256] = {0};
uint16_t ARGUS_HTTP_PORT = 0;
uint16_t ARGUS_MQTT_PORT = 0;
char WIFI_SSID[256] = {0};
char WIFI_PASSWORD[256] = {0};
char ARGUS_OTA_KEY_ID[256] = {0};
char ARGUS_OTA_PUBLIC_KEY_B64[256] = {0};
char ARGUS_ROOT_CA[4096] = {0};
char ARGUS_DEVICE_CERT[4096] = {0};
char ARGUS_DEVICE_PRIVATE_KEY[4096] = {0};

void initEnvVars() {
    auto loadEnv = [](const char* name, char* target, size_t maxLen) {
        const char* val = std::getenv(name);
        if (val) {
            std::string s(val);
            size_t pos = 0;
            while ((pos = s.find("\\n", pos)) != std::string::npos) {
                s.replace(pos, 2, "\n");
                pos += 1;
            }
            std::strncpy(target, s.c_str(), maxLen - 1);
            target[maxLen - 1] = '\0';
        }
    };

    loadEnv("ARGUS_FW_VERSION", ARGUS_FW_VERSION, sizeof(ARGUS_FW_VERSION));
    loadEnv("ARGUS_DEVICE_ID", ARGUS_DEVICE_ID, sizeof(ARGUS_DEVICE_ID));
    loadEnv("ARGUS_API_KEY", ARGUS_API_KEY, sizeof(ARGUS_API_KEY));
    loadEnv("ARGUS_SERVER_HOST", ARGUS_SERVER_HOST, sizeof(ARGUS_SERVER_HOST));
    loadEnv("ARGUS_MQTT_HOST", ARGUS_MQTT_HOST, sizeof(ARGUS_MQTT_HOST));
    loadEnv("WIFI_SSID", WIFI_SSID, sizeof(WIFI_SSID));
    loadEnv("WIFI_PASSWORD", WIFI_PASSWORD, sizeof(WIFI_PASSWORD));
    loadEnv("ARGUS_OTA_KEY_ID", ARGUS_OTA_KEY_ID, sizeof(ARGUS_OTA_KEY_ID));
    loadEnv("ARGUS_OTA_PUBLIC_KEY_B64", ARGUS_OTA_PUBLIC_KEY_B64, sizeof(ARGUS_OTA_PUBLIC_KEY_B64));
    loadEnv("ARGUS_ROOT_CA_PEM", ARGUS_ROOT_CA, sizeof(ARGUS_ROOT_CA));
    loadEnv("ARGUS_DEVICE_CERT_PEM", ARGUS_DEVICE_CERT, sizeof(ARGUS_DEVICE_CERT));
    loadEnv("ARGUS_DEVICE_PRIVATE_KEY_PEM", ARGUS_DEVICE_PRIVATE_KEY, sizeof(ARGUS_DEVICE_PRIVATE_KEY));

    const char* httpPortVal = std::getenv("ARGUS_HTTP_PORT");
    if (httpPortVal) {
        ARGUS_HTTP_PORT = static_cast<uint16_t>(std::stoi(httpPortVal));
    }
    const char* mqttPortVal = std::getenv("ARGUS_MQTT_PORT");
    if (mqttPortVal) {
        ARGUS_MQTT_PORT = static_cast<uint16_t>(std::stoi(mqttPortVal));
    }
}

#ifndef _WIN32
__attribute__((constructor)) static void constructor_init() {
    initEnvVars();
}
#endif
