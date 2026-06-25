import { useState } from "react";
import { useParams, Link } from "react-router-dom";
import { ArrowLeft, Wifi, Database, Check, Activity, AlertTriangle, RefreshCw, Send, Settings as SettingsIcon } from "lucide-react";
import { PageHeader, StatusChip, CopyableID, ProgressBar } from "../components/ui";
import { useDevices, useAlerts, useDeployments, useLatestTelemetry } from "../hooks/useArgusData";
import { formatDate } from "../lib/format";

export function DeviceDetailsPage() {
  const { deviceID } = useParams();
  const devices = useDevices();
  const alerts = useAlerts();
  const deployments = useDeployments(deviceID);
  const telemetry = useLatestTelemetry(deviceID);

  const device = devices.data?.find(d => d.id === deviceID);
  const deviceAlerts = (alerts.data ?? []).filter(a => a.device_id === deviceID);
  const deviceDeployments = deployments.data ?? [];

  const [activeTab, setActiveTab] = useState<"deployments" | "incidents" | "telemetry" | "commands" | "config">("deployments");

  if (devices.isLoading) return <div style={{ padding: 24, color: "var(--text-muted)" }}>Loading device...</div>;
  if (!device) return <div style={{ padding: 24, color: "var(--danger)" }}>Device not found</div>;

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Link to="/devices" style={{ display: "inline-flex", alignItems: "center", gap: 6, color: "var(--text-muted)", textDecoration: "none", fontSize: 13 }}>
          <ArrowLeft size={14} /> Back to Devices
        </Link>
      </div>

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 32 }}>
        <div>
          <h1 style={{ fontSize: 24, fontWeight: 600, margin: "0 0 8px 0", display: "flex", alignItems: "center", gap: 12 }}>
            <div className={`status-dot ${device.status}`} style={{ width: 12, height: 12, borderRadius: "50%", background: device.status === "online" ? "var(--success)" : device.status === "warning" ? "var(--warning)" : device.status === "critical" ? "var(--danger)" : "var(--text-muted)" }} />
            {device.name}
          </h1>
          <div style={{ display: "flex", alignItems: "center", gap: 16, fontSize: 13, color: "var(--text-muted)" }}>
            <CopyableID id={device.id} />
            <span style={{ fontFamily: "var(--font-mono)", padding: "2px 6px", background: "var(--surface-2)", borderRadius: "var(--radius-sm)", border: "1px solid var(--border)" }}>{device.type}</span>
            <span>Last seen: {formatDate(device.last_seen)}</span>
          </div>
        </div>
      </div>

      <div style={{ display: "flex", gap: 24, marginBottom: 32, padding: "24px", background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
        <div style={{ flex: 1 }}>
          <div className="muted" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6, fontFamily: "var(--font-mono)" }}>Firmware</div>
          <div style={{ fontSize: 16, fontWeight: 500, fontFamily: "var(--font-mono)" }}>v{device.firmware_version || "unset"}</div>
        </div>
        <div style={{ flex: 1 }}>
          <div className="muted" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6, fontFamily: "var(--font-mono)" }}>Network (RSSI)</div>
          <div style={{ fontSize: 16, fontWeight: 500, display: "flex", alignItems: "center", gap: 8 }}><Wifi size={14} /> -65 dBm</div>
        </div>
        <div style={{ flex: 1 }}>
          <div className="muted" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6, fontFamily: "var(--font-mono)" }}>Memory Heap</div>
          <div style={{ fontSize: 16, fontWeight: 500, display: "flex", alignItems: "center", gap: 8 }}><Database size={14} /> 120 KB</div>
        </div>
        <div style={{ flex: 1 }}>
          <div className="muted" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: 6, fontFamily: "var(--font-mono)" }}>Health</div>
          <div style={{ fontSize: 16, fontWeight: 500, color: device.status === "online" ? "var(--success)" : "var(--danger)" }}>{device.status}</div>
        </div>
      </div>

      <div className="tabs" style={{ display: "flex", gap: 24, borderBottom: "1px solid var(--border)", marginBottom: 24 }}>
        {[
          { id: "deployments", label: "Deployments", icon: RefreshCw },
          { id: "incidents", label: "Incidents", icon: AlertTriangle },
          { id: "telemetry", label: "Telemetry", icon: Activity },
          { id: "commands", label: "Commands", icon: Send },
          { id: "config", label: "Configuration", icon: SettingsIcon },
        ].map(t => (
          <button 
            key={t.id} 
            onClick={() => setActiveTab(t.id as any)}
            style={{ 
              background: "none", border: "none", padding: "0 0 12px 0", cursor: "pointer", 
              fontSize: 14, fontWeight: 500, display: "flex", alignItems: "center", gap: 8,
              color: activeTab === t.id ? "var(--text-primary)" : "var(--text-muted)",
              borderBottom: activeTab === t.id ? "2px solid var(--vercel-cyan)" : "2px solid transparent",
              marginBottom: -1
            }}
          >
            <t.icon size={14} /> {t.label}
          </button>
        ))}
      </div>

      <div>
        {activeTab === "deployments" && (
          <div className="table-wrapper" style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", overflow: "hidden" }}>
            <table className="data-table" style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
              <thead>
                <tr style={{ background: "var(--surface)", borderBottom: "1px solid var(--border)", textAlign: "left", color: "var(--text-muted)" }}>
                  <th style={{ padding: "12px 16px" }}>Version</th>
                  <th style={{ padding: "12px 16px" }}>Started</th>
                  <th style={{ padding: "12px 16px" }}>Status</th>
                </tr>
              </thead>
              <tbody>
                {deviceDeployments.length === 0 ? (
                  <tr><td colSpan={3} style={{ padding: 24, textAlign: "center", color: "var(--text-muted)" }}>No deployments recorded</td></tr>
                ) : (
                  deviceDeployments.map(d => (
                    <tr key={d.id} style={{ borderBottom: "1px solid var(--border)" }}>
                      <td style={{ padding: "12px 16px", fontFamily: "var(--font-mono)" }}>{d.version || "Custom"}</td>
                      <td style={{ padding: "12px 16px", color: "var(--text-muted)" }}>{formatDate(d.created_at)}</td>
                      <td style={{ padding: "12px 16px" }}><StatusChip value={d.status} /></td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
        
        {activeTab === "incidents" && (
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {deviceAlerts.length === 0 ? (
              <div style={{ padding: 24, textAlign: "center", color: "var(--text-muted)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>No incidents recorded</div>
            ) : (
              deviceAlerts.map(a => (
                <div key={a.id} style={{ padding: 16, border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", background: "var(--surface)" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
                    <strong style={{ color: "var(--danger)" }}>{a.metric.toUpperCase()} ALERT</strong>
                    <span className="muted" style={{ fontSize: 12 }}>{formatDate(a.created_at)}</span>
                  </div>
                  <p style={{ margin: 0, fontSize: 13 }}>{a.message}</p>
                </div>
              ))
            )}
          </div>
        )}

        {(activeTab === "telemetry" || activeTab === "commands" || activeTab === "config") && (
          <div style={{ padding: 48, textAlign: "center", border: "1px dashed var(--border)", borderRadius: "var(--radius-lg)" }}>
            <p className="muted">This tab is a placeholder for the respective feature.</p>
          </div>
        )}
      </div>
    </>
  );
}
