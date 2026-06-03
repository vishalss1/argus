import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../services/api";
import { withEffectiveDeviceStatus } from "../lib/format";

export const queryKeys = {
  devices: ["devices"] as const,
  alerts: ["alerts"] as const,
  rules: ["rules"] as const,
  firmware: ["firmware"] as const,
  allDeployments: ["deployments"] as const,
  otaStats: ["ota", "stats"] as const,
  health: ["health"] as const,
  metrics: ["metrics"] as const,
  commands: (deviceID: string) => ["commands", deviceID] as const,
  deployments: (deviceID: string) => ["deployments", deviceID] as const,
  deploymentEvents: (deploymentID: string) => ["deployment-events", deploymentID] as const,
  shadow: (deviceID: string) => ["shadow", deviceID] as const,
  fleetSummary: ["fleet", "summary"] as const,
  fleetHistory: ["fleet", "history"] as const,
  latestTelemetry: (deviceID: string) => ["telemetry", "latest", deviceID] as const,
  aiFindings: (deviceID: string) => ["ai", "findings", deviceID] as const,
  workspaces: ["workspaces"] as const,
  workspace: (id: string) => ["workspaces", id] as const,
  workspaceDevices: (id: string) => ["workspaces", id, "devices"] as const,
  sessions: (workspaceID: string) => ["sessions", workspaceID] as const,
  session: (id: string) => ["session", id] as const,
  sessionStats: (id: string) => ["session", id, "stats"] as const,
  sessionReport: (id: string) => ["session", id, "report"] as const,
  sessionArtifact: (id: string) => ["session", id, "artifact"] as const
};

export function useWorkspaces() {
  return useQuery({ queryKey: queryKeys.workspaces, queryFn: api.workspaces.list });
}

export function useWorkspace(id: string) {
  return useQuery({
    queryKey: queryKeys.workspace(id),
    queryFn: () => api.workspaces.get(id),
    enabled: Boolean(id)
  });
}

export function useCreateWorkspace() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, description }: { name: string; description: string }) =>
      api.workspaces.create(name, description),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: queryKeys.workspaces })
  });
}

export function useWorkspaceDevices(workspaceID?: string) {
  return useQuery({
    queryKey: queryKeys.workspaceDevices(workspaceID ?? ""),
    queryFn: () => api.workspaces.listDevices(workspaceID!),
    enabled: Boolean(workspaceID)
  });
}

export function useAssignDevice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceID, deviceID }: { workspaceID: string; deviceID: string }) =>
      api.workspaces.assignDevice(workspaceID, deviceID),
    onSuccess: (_, { workspaceID }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaceDevices(workspaceID) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces });
      void queryClient.invalidateQueries({ queryKey: queryKeys.devices });
    }
  });
}

export function useUnassignDevice() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceID, deviceID }: { workspaceID: string; deviceID: string }) =>
      api.workspaces.unassignDevice(workspaceID, deviceID),
    onSuccess: (_, { workspaceID }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaceDevices(workspaceID) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.workspaces });
      void queryClient.invalidateQueries({ queryKey: queryKeys.devices });
    }
  });
}

export function useSessions(workspaceID: string) {
  return useQuery({
    queryKey: queryKeys.sessions(workspaceID),
    queryFn: () => api.sessions.list(workspaceID),
    enabled: Boolean(workspaceID),
    refetchInterval: 5_000
  });
}

export function useCreateSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (workspaceID: string) => api.sessions.create(workspaceID),
    onSuccess: (_, variables) => queryClient.invalidateQueries({ queryKey: queryKeys.sessions(variables) })
  });
}

export function useStartSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (sessionID: string) => api.sessions.start(sessionID),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["sessions"] })
  });
}

export function useStopSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ sessionID, success }: { sessionID: string; success: boolean }) => api.sessions.stop(sessionID, success),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["sessions"] })
  });
}

export function useSession(sessionID: string) {
  return useQuery({
    queryKey: queryKeys.session(sessionID),
    queryFn: () => api.sessions.get(sessionID),
    enabled: Boolean(sessionID)
  });
}

export function useSessionStatistics(sessionID: string) {
  return useQuery({
    queryKey: queryKeys.sessionStats(sessionID),
    queryFn: () => api.sessions.getStatistics(sessionID),
    enabled: Boolean(sessionID)
  });
}

export function useSessionReport(sessionID: string) {
  return useQuery({
    queryKey: queryKeys.sessionReport(sessionID),
    queryFn: () => api.sessions.getReport(sessionID),
    enabled: Boolean(sessionID)
  });
}

export function useSessionArtifact(sessionID: string) {
  return useQuery({
    queryKey: queryKeys.sessionArtifact(sessionID),
    queryFn: () => api.sessions.getArtifact(sessionID),
    enabled: Boolean(sessionID)
  });
}

export function useFleetSummary() {
  return useQuery({ queryKey: queryKeys.fleetSummary, queryFn: api.fleet.summary, refetchInterval: 60_000 });
}

export function useFleetHistory() {
  return useQuery({ queryKey: queryKeys.fleetHistory, queryFn: api.fleet.history });
}

export function useLatestTelemetry(deviceID?: string) {
  return useQuery({
    queryKey: queryKeys.latestTelemetry(deviceID ?? ""),
    queryFn: () => api.telemetry.latest(deviceID!),
    enabled: Boolean(deviceID),
    refetchInterval: 5_000
  });
}

export function useAIFindings(deviceID?: string) {
  return useQuery({
    queryKey: queryKeys.aiFindings(deviceID ?? ""),
    queryFn: () => api.ai.getFindings(deviceID!),
    enabled: Boolean(deviceID)
  });
}

export function useDevices({ realtime = false }: { realtime?: boolean } = {}) {
  void realtime;
  return useQuery({
    queryKey: queryKeys.devices,
    queryFn: api.devices.list,
    refetchInterval: false,
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

export function useAllDeployments() {
  return useQuery({ queryKey: queryKeys.allDeployments, queryFn: api.deployments.listAll });
}

export function useOTAStats() {
  return useQuery({ queryKey: queryKeys.otaStats, queryFn: api.deployments.stats });
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

export function useDeploymentEvents(deploymentID?: string) {
  return useQuery({
    queryKey: queryKeys.deploymentEvents(deploymentID ?? ""),
    queryFn: () => api.deployments.events(deploymentID!),
    enabled: Boolean(deploymentID)
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
      client.invalidateQueries({ queryKey: queryKeys.firmware }),
      client.invalidateQueries({ queryKey: queryKeys.allDeployments }),
      client.invalidateQueries({ queryKey: queryKeys.otaStats })
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
