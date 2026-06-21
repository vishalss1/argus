#pragma once

namespace argus_sdk {

enum ConnectivityState {
    ARGUS_WIFI_DOWN,
    ARGUS_WIFI_CONNECTED,
    ARGUS_MQTT_CONNECTED
};

extern ConnectivityState connectivityState;

const char* connectivityStateName(ConnectivityState state);
ConnectivityState computeConnectivityState();
void updateConnectivityState(const char* reason);

}
