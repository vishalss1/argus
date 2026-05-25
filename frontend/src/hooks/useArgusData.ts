import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../services/api";
import { withEffectiveDeviceStatus } from "../lib/format";

export const queryKeys = {
  devices: ["devices"] as const,
  alerts: ["alerts"] as const,
  rules: ["rules"] as const,
  firmware: ["firmware"] as const,
  health: ["health"] as const,
  metrics: ["metrics"] as const,
  commands: (deviceID: string) => ["commands", deviceID] as const,
  deployments: (deviceID: string) => ["deployments", deviceID] as const,
  shadow: (deviceID: string) => ["shadow", deviceID] as const
};

export function useDevices({ realtime = false }: { realtime?: boolean } = {}) {
  return useQuery({
    queryKey: queryKeys.devices,
    queryFn: api.devices.list,
    refetchInterval: realtime ? false : 15_000,
    select: (devices) => {
      const now = Date.now();
      return devices.map((device) => withEffectiveDeviceStatus(device, now));
    }
  });
}

export function useAlerts() {
  return useQuery({ queryKey: queryKeys.alerts, queryFn: api.alerts.list });
}

export function useRules() {
  return useQuery({ queryKey: queryKeys.rules, queryFn: api.rules.list });
}

export function useFirmware() {
  return useQuery({ queryKey: queryKeys.firmware, queryFn: api.firmware.list });
}

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health,
    queryFn: api.health,
    retry: false,
    refetchInterval: 30_000
  });
}

export function useMetrics() {
  return useQuery({
    queryKey: queryKeys.metrics,
    queryFn: api.metrics,
    refetchInterval: 30_000
  });
}

export function useCommands(deviceID?: string) {
  return useQuery({
    queryKey: queryKeys.commands(deviceID ?? ""),
    queryFn: () => api.commands.list(deviceID!),
    enabled: Boolean(deviceID)
  });
}

export function useDeployments(deviceID?: string) {
  return useQuery({
    queryKey: queryKeys.deployments(deviceID ?? ""),
    queryFn: () => api.deployments.list(deviceID!),
    enabled: Boolean(deviceID)
  });
}

export function useShadow(deviceID?: string) {
  return useQuery({
    queryKey: queryKeys.shadow(deviceID ?? ""),
    queryFn: () => api.shadows.get(deviceID!),
    enabled: Boolean(deviceID),
    retry: false
  });
}

export function useInvalidateFleet() {
  const client = useQueryClient();
  return () =>
    Promise.all([
      client.invalidateQueries({ queryKey: queryKeys.devices }),
      client.invalidateQueries({ queryKey: queryKeys.alerts }),
      client.invalidateQueries({ queryKey: queryKeys.rules }),
      client.invalidateQueries({ queryKey: queryKeys.firmware })
    ]);
}

export function useCreateDevice() {
  const invalidate = useInvalidateFleet();
  return useMutation({
    mutationFn: api.devices.create,
    onSuccess: invalidate
  });
}

export function useUpdateDevice() {
  const invalidate = useInvalidateFleet();
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: Partial<Parameters<typeof api.devices.create>[0]> }) => api.devices.update(id, payload),
    onSuccess: invalidate
  });
}

export function useUpdateRule() {
  const invalidate = useInvalidateFleet();
  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: Parameters<typeof api.rules.update>[1] }) => api.rules.update(id, payload),
    onSuccess: invalidate
  });
}

export function useHeartbeat() {
  const invalidate = useInvalidateFleet();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status?: string }) => api.devices.heartbeat(id, status),
    onSuccess: invalidate
  });
}
