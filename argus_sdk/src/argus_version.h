#pragma once

// ARGUS_FW_VERSION is declared in argus_config.h and defined in the generated template/sketch.

namespace argus_sdk
{
    constexpr char SDK_VERSION[] = "1.0.0";
    constexpr uint32_t SDK_VERSION_MAJOR = 1;
    constexpr uint32_t SDK_VERSION_MINOR = 0;
    constexpr uint32_t SDK_VERSION_PATCH = 0;
}

inline const char* getArgusSdkVersion() {
  return argus_sdk::SDK_VERSION;
}

inline uint32_t getArgusSdkVersionMajor() {
  return argus_sdk::SDK_VERSION_MAJOR;
}

inline uint32_t getArgusSdkVersionMinor() {
  return argus_sdk::SDK_VERSION_MINOR;
}

inline uint32_t getArgusSdkVersionPatch() {
  return argus_sdk::SDK_VERSION_PATCH;
}
