#ifndef PREFERENCES_H
#define PREFERENCES_H

#include "Arduino.h"
#include <map>

class Preferences {
protected:
    std::string _ns;
    bool _readOnly;
    static std::map<std::string, std::map<std::string, std::string>> _prefsMap;
public:
    Preferences() : _readOnly(false) {}
    virtual ~Preferences() {}

    bool begin(const char* name, bool readOnly = false);
    void end() {}

    size_t putString(const char* key, const String value);
    String getString(const char* key, const String defaultValue = String(""));

    size_t putBool(const char* key, bool value);
    bool getBool(const char* key, bool defaultValue = false);

    bool remove(const char* key);
};

#endif // PREFERENCES_H
