import { type ReactNode, useMemo, useState } from "react";
import { RefreshCw } from "lucide-react";
import { Link } from "react-router-dom";
import {
  EmptyState,
  ErrorState,
  EventStreamEntry,
  FilterTabs,
  CopyableID,
  LoadingRows,
  PageHeader,
  Pagination,
  Panel,
  ProgressBar,
  SignalStrength,
  StatCard,
  StatusChip
} from "../components/ui";
import { useAlerts, useDevices, useFirmware, useHealth } from "../hooks/useArgusData";
import { useRealtime } from "../hooks/useRealtime";
import { countByStatus, formatDate } from "../lib/format";
import type { JsonValue } from "../types/api";

const FILTERS = ["All", "Online", "Warning", "Critical", "Offline"];
const ROWS_PER_PAGE = 12;

function deviceRegion(metadata: any) {
  if (metadata && typeof metadata === "object" && typeof metadata.region === "string") {
    return metadata.region;
  }
  return "Unknown";
}

interface SignalReading {
  level: number;
  label: string;
}

function toNumber(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value.trim().replace(/dbm$/i, ""));
    if (Number.isFinite(parsed)) return parsed;
  }
  return undefined;
}

function objectValue(value: JsonValue | undefined, key: string): unknown {
  if (!value || typeof value !== "object" || Array.isArray(value)) return undefined;
  const record = value as Record<string, unknown>;
  const foundKey = Object.keys(record).find((item) => item.toLowerCase() === key.toLowerCase());
  return foundKey ? record[foundKey] : undefined;
}

function nestedObject(value: JsonValue | undefined, key: string): JsonValue | undefined {
  const nested = objectValue(value, key);
  if (!nested || typeof nested !== "object" || Array.isArray(nested)) return undefined;
  return nested as JsonValue;
}

function signalMetricValue(source: JsonValue | undefined): number | undefined {
  const directKeys = ["rssi", "wifi_rssi", "signal", "signal_strength", "signalStrength", "rssi_dbm"];
  for (const key of directKeys) {
    const value = toNumber(objectValue(source, key));
    if (value !== undefined) return value;
  }

  const wifi = nestedObject(source, "wifi");
  const wireless = nestedObject(source, "wireless");
  return toNumber(objectValue(wifi, "rssi")) ?? toNumber(objectValue(wifi, "signal")) ?? toNumber(objectValue(wireless, "rssi"));
}

function convertToSignalLevel(val?: number): number {
  if (val === undefined || val === null || !Number.isFinite(val)) return 0;
  
  // If it's a percentage or 0-4 level
  if (val >= 0) {
    if (val <= 4) return Math.round(val);
    if (val >= 80) return 4;
    if (val >= 60) return 3;
    if (val >= 40) return 2;
    if (val >= 20) return 1;
    return 0;
  }
  
  // If it's a negative dBm (RSSI)
  if (val >= -50) return 4;
  if (val >= -65) return 3;
  if (val >= -75) return 2;
  if (val >= -85) return 1;
  return 0;
}

function formatSignalLabel(value: number): string {
  if (value < 0) return `${Math.round(value)} dBm`;
  if (value <= 4) return `${Math.round(value)}/4`;
  return `${Math.round(value)}%`;
}

function extractSignalReading(device: { metadata?: JsonValue }, liveTelemetry?: { metrics?: JsonValue }): SignalReading {
  const value = signalMetricValue(liveTelemetry?.metrics) ?? signalMetricValue(device.metadata);
  if (value === undefined) return { level: 0, label: "No signal" };
  return {
    level: convertToSignalLevel(value),
    label: formatSignalLabel(value)
  };
}

