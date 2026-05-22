import { RefreshCw } from "lucide-react";
import { Link } from "react-router-dom";
import { EmptyState, ErrorState, LoadingRows, PageHeader, Panel, StatCard, StatusChip } from "../components/ui";
import { useAlerts, useDevices, useFirmware, useHealth } from "../hooks/useArgusData";
import { compactID, countByStatus, formatDate } from "../lib/format";

export function DashboardPage() {
  const devices = useDevices();
  const alerts = useAlerts();
  const firmware = useFirmware();
  const health = useHealth();
  const deviceList = devices.data ?? [];
  const statusCounts = countByStatus(deviceList);

  const refresh = () => {
    void devices.refetch();
    void alerts.refetch();
    void firmware.refetch();
    void health.refetch();
  };

  return (
    <>
      <PageHeader
        eyebrow="Fleet Overview"
        title="Operational dashboard"
        description="Live backend state from the ARGUS API. Metrics render only when records exist."
        actions={
          <button className="button secondary" type="button" onClick={refresh}>
            <RefreshCw size={15} aria-hidden />
            Refresh
          </button>
        }
      />
      <div className="stat-grid">
        <StatCard label="API Health" value={health.isError ? "Error" : health.isLoading ? "Checking" : "Healthy"} tone={health.isError ? "danger" : "success"} />
        <StatCard label="Total Devices" value={deviceList.length} detail="Registered records" />
        <StatCard label="Online" value={statusCounts.online ?? 0} tone="success" />
        <StatCard label="Alerts" value={alerts.data?.length ?? 0} tone={(alerts.data?.length ?? 0) > 0 ? "warning" : "neutral"} />
      </div>
      <div className="split">
        <Panel title="Fleet Devices" subtitle="Registry records from /devices">
          {devices.isError ? (
            <ErrorState message={(devices.error as Error).message} onRetry={() => void devices.refetch()} />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Device</th>
                    <th>Type</th>
                    <th>Status</th>
                    <th>Firmware</th>
                    <th>Last Seen</th>
                  </tr>
                </thead>
                <tbody>
                  {devices.isLoading && <LoadingRows rows={6} />}
                  {!devices.isLoading && deviceList.length === 0 && (
                    <tr>
                      <td colSpan={5}>
                        <EmptyState title="No devices registered" description="Create devices from the Devices page to populate the fleet dashboard." action={<Link className="button primary" to="/devices">Open Devices</Link>} />
                      </td>
                    </tr>
                  )}
                  {deviceList.slice(0, 10).map((device) => (
                    <tr key={device.id}>
                      <td>
                        <strong>{device.name}</strong>
                        <div className="muted mono">{compactID(device.id)}</div>
                      </td>
                      <td>{device.type}</td>
                      <td><StatusChip value={device.status} /></td>
                      <td>{device.firmware_version || "Unset"}</td>
                      <td>{formatDate(device.last_seen)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <div className="grid">
          <Panel title="Recent Alerts" subtitle="Rule alerts from /alerts">
            {alerts.isError ? (
              <ErrorState message={(alerts.error as Error).message} onRetry={() => void alerts.refetch()} />
            ) : alerts.isLoading ? (
              <div className="empty-state">Loading alerts</div>
            ) : (alerts.data?.length ?? 0) === 0 ? (
              <EmptyState title="No alerts" description="No rule alerts have been created by telemetry evaluation." />
            ) : (
              <div className="settings-list">
                {alerts.data?.slice(0, 5).map((alert) => (
                  <div className="settings-row" key={alert.id}>
                    <div>
                      <strong>{alert.metric} {alert.operator} {alert.threshold}</strong>
                      <p className="muted">{alert.message}</p>
                    </div>
                    <span className="muted">{formatDate(alert.created_at)}</span>
                  </div>
                ))}
              </div>
            )}
          </Panel>
          <Panel title="Firmware Artifacts" subtitle="Objects registered through MinIO-backed OTA">
            <StatCard label="Artifacts" value={firmware.data?.length ?? 0} detail={firmware.isError ? (firmware.error as Error).message : "Available for deployments"} />
          </Panel>
        </div>
      </div>
    </>
  );
}
