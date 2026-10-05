#include <iostream>
#include <sstream>
#include <string>
#include <vector>
#include <iomanip>
#include <argus.h>

extern void initEnvVars();

struct AssertionCheck {
    std::string name;
    std::string pattern;
    bool expected_present; // true for positive, false for negative
};

int countOccurrences(const std::string& text, const std::string& pattern) {
    if (pattern.empty()) return 0;
    int count = 0;
    std::size_t pos = 0;
    while ((pos = text.find(pattern, pos)) != std::string::npos) {
        count++;
        pos += pattern.length();
    }
    return count;
}

// Exercises the OTA anti-downgrade decision against the compiled-in firmware
// version. The device's reported version comes from ARGUS_FIRMWARE_VERSION, not
// from NVS, so these checks prove the running binary — not stale provisioning
// metadata — decides whether a manifest may be flashed.
bool runVersionChecks() {
    bool ok = true;

    Serial.printf("[TEST] Compiled-in firmware version: %s\n", ARGUS_FIRMWARE_VERSION);

    argus_sdk::Version current = argus_sdk::parseVersion(ARGUS_FW_VERSION);
    if (!current.valid) {
        Serial.println("[TESTFAIL] ARGUS_FW_VERSION is not valid semver");
        ok = false;
    }

    // The running version and the compile-time constant must be the same value.
    if (std::string(ARGUS_FW_VERSION) != std::string(ARGUS_FIRMWARE_VERSION)) {
        Serial.println("[TESTFAIL] ARGUS_FW_VERSION does not match ARGUS_FIRMWARE_VERSION");
        ok = false;
    }

    // parseVersion must accept exactly the grammar the backend enforces.
    const char* valid[] = {"1.2.0", "v1.2.0", "V1.2.0", "0.0.0", "10.20.30"};
    for (const char* v : valid) {
        if (!argus_sdk::parseVersion(String(v)).valid) {
            Serial.printf("[TESTFAIL] parseVersion rejected valid version %s\n", v);
            ok = false;
        }
    }
    const char* invalid[] = {"", "v", "1.2", "1.2.3.4", ".2.0", "1..0", "1.2.", "x.2.0", "1.2.x", "1.2.0-rc1", "1.2.0+b1"};
    for (const char* v : invalid) {
        if (argus_sdk::parseVersion(String(v)).valid) {
            Serial.printf("[TESTFAIL] parseVersion accepted invalid version %s\n", v);
            ok = false;
        }
    }

    // Ordering used by the downgrade check.
    if (argus_sdk::compareVersions(argus_sdk::parseVersion("1.2.0"), argus_sdk::parseVersion("1.2.1")) >= 0) {
        Serial.println("[TESTFAIL] compareVersions ordering is wrong");
        ok = false;
    }
    if (argus_sdk::compareVersions(argus_sdk::parseVersion("1.2.0"), argus_sdk::parseVersion("1.2.0")) != 0) {
        Serial.println("[TESTFAIL] equal versions must compare equal");
        ok = false;
    }

    auto manifestFor = [](const char* version, bool allowDowngrade) {
        argus_sdk::OTAManifest m;
        m.version = String(version);
        m.deploymentId = String("test-deployment");
        m.allowDowngrade = allowDowngrade;
        return m;
    };

    // An upgrade is allowed; the check compares against the compiled-in version.
    std::string bumped = std::to_string(current.major) + "." +
                         std::to_string(current.minor) + "." +
                         std::to_string(current.patch + 1);
    if (!argus_sdk::versionAllowed(manifestFor(bumped.c_str(), false))) {
        Serial.printf("[TESTFAIL] upgrade to %s should be allowed\n", bumped.c_str());
        ok = false;
    }

    // The current version is a no-op rather than a flash.
    std::string same = std::to_string(current.major) + "." +
                       std::to_string(current.minor) + "." +
                       std::to_string(current.patch);
    if (argus_sdk::versionAllowed(manifestFor(same.c_str(), false))) {
        Serial.printf("[TESTFAIL] re-flashing the running version %s should be skipped\n", same.c_str());
        ok = false;
    }

    // A downgrade is rejected unless explicitly permitted.
    std::string older = std::to_string(current.major) + "." +
                        std::to_string(current.minor) + "." +
                        std::to_string(current.patch > 0 ? current.patch - 1 : 0);
    if (current.patch > 0) {
        if (argus_sdk::versionAllowed(manifestFor(older.c_str(), false))) {
            Serial.printf("[TESTFAIL] downgrade to %s should be rejected\n", older.c_str());
            ok = false;
        }
        if (!argus_sdk::versionAllowed(manifestFor(older.c_str(), true))) {
            Serial.printf("[TESTFAIL] downgrade to %s should be allowed when permitted\n", older.c_str());
            ok = false;
        }
    }

    // Unparseable manifest versions fail closed.
    if (argus_sdk::versionAllowed(manifestFor("not-a-version", false))) {
        Serial.println("[TESTFAIL] a malformed manifest version must be rejected");
        ok = false;
    }

    Serial.println(ok ? "[TEST] Version checks passed" : "[TEST] Version checks FAILED");
    return ok;
}

