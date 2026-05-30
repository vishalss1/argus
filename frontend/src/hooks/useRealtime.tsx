import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { withEffectiveDeviceStatus } from "../lib/format";
import { websocketURL } from "../services/http";
import type { Deployment, Device, Telemetry } from "../types/api";
import { queryKeys } from "./useArgusData";

type RealtimeStatus = "connecting" | "connected" | "disconnected";

type OTARealtimeType = "ota_created" | "ota_progress" | "ota_status_changed" | "ota_completed" | "ota_failed";

interface RealtimeMessage {
  type: "device_update" | "telemetry" | "device_presence" | "command_update" | OTARealtimeType;
  payload: unknown;
  deviceId?: string;
  status?: string;
  timestamp?: string;
}

interface RealtimeContextValue {
  status: RealtimeStatus;
  telemetryByDevice: Record<string, Telemetry[]>;
}

const RealtimeContext = createContext<RealtimeContextValue>({
  status: "connecting",
  telemetryByDevice: {}
});

export function RealtimeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<RealtimeStatus>("connecting");
  const [telemetryByDevice, setTelemetryByDevice] = useState<Record<string, Telemetry[]>>({});
  const retryRef = useRef(0);

  useEffect(() => {
    let closed = false;
    let socket: WebSocket | null = null;
    let initialTimer: number | undefined;
    let reconnectTimer: number | undefined;

    function scheduleReconnect() {
      if (closed) return;
      setStatus("disconnected");
      const delay = Math.min(10_000, 500 * 2 ** retryRef.current);
      retryRef.current += 1;
      reconnectTimer = window.setTimeout(connect, delay);
    }

    function connect() {
      if (closed) return;
      const url = websocketURL();
      setStatus("connecting");
      socket = new WebSocket(url);

      socket.onopen = () => {
        retryRef.current = 0;
        setStatus("connected");
      };

      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as RealtimeMessage;
          
          if (message.type === "device_update") {
            const device = withEffectiveDeviceStatus(message.payload as Device);
            queryClient.setQueryData<Device[]>(queryKeys.devices, (current = []) => {
              const exists = current.some((item) => item.id === device.id);
              if (!exists) return [device, ...current];
              return current.map((item) => (item.id === device.id ? device : item));
            });
          }
          
          if (message.type === "device_presence" && message.deviceId && message.status) {
            queryClient.setQueryData<Device[]>(queryKeys.devices, (current = []) =>
              current.map((device) =>
                device.id === message.deviceId
                  ? {
                      ...device,
                      status: message.status!,
                      last_seen: message.timestamp ?? device.last_seen,
                      updated_at: message.timestamp ?? device.updated_at
                    }
                  : device
              )
            );
          }
          
          if (message.type === "telemetry") {
            const telemetry = message.payload as Telemetry;
            setTelemetryByDevice((current) => {
              const prev = current[telemetry.device_id] || [];
              return {
                ...current,
                [telemetry.device_id]: [telemetry, ...prev].slice(0, 50)
              };
            });
          }

          if (message.type === "command_update") {
            queryClient.invalidateQueries({ queryKey: ["commands"] });
          }

          if (["ota_created", "ota_progress", "ota_status_changed", "ota_completed", "ota_failed"].includes(message.type)) {
            const deployment = message.payload as Deployment;
            queryClient.setQueryData<Deployment[]>(queryKeys.allDeployments, (current = []) => {
              const exists = current.some((item) => item.id === deployment.id);
              if (!exists) return [deployment, ...current];
              return current.map((item) => (item.id === deployment.id ? deployment : item));
            });
            queryClient.setQueryData<Deployment[]>(queryKeys.deployments(deployment.device_id), (current = []) => {
              const exists = current.some((item) => item.id === deployment.id);
              if (!exists) return [deployment, ...current];
              return current.map((item) => (item.id === deployment.id ? deployment : item));
            });
            queryClient.invalidateQueries({ queryKey: queryKeys.otaStats });
            queryClient.invalidateQueries({ queryKey: ["deployment-events", deployment.id] });
          }
        } catch {
          // Ignore malformed frames
        }
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onclose = () => {
        scheduleReconnect();
      };
    }

    initialTimer = window.setTimeout(connect, 0);

    return () => {
      closed = true;
      if (initialTimer !== undefined) window.clearTimeout(initialTimer);
      if (reconnectTimer !== undefined) window.clearTimeout(reconnectTimer);
      if (socket?.readyState === WebSocket.OPEN) {
        socket.close();
      }
    };
  }, [queryClient]);

  return (
    <RealtimeContext.Provider value={{ status, telemetryByDevice }}>
      {children}
    </RealtimeContext.Provider>
  );
}

export function useRealtime() {
  return useContext(RealtimeContext);
}
