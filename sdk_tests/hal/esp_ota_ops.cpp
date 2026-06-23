#include "esp_ota_ops.h"
#include <cstdlib>

static const esp_partition_t running_partition = {
    "app0",
    ESP_PARTITION_SUBTYPE_APP_OTA_0,
    0x10000,
    0x1E0000
};

static const esp_partition_t next_partition = {
    "app1",
    ESP_PARTITION_SUBTYPE_APP_OTA_1,
    0x200000,
    0x1E0000
};

const esp_partition_t* esp_ota_get_running_partition(void) {
    return &running_partition;
}

const esp_partition_t* esp_ota_get_next_update_partition(const esp_partition_t* start_from) {
    return &next_partition;
}

esp_err_t esp_ota_get_state_partition(const esp_partition_t* partition, esp_ota_img_states_t* ota_state) {
    if (ota_state) {
        *ota_state = ESP_OTA_IMG_VALID;
    }
    return ESP_OK;
}

esp_err_t esp_ota_mark_app_valid_cancel_rollback(void) {
    return ESP_OK;
}

esp_err_t esp_ota_mark_app_invalid_rollback_and_reboot(void) {
    std::exit(0);
}