int main() {
    std::cout << "[TEST] Initializing env vars..." << std::endl;
    initEnvVars();

    bool versionsOk = runVersionChecks();
    if (!versionsOk) {
        std::cerr << "\n[TEST] SDK integration test FAILED in version checks!" << std::endl;
        return 1;
    }

    std::cout << "[TEST] Starting SDK integration test harness..." << std::endl;
    argusBegin();

    std::cout << "[TEST] Running event loop for 120 seconds..." << std::endl;
    unsigned long start = millis();
    while (millis() - start < 120000) {
        argusLoop();
        delay(10);
    }

    std::cout << "[TEST] Event loop completed. Analyzing results..." << std::endl;
    std::string capture = g_serialCapture.str();
    
    std::cout << "\n================= CAPTURED SERIAL OUTPUT =================\n";
    std::cout << capture;
    std::cout << "==========================================================\n\n";

    // Verified assertion strings against the actual SDK source code
    std::vector<AssertionCheck> assertions = {
        // Positive assertions (must appear >= 1 time)
        {"bootOk", "[BOOT] ARGUS ESP32 firmware starting", true},
        {"mqttOk", "[MQTT] Connected to broker", true},
        {"shadowOk", "[SHADOW] Reported state update HTTP 200", true},
        {"heartbeatOk", "[HEARTBEAT] HTTP 200", true},
        {"otaPollOk", "[OTA] No pending deployment available", true},
        {"telemetryOk", "[TELEMETRY] Publish ok", true},

        // Negative assertions (must NOT appear, i.e., count == 0)
        {"certificate pin mismatch", "certificate pin mismatch", false},
        {"HTTPS rejected", "HTTPS rejected", false},
        {"Ed25519 public key is missing", "Ed25519 public key is missing", false},
        {"[AUTH] Device API key not set", "[AUTH] Device API key not set", false},
        {"[MQTT] Connection failed", "[MQTT] Connection failed", false},
        {"[BOOT] NVS not provisioned", "[BOOT] NVS not provisioned", false}
    };

    std::cout << "[TEST] Validation Results Scorecard:" << std::endl;
    std::cout << std::left 
              << std::setw(32) << "Assertion Name" 
              << std::setw(20) << "Expected" 
              << std::setw(12) << "Result" 
              << std::setw(8) << "Count" << std::endl;
    std::cout << "------------------------------------------------------------------------" << std::endl;

    bool allPassed = true;
    for (const auto& a : assertions) {
        int count = countOccurrences(capture, a.pattern);
        bool passed = a.expected_present ? (count >= 1) : (count == 0);
        if (!passed) {
            allPassed = false;
        }

        std::string expectedStr = a.expected_present ? "Present (>= 1)" : "Absent (== 0)";
        std::string resultStr = passed ? "PASSED" : "FAILED";

        std::cout << std::left 
                  << std::setw(32) << a.name 
                  << std::setw(20) << expectedStr 
                  << std::setw(12) << resultStr 
                  << std::setw(8) << count << std::endl;
    }
    std::cout << "------------------------------------------------------------------------" << std::endl;

    if (allPassed) {
        std::cout << "\n[TEST] ALL INTEGRATION TESTS PASSED!" << std::endl;
        return 0;
    } else {
        std::cerr << "\n[TEST] INTEGRATION TESTS FAILED!" << std::endl;
        return 1;
    }
}
