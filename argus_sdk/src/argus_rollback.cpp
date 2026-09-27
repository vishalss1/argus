#include "argus_rollback.h"
#include "argus_mqtt.h"
#include "argus_ota.h"
#include "argus_state_machine.h"
#include <WiFi.h>
#include <esp_ota_ops.h>

namespace argus_sdk {

extern bool timeSynced;

#ifdef CONFIG_BOOTLOADER_APP_ROLLBACK_ENABLE
// Grace window for the PENDING_VERIFY health check: a transient broker outage
// or a slow NTP sync must not invalidate an otherwise healthy image.
constexpr int ROLLBACK_VERIFY_ATTEMPTS = 3;
constexpr unsigned long ROLLBACK_VERIFY_RETRY_MS = 5000UL;
#endif

void handleRollbackVerification() {
#ifdef CONFIG_BOOTLOADER_APP_ROLLBACK_ENABLE
  const esp_partition_t* running = esp_ota_get_running_partition();
  esp_ota_img_states_t state;

  if (running == nullptr) {
    Serial.println("[ROLLBACK] Cannot inspect running partition");
    return;
  }

  esp_err_t err = esp_ota_get_state_partition(running, &state);
  if (err != ESP_OK) {
    Serial.printf("[ROLLBACK] State query failed: %d\n", err);
    return;
  }

  if (state != ESP_OTA_IMG_PENDING_VERIFY) {
    Serial.printf("[ROLLBACK] Running image state=%d; no validation needed\n", state);
    return;
  }

  Serial.println("[ROLLBACK] Running image pending verification");

  // Grace window: a single failed probe at first boot can be a transient
  // broker outage or a slow NTP sync, neither of which means the image is
  // broken.  Re-check before giving up on a healthy firmware.
  bool basicHealthOk = false;
  for (int attempt = 1; attempt <= ROLLBACK_VERIFY_ATTEMPTS; attempt++) {
    if (WiFi.status() == WL_CONNECTED && mqtt.connected() && timeSynced) {
      basicHealthOk = true;
      break;
    }
    if (attempt < ROLLBACK_VERIFY_ATTEMPTS) {
      Serial.printf("[ROLLBACK] Health check %d/%d failed; retrying in %d ms\n",
                    attempt, ROLLBACK_VERIFY_ATTEMPTS, ROLLBACK_VERIFY_RETRY_MS);
      // Pump the MQTT client so a handshake already in flight can complete
      // before the next probe instead of stalling the whole grace window.
      for (unsigned long waited = 0; waited < ROLLBACK_VERIFY_RETRY_MS; waited += 100) {
        mqtt.loop();
        delay(100);
      }
    }
  }

  if (basicHealthOk) {
    esp_err_t markErr = esp_ota_mark_app_valid_cancel_rollback();
    Serial.printf("[ROLLBACK] Mark app valid result=%d\n", markErr);
    return;
  }

  Serial.println("[ROLLBACK] Health checks failed. Rolling back...");
  PendingOTAResult pending = loadPendingOTAACK();
  if (pending.deploymentId.length() > 0) {
    publishOTANack(pending.deploymentId, "Boot verification failed; rollback requested");
  }
  // ponytail: clear pending ACK on rollback so old firmware does not publish fake success
  clearPendingOTAACK();
  Serial.println("[ROLLBACK] Boot health failed; marking app invalid and rebooting for rollback");
  esp_ota_mark_app_invalid_rollback_and_reboot();
#else
  Serial.println("[ROLLBACK] ESP-IDF rollback support is not enabled in this build");
#endif
}

}
