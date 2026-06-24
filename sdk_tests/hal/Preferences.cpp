#include "Preferences.h"

std::map<std::string, std::map<std::string, std::string>> Preferences::_prefsMap;

bool Preferences::begin(const char* name, bool readOnly) {
    _ns = name;
    _readOnly = readOnly;
    return true;
}

size_t Preferences::putString(const char* key, const String value) {
    if (_readOnly) return 0;
    _prefsMap[_ns][key] = value.c_str();
    return value.length();
}

String Preferences::getString(const char* key, const String defaultValue) {
    auto nsIt = _prefsMap.find(_ns);
    if (nsIt != _prefsMap.end()) {
        auto keyIt = nsIt->second.find(key);
        if (keyIt != nsIt->second.end()) {
            return String(keyIt->second.c_str());
        }
    }
    return defaultValue;
}

size_t Preferences::putBool(const char* key, bool value) {
    return putString(key, value ? "1" : "0");
}

bool Preferences::getBool(const char* key, bool defaultValue) {
    String val = getString(key, defaultValue ? "1" : "0");
    return val == "1";
}

bool Preferences::remove(const char* key) {
    if (_readOnly) return false;
    auto nsIt = _prefsMap.find(_ns);
    if (nsIt != _prefsMap.end()) {
        return nsIt->second.erase(key) > 0;
    }
    return false;
}
