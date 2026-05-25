import { createContext, type ReactNode, useContext, useEffect, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { withEffectiveDeviceStatus } from "../lib/format";
import { websocketURL } from "../services/http";
import type { Device, Telemetry } from "../types/api";
import { queryKeys } from "./useArgusData";

type RealtimeStatus = "connecting" | "connected" | "disconnected";

interface RealtimeMessage {
  type: "device_update" | "telemetry";
  payload: unknown;
}

interface RealtimeContextValue {
  status: RealtimeStatus;
  telemetryByDevice: Record<string, Telemetry>;
}

const RealtimeContext = createContext<RealtimeContextValue>({
  status: "connecting",
  telemetryByDevice: {}
});

export function RealtimeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<RealtimeStatus>("connecting");
  const [telemetryByDevice, setTelemetryByDevice] = useState<Record<string, Telemetry>>({});
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
      setStatus("connecting");
      socket = new WebSocket(websocketURL());

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
          if (message.type === "telemetry") {
            const telemetry = message.payload as Telemetry;
            setTelemetryByDevice((current) => ({
              ...current,
              [telemetry.device_id]: telemetry
            }));
          }
        } catch {
          // Ignore malformed realtime frames.
        }
      };

      socket.onerror = () => {
        socket?.close();
      };

      socket.onclose = scheduleReconnect;
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
