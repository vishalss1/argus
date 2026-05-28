import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { withEffectiveDeviceStatus } from "../lib/format";
import { websocketURL } from "../services/http";
import type { Device, Telemetry } from "../types/api";
import { queryKeys } from "./useArgusData";

type RealtimeStatus = "connecting" | "connected" | "disconnected";

interface RealtimeMessage {
  type: "device_update" | "telemetry" | "device_presence";
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
      console.log(`[WS DEBUG] reconnecting in ${delay}ms...`);
      reconnectTimer = window.setTimeout(connect, delay);
    }

    function connect() {
      if (closed) return;
      const url = websocketURL();
      console.log("[WS DEBUG] connecting to:", url);
      setStatus("connecting");
      socket = new WebSocket(url);

      socket.onopen = () => {
        console.log("[WS DEBUG] connection established");
        retryRef.current = 0;
        setStatus("connected");
      };

      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data) as RealtimeMessage;
          console.log("[WS DEBUG] received message type:", message.type);
          
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
            console.log("[WS DEBUG] telemetry update for device:", telemetry.device_id);
            setTelemetryByDevice((current) => {
              const prev = current[telemetry.device_id] || [];
              return {
                ...current,
                [telemetry.device_id]: [telemetry, ...prev].slice(0, 50)
              };
            });
          }
        } catch (err) {
          console.error("[WS DEBUG] parse error:", err);
        }
      };

      socket.onerror = (err) => {
        console.error("[WS DEBUG] socket error:", err);
        socket?.close();
      };

      socket.onclose = (event) => {
        console.warn("[WS DEBUG] socket closed:", event.code, event.reason);
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
