import type {
  Alert,
  Command,
  CreateDeviceRequest,
  CreateTelemetryRequest,
  Deployment,
  Device,
  FirmwareArtifact,
  JsonValue,
  Manifest,
  MetricSample,
  Rule,
  Shadow
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
      })
  },

  commands: {
    list: (deviceID: string) => request<Command[]>(`/devices/${deviceID}/commands`),
    send: (deviceID: string, payload: { type: string; payload?: JsonValue }) =>
      request<Command>(`/devices/${deviceID}/commands`, {
        method: "POST",
        body: JSON.stringify(payload)
      }),
    ack: (deviceID: string, commandID: string, message?: string) =>
      request<Command>(`/devices/${deviceID}/commands/${commandID}/ack`, {
        method: "POST",
        body: JSON.stringify(message ? { message } : {})
      }),
    nack: (deviceID: string, commandID: string, message?: string) =>
      request<Command>(`/devices/${deviceID}/commands/${commandID}/nack`, {
        method: "POST",
        body: JSON.stringify(message ? { message } : {})
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
      })
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
