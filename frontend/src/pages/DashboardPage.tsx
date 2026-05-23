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
import { countByStatus, formatDate } from "../lib/format";

const FILTERS = ["All", "Online", "Warning", "Critical", "Offline"];
const ROWS_PER_PAGE = 12;

function deviceRegion(metadata: any) {
  if (metadata && typeof metadata === "object" && typeof metadata.region === "string") {
    return metadata.region;
  }
  return "Unknown";
}

function deviceSignal(metadata: any) {
  if (metadata && typeof metadata === "object" && typeof metadata.signal === "number") {
    return metadata.signal;
  }
  return 0;
}

export function DashboardPage() {
  const devices = useDevices();
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
                    {paged.map((device) => (
                      <tr key={device.id}>
                        <td><CopyableID id={device.id} length={8} /></td>
                        <td><strong>{device.name}</strong></td>
                        <td className="muted">{deviceRegion(device.metadata)}</td>
                        <td><StatusChip value={device.status} /></td>
                        <td>{device.firmware_version || "Unset"}</td>
                        <td className="muted">{formatDate(device.last_seen)}</td>
                        <td><SignalStrength strength={deviceSignal(device.metadata)} /></td>
                      </tr>
                    ))}
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
