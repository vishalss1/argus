#include "argus_rollback.h"
#include "argus_mqtt.h"
#include "argus_ota.h"
#include "argus_state_machine.h"
#include <WiFi.h>
#include <esp_ota_ops.h>

namespace argus_sdk {

extern bool timeSynced;

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
  bool basicHealthOk = WiFi.status() == WL_CONNECTED && mqtt.connected() && timeSynced;
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
  Serial.println("[ROLLBACK] Boot health failed; marking app invalid and rebooting for rollback");
  esp_ota_mark_app_invalid_rollback_and_reboot();
#else
  Serial.println("[ROLLBACK] ESP-IDF rollback support is not enabled in this build");
#endif
}

}
