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

int main() {
    std::cout << "[TEST] Initializing env vars..." << std::endl;
    initEnvVars();

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
        {"[MQTT] Connection failed", "[MQTT] Connection failed", false}
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
