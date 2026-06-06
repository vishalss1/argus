import type {
  Alert,
  Command,
  CreateDeviceRequest,
  CreateTelemetryRequest,
  Deployment,
  DeploymentEvent,
  Device,
  DeviceSummary,
  FirmwareArtifact,
  JsonValue,
  Manifest,
  MetricSample,
  Rule,
  Shadow,
  SemanticEvent,
  Incident,
  OperationalMemory,
  OTAFleetStats,
  ReasoningResponse,
  Workspace,
  Session,
  SessionEvent,
  SessionAlert,
  SessionCommand,
  SessionStatistics,
  SessionArtifact,
  DeviceStatusInfo,
  ActiveIncident
} from "../types/api";
import { request } from "./http";

export const api = {
  health: async () => {
    await request<void>("/healthz");
    return { ok: true };
  },
  metrics: () => request<string>("/metrics", { raw: true }),

  devices: {
    list: () => request<Device[]>("/devices/"),
    create: (payload: CreateDeviceRequest) =>
      request<Device>("/devices/", {
        method: "POST",
        body: JSON.stringify(payload)
      }),
    update: (id: string, payload: Partial<CreateDeviceRequest>) =>
      request<Device>(`/devices/${id}/`, {
        method: "PUT",
        body: JSON.stringify(payload)
      }),
    remove: (id: string) => request<void>(`/devices/${id}/`, { method: "DELETE" }),
    heartbeat: (id: string, status?: string) =>
      request<Device>(`/devices/${id}/heartbeat`, {
        method: "POST",
        body: JSON.stringify(status ? { status } : {})
      })
  },

  telemetry: {
    ingest: (deviceID: string, payload: CreateTelemetryRequest) =>
      request(`/devices/${deviceID}/telemetry`, {
        method: "POST",
        body: JSON.stringify(payload)
      }),
    latest: (deviceID: string) => request<any>(`/devices/${deviceID}/telemetry/latest`)
  },


  workspaces: {
    list: () => request<Workspace[]>("/workspaces"),
    create: (name: string, description: string) =>
      request<Workspace>("/workspaces", {
        method: "POST",
        body: JSON.stringify({ name, description })
      }),
    get: (id: string) => request<Workspace>(`/workspaces/${id}`),
    delete: (id: string) =>
      request<void>(`/workspaces/${id}`, {
        method: "DELETE"
      }),
    update: (id: string, name: string, description: string) =>
      request<Workspace>(`/workspaces/${id}`, {
        method: "PUT",
        body: JSON.stringify({ name, description })
      }),
    listDevices: (workspaceID: string) =>
      request<DeviceSummary[]>(`/workspaces/${workspaceID}/devices`),
    assignDevice: (workspaceID: string, deviceID: string) =>
      request<void>(`/workspaces/${workspaceID}/devices`, {
        method: "POST",
        body: JSON.stringify({ device_id: deviceID })
      }),
    unassignDevice: (workspaceID: string, deviceID: string) =>
      request<void>(`/workspaces/${workspaceID}/devices/${deviceID}`, {
        method: "DELETE"
      })
  },

  sessions: {
    create: (workspaceID: string) => request<Session>(`/workspaces/${workspaceID}/sessions`, { method: "POST" }),
    list: (workspaceID: string) => request<Session[]>(`/workspaces/${workspaceID}/sessions`),
    get: (id: string) => request<Session>(`/sessions/${id}`),
    getStatistics: (id: string) => request<SessionStatistics>(`/sessions/${id}/statistics`),
    start: (id: string) => request<Session>(`/sessions/${id}/start`, { method: "POST" }),
    stop: (id: string, success: boolean) =>
      request<Session>(`/sessions/${id}/stop`, {
        method: "POST",
        body: JSON.stringify({ success })
      }),
    getArtifact: (id: string) => request<SessionArtifact>(`/sessions/${id}/artifact`)
  },

  commands: {
    list: (deviceID: string) => request<Command[]>(`/devices/${deviceID}/commands`),
    send: (deviceID: string, payload: { type: string; payload?: JsonValue }) =>
      request<Command>(`/devices/${deviceID}/commands`, {
        method: "POST",
        body: JSON.stringify(payload)
      })
  },

  firmware: {
    list: () => request<FirmwareArtifact[]>("/ota/firmware/"),
    upload: (form: FormData) =>
      request<FirmwareArtifact>("/ota/firmware/", {
        method: "POST",
        body: form
      })
  },

  deployments: {
    listAll: () => request<Deployment[]>("/ota/deployments/"),
    stats: () => request<OTAFleetStats>("/ota/deployments/stats"),
    list: (deviceID: string) => request<Deployment[]>(`/devices/${deviceID}/ota`),
    create: (deviceID: string, artifactID: string) =>
      request<Manifest>(`/devices/${deviceID}/ota`, {
        method: "POST",
        body: JSON.stringify({ artifact_id: artifactID })
      }),
    manifest: (deviceID: string, deploymentID: string) =>
      request<Manifest>(`/devices/${deviceID}/ota/${deploymentID}/manifest`),
    ack: (deviceID: string, deploymentID: string, message?: string) =>
      request<Deployment>(`/devices/${deviceID}/ota/${deploymentID}/ack`, {
        method: "POST",
        body: JSON.stringify(message ? { message } : {})
      }),
    nack: (deviceID: string, deploymentID: string, message?: string) =>
      request<Deployment>(`/devices/${deviceID}/ota/${deploymentID}/nack`, {
        method: "POST",
        body: JSON.stringify(message ? { message } : {})
      }),
    events: (deploymentID: string) => request<DeploymentEvent[]>(`/ota/deployments/${deploymentID}/events`)
  },

  rules: {
    list: () => request<Rule[]>("/rules/"),
    create: (payload: Pick<Rule, "name" | "metric" | "operator" | "threshold" | "enabled">) =>
      request<Rule>("/rules/", {
        method: "POST",
        body: JSON.stringify(payload)
      }),
    update: (id: string, payload: Partial<Pick<Rule, "name" | "metric" | "operator" | "threshold" | "enabled">>) =>
      request<Rule>(`/rules/${id}/`, {
        method: "PUT",
        body: JSON.stringify(payload)
      }),
    remove: (id: string) => request<void>(`/rules/${id}/`, { method: "DELETE" })
  },

  alerts: {
    list: () => request<Alert[]>("/alerts")
  },

  shadows: {
    get: (deviceID: string) => request<Shadow>(`/devices/${deviceID}/shadow`),
    updateDesired: (deviceID: string, state: JsonValue) =>
      request<Shadow>(`/devices/${deviceID}/shadow/desired`, {
        method: "PUT",
        body: JSON.stringify({ state })
      }),
    updateReported: (deviceID: string, state: JsonValue) =>
      request<Shadow>(`/devices/${deviceID}/shadow/reported`, {
        method: "PUT",
        body: JSON.stringify({ state })
      })
  },

  ai: {
    listEvents: (limit?: number, offset?: number) => {
      const q = new URLSearchParams();
      if (limit !== undefined) q.append("limit", String(limit));
      if (offset !== undefined) q.append("offset", String(offset));
      const qs = q.toString();
      return request<SemanticEvent[]>(`/ai/events${qs ? "?" + qs : ""}`);
    },
    listDeviceEvents: (deviceID: string, limit?: number, offset?: number) => {
      const q = new URLSearchParams();
      if (limit !== undefined) q.append("limit", String(limit));
      if (offset !== undefined) q.append("offset", String(offset));
      const qs = q.toString();
      return request<SemanticEvent[]>(`/devices/${deviceID}/ai/events${qs ? "?" + qs : ""}`);
    },
    getDeviceHistory: (deviceID: string, limit?: number, offset?: number) => {
      const q = new URLSearchParams();
      if (limit !== undefined) q.append("limit", String(limit));
      if (offset !== undefined) q.append("offset", String(offset));
      const qs = q.toString();
      return request<OperationalMemory[]>(`/devices/${deviceID}/ai/history${qs ? "?" + qs : ""}`);
    },
    getDeviceStatus: (deviceID: string) => request<DeviceStatusInfo>(`/devices/${deviceID}/status`),
    getSessionActiveIncidents: (sessionID: string) => request<ActiveIncident[]>(`/sessions/${sessionID}/active-incidents`),
    getFleetIncidents: () => request<ActiveIncident[]>("/fleet/incidents"),
    query: (query: string) => request<ReasoningResponse>("/ai/query", {
      method: "POST",
      body: JSON.stringify({ query })
    })
  }
};

export function parsePrometheusMetrics(text: string): MetricSample[] {
  return text
    .split("\n")
    .filter((line) => line && !line.startsWith("#"))
    .map((line) => {
      const [series, rawValue] = line.trim().split(/\s+/);
      const nameMatch = series.match(/^([^{]+)(?:{(.+)})?$/);
      const labels: Record<string, string> = {};

      if (nameMatch?.[2]) {
        for (const label of nameMatch[2].split(",")) {
          const [key, value] = label.split("=");
          labels[key] = value?.replace(/^"|"$/g, "") ?? "";
        }
      }

      return {
        name: nameMatch?.[1] ?? series,
        labels,
        value: Number(rawValue)
      };
    })
    .filter((sample) => Number.isFinite(sample.value));
}
