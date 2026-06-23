#include <iostream>
#include <sstream>
#include <string>
#include <argus.h>

extern void initEnvVars();

int main() {
    std::cout << "[TEST] Initializing env vars..." << std::endl;
    initEnvVars();

    std::cout << "[TEST] Starting SDK integration test harness..." << std::endl;
    argusBegin();

    std::cout << "[TEST] Running event loop for 15 seconds..." << std::endl;
    unsigned long start = millis();
    while (millis() - start < 15000) {
        argusLoop();
        delay(10);
    }

    std::cout << "[TEST] Event loop completed. Analyzing results..." << std::endl;
    std::string capture = g_serialCapture.str();
    
    std::cout << "\n================= CAPTURED SERIAL OUTPUT =================\n";
    std::cout << capture;
    std::cout << "==========================================================\n\n";

    // Validation checks
    bool bootOk = capture.find("[BOOT] ARGUS ESP32 firmware starting") != std::string::npos;
    bool mqttOk = capture.find("[MQTT] Connected to broker") != std::string::npos;
    
    // HTTP status check - we expect GET/POST status codes to be 200 or 204 or other positive status codes.
    // Let's look for "completed with status 20" (e.g. 200, 204)
    bool httpGetOk = capture.find("completed with status 20") != std::string::npos;

    std::cout << "[TEST] Validation Results:" << std::endl;
    std::cout << "  - Boot sequence: " << (bootOk ? "PASSED" : "FAILED") << std::endl;
    std::cout << "  - MQTT broker connection: " << (mqttOk ? "PASSED" : "FAILED") << std::endl;
    std::cout << "  - HTTP API calls: " << (httpGetOk ? "PASSED" : "FAILED") << std::endl;

    if (bootOk && mqttOk && httpGetOk) {
        std::cout << "\n[TEST] ALL INTEGRATION TESTS PASSED!" << std::endl;
        return 0;
    } else {
        std::cerr << "\n[TEST] INTEGRATION TESTS FAILED!" << std::endl;
        return 1;
    }
}
