cmake_minimum_required(VERSION 3.15)
project(argus_sdk_test CXX C)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

# Find OpenSSL
find_package(OpenSSL REQUIRED)

# Find libsodium
find_library(SODIUM_LIBRARY NAMES sodium)
if(NOT SODIUM_LIBRARY)
    message(FATAL_ERROR "libsodium not found")
endif()

# Find mbedtls
find_library(MBEDCRYPTO_LIBRARY NAMES mbedcrypto)
find_library(MBEDTLS_LIBRARY NAMES mbedtls)
find_library(MBEDX509_LIBRARY NAMES mbedx509)
if(NOT MBEDCRYPTO_LIBRARY OR NOT MBEDTLS_LIBRARY OR NOT MBEDX509_LIBRARY)
    message(FATAL_ERROR "mbedtls libraries not found")
endif()

# Threads
find_package(Threads REQUIRED)

# Fetch dependencies dynamically
include(FetchContent)

FetchContent_Declare(
    ArduinoJson
    GIT_REPOSITORY https://github.com/bblanchon/ArduinoJson.git
    GIT_TAG v6.21.6
)

FetchContent_Declare(
    PubSubClient
    GIT_REPOSITORY https://github.com/knolleary/pubsubclient.git
    GIT_TAG v2.8
)

FetchContent_MakeAvailable(ArduinoJson PubSubClient)

# Include directories
include_directories(
    ${CMAKE_CURRENT_SOURCE_DIR}/hal
    ${pubsubclient_SOURCE_DIR}/src
    ${arduinojson_SOURCE_DIR}/src
    ${CMAKE_CURRENT_SOURCE_DIR}/../argus_sdk/src
)

# Source files
set(SOURCES
    # HAL Stubs
    hal/Arduino.cpp
    hal/WiFi.cpp
    hal/WiFiClient.cpp
    hal/WiFiClientSecure.cpp
    hal/HTTPClient.cpp
    hal/Preferences.cpp
    hal/Update.cpp
    hal/esp_ota_ops.cpp
    
    # PubSubClient
    ${pubsubclient_SOURCE_DIR}/src/PubSubClient.cpp
    
    # Shim & Runner
    config_shim.cpp
    main.cpp

    # SDK Source
    ../argus_sdk/src/argus.cpp
    ../argus_sdk/src/argus_diag.cpp
    ../argus_sdk/src/argus_http.cpp
    ../argus_sdk/src/argus_mqtt.cpp
    ../argus_sdk/src/argus_network.cpp
    ../argus_sdk/src/argus_ota.cpp
    ../argus_sdk/src/argus_rollback.cpp
    ../argus_sdk/src/argus_security.cpp
    ../argus_sdk/src/argus_state_machine.cpp
    ../argus_sdk/src/argus_time.cpp
    ../argus_sdk/src/argus_version.cpp
)

add_executable(argus_sdk_test ${SOURCES})

target_compile_definitions(argus_sdk_test PRIVATE ARGUS_NATIVE_BUILD)

# Target link libraries
target_link_libraries(argus_sdk_test
    PRIVATE
    OpenSSL::SSL
    OpenSSL::Crypto
    ${SODIUM_LIBRARY}
    ${MBEDTLS_LIBRARY}
    ${MBEDCRYPTO_LIBRARY}
    ${MBEDX509_LIBRARY}
    Threads::Threads
)

# Compile-only coverage for the relaxed-signature build. With
# ARGUS_REQUIRE_FIRMWARE_SIGNATURES=false the signature checks in
# verifyFirmwareAuthenticity() and parseOTAManifest() take their non-required
# branches, which is otherwise never compiled. Not built by default: it is a
# syntax/compile guard, not a runnable test target.
option(ARGUS_SDK_RELAXED_SIGNATURE_BUILD "Also build the relaxed-signature compile check" OFF)
if(ARGUS_SDK_RELAXED_SIGNATURE_BUILD)
    add_library(argus_sdk_nosig_compile OBJECT ${SOURCES})
    target_compile_definitions(argus_sdk_nosig_compile
        PRIVATE
        ARGUS_NATIVE_BUILD
        ARGUS_REQUIRE_FIRMWARE_SIGNATURES=false
    )
    target_include_directories(argus_sdk_nosig_compile PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/hal
        ${pubsubclient_SOURCE_DIR}/src
        ${arduinojson_SOURCE_DIR}/src
        ${CMAKE_CURRENT_SOURCE_DIR}/../argus_sdk/src
    )
endif()

if(WIN32)
    target_link_libraries(argus_sdk_test PRIVATE ws2_32)
endif()
