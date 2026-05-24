import type { Device, JsonValue } from "../types/api";

const DEVICE_HEARTBEAT_TIMEOUT_MS = 60_000;

export function formatDate(value?: string) {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Invalid date";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}

export function formatBytes(bytes?: number) {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  let size = bytes;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

export function compactID(id?: string, length = 8) {
  if (!id) return "";
  return id.length <= length ? id : id.slice(0, length);
}

export function safeJsonParse(value: string): JsonValue {
  if (!value.trim()) return {};
  return JSON.parse(value) as JsonValue;
}

export function stringifyJson(value?: unknown) {
  if (value === undefined || value === null) return "{}";
  return JSON.stringify(value, null, 2);
}

export function statusTone(status?: string) {
  const normalized = status?.toLowerCase();
  if (normalized === "online" || normalized === "acked" || normalized === "healthy") return "success";
  if (normalized === "warning" || normalized === "pending") return "warning";
  if (normalized === "critical" || normalized === "nacked" || normalized === "error") return "danger";
  return "neutral";
}

export function countByStatus<T extends { status?: string }>(items: T[]) {
  return items.reduce<Record<string, number>>((acc, item) => {
    const key = item.status?.toLowerCase() || "unknown";
    acc[key] = (acc[key] || 0) + 1;
    return acc;
  }, {});
}

export function effectiveDeviceStatus(device: Pick<Device, "status" | "last_seen">, now = Date.now()) {
  const status = device.status?.toLowerCase() || "offline";
  if (status === "offline") return "offline";
  if (!device.last_seen) return "offline";

  const lastSeen = new Date(device.last_seen).getTime();
  if (Number.isNaN(lastSeen)) return "offline";
  if (now - lastSeen > DEVICE_HEARTBEAT_TIMEOUT_MS) return "offline";

  return status;
}

export function withEffectiveDeviceStatus<T extends Device>(device: T, now = Date.now()): T {
  return {
    ...device,
    status: effectiveDeviceStatus(device, now)
  };
}
