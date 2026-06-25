import { useMemo } from "react";
import { useAlerts, useAllDeployments, useOTAStats, useFleetIncidents, useAIEvents } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { compactID, formatDate, countByStatus } from "../lib/format";
import type { Deployment } from "../types/api";
import { PageHeader, Card, CardHeader, CardContent, CardTitle, StatusChip } from "../components/ui";
import { AlertTriangle, CheckCircle2, Activity, Package, AlertOctagon } from "lucide-react";

export function FleetOverviewPage() {
  const { workspaceDevices } = useWorkspaceContext();
  const alerts = useAlerts();
  const otaStats = useOTAStats();
  const deployments = useAllDeployments();
  const fleetIncidents = useFleetIncidents();
  const aiEvents = useAIEvents();

  const deviceList = workspaceDevices;
  const statusCounts = countByStatus(deviceList);
  const activeDeviceIds = useMemo(() => new Set(deviceList.map(d => d.id)), [deviceList]);

  const totalDevices = deviceList.length;
  const onlineCount = statusCounts.online ?? 0;
  const warningCount = statusCounts.warning ?? 0;
  const criticalCount = statusCounts.critical ?? 0;
  const offlineCount = statusCounts.offline ?? 0;

  const incidents = fleetIncidents.data ?? [];
  const offlineDevices = deviceList.filter(d => d.status === "offline" || d.status === "critical");
  const activeDeployments = (deployments.data ?? []).filter(d =>
    activeDeviceIds.has(d.device_id) && ["pending", "available", "downloading", "flashing", "rebooting"].includes(d.status)
  );

  const ongoingCount = activeDeployments.length;
  const successRate = otaStats.data?.success_rate ?? 0;

  const deploymentsByVersion = useMemo(() => {
    const map = new Map<string, Deployment[]>();
    for (const d of activeDeployments) {
      const v = d.version || "Custom Build";
      if (!map.has(v)) map.set(v, []);
      map.get(v)!.push(d);
    }
    return Array.from(map.entries());
  }, [activeDeployments]);

  const criticalIncidents = incidents.filter(i => i.severity === "HIGH");

  const events = useMemo(() => {
    return (aiEvents.data ?? [])
      .filter(e => activeDeviceIds.has(e.device_id))
      .slice(0, 15);
  }, [aiEvents.data, activeDeviceIds]);

  return (
    <div className="fleet-overview">
      <PageHeader
        title="Fleet Overview"
        description="Real-time operations command center."
      />

      <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 16 }}>
          <StatCard
            label="Devices Online"
            value={`${onlineCount} / ${totalDevices}`}
            color={onlineCount > 0 ? "#10b981" : undefined}
          />
          <StatCard
            label="OTA Jobs"
            value={`${ongoingCount} active`}
            color={ongoingCount > 0 ? "#f59e0b" : undefined}
          />
          <StatCard
            label="OTA Success"
            value={`${successRate.toFixed(1)}%`}
            detail={`${otaStats.data?.successful_deployments ?? 0} completed`}
            color={successRate >= 95 ? "#10b981" : successRate >= 80 ? "#f59e0b" : "#ef4444"}
          />
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: 24 }}>
          <EventsFeed events={events} />
          <AttentionRequired
            criticalIncidents={criticalIncidents}
            offlineDevices={offlineDevices}
          />
        </div>

        <ActiveDeployments deploymentsByVersion={deploymentsByVersion} />
      </div>
    </div>
  );
}

function StatCard({ label, value, detail, color }: {
  label: string;
  value: string | number;
  detail?: string;
  color?: string;
}) {
  return (
    <div style={{
      background: "var(--surface)",
      border: "1px solid rgba(255,255,255,0.06)",
      borderRadius: "var(--radius-lg)",
      boxShadow: "0 1px 3px 0 rgba(0,0,0,0.3), inset 0 1px 0 0 rgba(255,255,255,0.03)",
      padding: "20px 24px",
      display: "flex",
      flexDirection: "column",
      gap: 4,
    }}>
      <span style={{
        fontFamily: "var(--font-mono)",
        fontSize: 11,
        fontWeight: 500,
        letterSpacing: "0.06em",
        textTransform: "uppercase",
        color: "var(--text-muted)",
      }}>{label}</span>
      <span style={{
        fontFamily: "var(--font-mono)",
        fontSize: 28,
        fontWeight: 600,
        letterSpacing: "-0.02em",
        color: color || "var(--text-primary)",
      }}>{value}</span>
      {detail && (
        <span style={{ fontSize: 13, color: "var(--text-secondary)", marginTop: 2 }}>
          {detail}
        </span>
      )}
    </div>
  );
}