export function DashboardPage() {
  const devices = useDevices({ realtime: true });
  const { telemetryByDevice } = useRealtime();
  const alerts = useAlerts();
  const firmware = useFirmware();
  const health = useHealth();
  const deviceList = devices.data ?? [];
  const statusCounts = countByStatus(deviceList);
  const [filter, setFilter] = useState("All");
  const [page, setPage] = useState(1);

  const filtered = useMemo(() => {
    if (filter === "All") return deviceList;
    return deviceList.filter((d) => d.status?.toLowerCase() === filter.toLowerCase());
  }, [deviceList, filter]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / ROWS_PER_PAGE));
  const paged = filtered.slice((page - 1) * ROWS_PER_PAGE, page * ROWS_PER_PAGE);

  const refresh = () => {
    void devices.refetch();
    void alerts.refetch();
    void firmware.refetch();
    void health.refetch();
  };

  const eventEntries = useMemo(() => {
    const entries: { time: string; type: string; detail: ReactNode }[] = [];
    if (alerts.data) {
      alerts.data.slice(0, 6).forEach((alert) => {
        const time = new Date(alert.created_at).toLocaleTimeString("en-US", { hour12: false, hour: "2-digit", minute: "2-digit", second: "2-digit" });
        entries.push({
          time,
          type: "ALERT",
          detail: (
            <>
              <CopyableID id={alert.device_id} length={8} /> {alert.message}
            </>
          )
        });
      });
    }
    return entries;
  }, [alerts.data]);

  const totalDevices = deviceList.length;
  const onlineCount = statusCounts.online ?? 0;
  const warningCount = statusCounts.warning ?? 0;
  const criticalCount = statusCounts.critical ?? 0;
  const offlineCount = statusCounts.offline ?? 0;
  const otaCount = firmware.data?.length ?? 0;

  return (
    <>
      <PageHeader
        title="Fleet Overview"
        actions={
          <button className="button secondary" type="button" onClick={refresh}>
            <RefreshCw size={15} aria-hidden />
            Refresh
          </button>
        }
      />
      <div className="stat-grid five">
        <StatCard
          label="Total Devices"
          value={totalDevices.toLocaleString()}
          detail="Total Devices"
        />
        <StatCard
          label="Online"
          value={onlineCount.toLocaleString()}
          detail="Online"
          tone="success"
        />
        <StatCard
          label="Warnings"
          value={warningCount.toLocaleString()}
          detail="Warnings"
          tone="warning"
        />
        <StatCard
          label="Critical"
          value={criticalCount.toLocaleString()}
          detail="Critical"
          tone="danger"
        />
        <StatCard
          label="Active OTA Jobs"
          value={otaCount}
          detail="Active OTA Jobs"
        />
      </div>
      <div className="split">
        <Panel
          title="Fleet Devices"
          subtitle={`${filtered.length.toLocaleString()} total`}
          actions={<FilterTabs options={FILTERS} active={filter} onChange={(v) => { setFilter(v); setPage(1); }} />}
        >
          {devices.isError ? (
            <ErrorState message={(devices.error as Error).message} onRetry={() => void devices.refetch()} />
          ) : (
            <>
              <div className="table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Device ID</th>
                      <th>Name</th>
                      <th>Region</th>
                      <th>Status</th>
                      <th>Firmware</th>
                      <th>Last Seen</th>
                      <th>Signal</th>
                    </tr>
                  </thead>
                  <tbody>
                    {devices.isLoading && <LoadingRows rows={6} />}
                    {!devices.isLoading && paged.length === 0 && (
                      <tr>
                        <td colSpan={7}>
                          <EmptyState
                            title="No devices registered"
                            description="Create devices from the Devices page to populate the fleet dashboard."
                            action={<Link className="button primary" to="/devices">Open Devices</Link>}
                          />
                        </td>
                      </tr>
                    )}
                    {paged.map((device) => {
                      const telemetryList = telemetryByDevice[device.id];
                      const telemetry = telemetryList && telemetryList.length > 0 ? telemetryList[0] : undefined;
                      const signal = extractSignalReading(device, telemetry);
                      return (
                        <tr key={device.id}>
                          <td><CopyableID id={device.id} length={8} /></td>
                          <td><strong>{device.name}</strong></td>
                          <td className="muted">{deviceRegion(device.metadata)}</td>
                          <td><StatusChip value={device.status} /></td>
                          <td>{device.firmware_version || "Unset"}</td>
                          <td className="muted">{formatDate(device.last_seen)}</td>
                          <td>
                            <span className="signal-cell">
                              <SignalStrength strength={signal.level} />
                              <span className="muted">{signal.label}</span>
                            </span>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
              <div className="muted" style={{ padding: "10px 0 4px", fontSize: 12 }}>
                Showing {paged.length} of {filtered.length.toLocaleString()} devices
              </div>
              <Pagination current={page} total={totalPages} onChange={setPage} />
            </>
          )}
        </Panel>
        <div className="grid">
          <Panel title="Recent Alerts" actions={<Link to="/alerts" className="muted" style={{ fontSize: 12 }}>View All</Link>}>
            {eventEntries.length === 0 ? (
              <EmptyState title="No alerts" description="No alerts have been generated." />        
            ) : (
              <div className="event-stream">
                {eventEntries.map((entry, i) => (
                  <EventStreamEntry key={i} time={entry.time} type={entry.type} detail={entry.detail} />
                ))}
              </div>
            )}
          </Panel>
          <Panel title="Fleet Health" actions={<button className="button compact secondary" type="button" onClick={refresh}>Refresh</button>}>
            <div className="health-list">
              <div className="health-row">
                <div className="health-meta">
                  <span>Online</span>
                  <strong style={{ color: "var(--success)" }}>{onlineCount.toLocaleString()}</strong>
                </div>
                <ProgressBar value={onlineCount} max={Math.max(totalDevices, 1)} color="var(--success)" />
                <span className="muted" style={{ fontSize: 11 }}>{totalDevices > 0 ? ((onlineCount / totalDevices) * 100).toFixed(1) : 0}%</span>
              </div>
              <div className="health-row">
                <div className="health-meta">
                  <span>Warning</span>
                  <strong style={{ color: "var(--warning)" }}>{warningCount.toLocaleString()}</strong>
                </div>
                <ProgressBar value={warningCount} max={Math.max(totalDevices, 1)} color="var(--warning)" />
                <span className="muted" style={{ fontSize: 11 }}>{totalDevices > 0 ? ((warningCount / totalDevices) * 100).toFixed(1) : 0}%</span>
              </div>
              <div className="health-row">
                <div className="health-meta">
                  <span>Critical</span>
                  <strong style={{ color: "var(--danger)" }}>{criticalCount.toLocaleString()}</strong>
                </div>
                <ProgressBar value={criticalCount} max={Math.max(totalDevices, 1)} color="var(--danger)" />
                <span className="muted" style={{ fontSize: 11 }}>{totalDevices > 0 ? ((criticalCount / totalDevices) * 100).toFixed(1) : 0}%</span>
              </div>
              <div className="health-row">
                <div className="health-meta">
                  <span>Offline</span>
                  <strong>{offlineCount}</strong>
                </div>
                <ProgressBar value={offlineCount} max={Math.max(totalDevices, 1)} color="var(--line)" />
                <span className="muted" style={{ fontSize: 11 }}>{totalDevices > 0 ? ((offlineCount / totalDevices) * 100).toFixed(1) : 0}%</span>
              </div>
            </div>
          </Panel>
        </div>
      </div>
    </>
  );
}
