#pragma once

// ---------------------------------------------------------------------------
// argus_nvs.h — NVS provisioning helpers
//
// These two functions are implemented in argus_nvs.cpp (on-device build) or
// stubbed out by the native host build's config_shim.cpp.
// ---------------------------------------------------------------------------

/// Load all device-identity values from the "argus_cfg" NVS namespace into
/// the extern buffers declared in argus_config.h.
/// Returns true on success, false if any required key (device_id, api_key,
/// root_ca) is absent.  Must be called before any other SDK function.
bool argusNVSLoad();

/// Return true if the "argus_cfg" NVS namespace contains a non-empty
/// "device_id" key.  Lightweight probe — does not populate any buffers.
bool argusIsProvisioned();