function EventsFeed({ events }: { events: any[] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Activity size={18} />
            RECENT EVENTS
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {events.length === 0 ? (
          <div style={{ textAlign: "center", padding: "48px 0" }}>
            <CheckCircle2 size={32} style={{ margin: "0 auto 16px", color: "#10b981" }} />
            <div style={{ color: "var(--text-muted)" }}>No recent events</div>
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column" }}>
            {events.map(event => {
              const timeStr = new Date(event.created_at).toLocaleTimeString("en-US", {
                hour12: false, hour: "2-digit", minute: "2-digit"
              });
              return (
                <div key={event.id} style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 12,
                  padding: "10px 0",
                  borderBottom: "1px solid rgba(255,255,255,0.04)",
                }}>
                  <span style={{
                    fontFamily: "var(--font-mono)",
                    fontSize: 12,
                    color: "var(--text-muted)",
                    width: 72,
                  }}>{timeStr}</span>
                  <StatusChip value={event.type || "event"} />
                  <span style={{
                    fontFamily: "var(--font-mono)",
                    fontSize: 12,
                    color: "var(--text-secondary)",
                    width: 80,
                  }}>{compactID(event.device_id)}</span>
                  <span style={{
                    fontSize: 13,
                    color: "var(--text-primary)",
                    flex: 1,
                  }}>{event.title || event.summary}</span>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function AttentionRequired({ criticalIncidents, offlineDevices }: {
  criticalIncidents: any[];
  offlineDevices: any[];
}) {
  const hasItems = criticalIncidents.length > 0 || offlineDevices.length > 0;

  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <AlertOctagon size={18} style={{ color: "#ef4444" }} />
            ATTENTION REQUIRED
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {!hasItems ? (
          <div style={{ textAlign: "center", padding: "32px 0" }}>
            <CheckCircle2 size={24} style={{ margin: "0 auto 12px", color: "#10b981" }} />
            <div style={{ color: "#10b981", fontSize: 14 }}>All clear</div>
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            {criticalIncidents.length > 0 && (
              <div>
                <div style={{ color: "#ef4444", fontSize: 12, fontWeight: 600, marginBottom: 8 }}>
                  CRITICAL INCIDENTS ({criticalIncidents.length})
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  {criticalIncidents.slice(0, 5).map(inc => (
                    <div key={inc.device_id + inc.start_time} style={{
                      display: "flex", alignItems: "center", gap: 10,
                      padding: "10px 12px", background: "var(--surface)",
                      borderRadius: "var(--radius-sm)",
                    }}>
                      <AlertTriangle size={14} style={{ color: "#ef4444", flexShrink: 0 }} />
                      <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, width: 80 }}>
                        {compactID(inc.device_id)}
                      </span>
                      <span style={{ fontSize: 12, flex: 1 }}>{inc.summary}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {offlineDevices.length > 0 && (
              <div>
                <div style={{ color: "#ef4444", fontSize: 12, fontWeight: 600, marginBottom: 8 }}>
                  OFFLINE DEVICES ({offlineDevices.length})
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                  {offlineDevices.slice(0, 5).map(device => (
                    <div key={device.id} style={{
                      display: "flex", alignItems: "center", gap: 10,
                      padding: "10px 12px", background: "var(--surface)",
                      borderRadius: "var(--radius-sm)",
                    }}>
                      <AlertOctagon size={14} style={{ color: "#ef4444", flexShrink: 0 }} />
                      <span style={{ fontFamily: "var(--font-mono)", fontSize: 12, width: 80 }}>
                        {device.name}
                      </span>
                      <span style={{ fontSize: 12, flex: 1 }}>Heartbeat lost</span>
                      <span style={{ color: "var(--text-muted)", fontSize: 11 }}>
                        {device.last_seen ? formatDate(device.last_seen).split(",")[1]?.trim() : "never"}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function ActiveDeployments({ deploymentsByVersion }: { deploymentsByVersion: [string, any[]][] }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Package size={18} />
            ACTIVE DEPLOYMENTS
          </div>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {deploymentsByVersion.length === 0 ? (
          <div style={{ textAlign: "center", padding: "32px 0" }}>
            <CheckCircle2 size={24} style={{ margin: "0 auto 12px", color: "#10b981" }} />
            <div style={{ color: "var(--text-muted)" }}>No active deployments</div>
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
            {deploymentsByVersion.map(([version, deps]) => {
              const flashing = deps.filter(d => d.status === "flashing" || d.status === "downloading").length;
              const awaitingAck = deps.filter(d => d.status === "rebooting").length;
              const pending = deps.filter(d => d.status === "pending" || d.status === "available").length;
              const failed = deps.filter(d => d.status === "failed").length;

              return (
                <div key={version} style={{
                  border: "1px solid rgba(255,255,255,0.06)",
                  borderRadius: "var(--radius)",
                  padding: "16px 20px",
                }}>
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 12 }}>
                    <span style={{ fontWeight: 500, fontSize: 14 }}>{version}</span>
                    <span style={{ color: "var(--text-muted)", fontSize: 13 }}>{deps.length} devices</span>
                  </div>
                  <div style={{ display: "flex", gap: 12 }}>
                    {flashing > 0 && (
                      <span style={{ fontSize: 13, color: "var(--text-secondary)" }}>
                        <strong>{flashing}</strong> Flashing
                      </span>
                    )}
                    {awaitingAck > 0 && (
                      <span style={{ fontSize: 13, color: "var(--text-secondary)" }}>
                        <strong style={{ color: "#f59e0b" }}>{awaitingAck}</strong> Awaiting ACK
                      </span>
                    )}
                    {pending > 0 && (
                      <span style={{ fontSize: 13, color: "var(--text-secondary)" }}>
                        <strong>{pending}</strong> Pending
                      </span>
                    )}
                    {failed > 0 && (
                      <span style={{ fontSize: 13, color: "var(--text-secondary)" }}>
                        <strong style={{ color: "#ef4444" }}>{failed}</strong> Failed
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
