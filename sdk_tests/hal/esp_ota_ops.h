#ifndef ESP_OTA_OPS_H
#define ESP_OTA_OPS_H

#include "esp_partition.h"

#ifdef __cplusplus
extern "C" {
#endif

const esp_partition_t* esp_ota_get_running_partition(void);
const esp_partition_t* esp_ota_get_next_update_partition(const esp_partition_t* start_from);
esp_err_t esp_ota_get_state_partition(const esp_partition_t* partition, esp_ota_img_states_t* ota_state);
esp_err_t esp_ota_mark_app_valid_cancel_rollback(void);
esp_err_t esp_ota_mark_app_invalid_rollback_and_reboot(void);

#ifdef __cplusplus
}
#endif

#endif // ESP_OTA_OPS_H
