import { useMemo, useState } from "react";
import { Activity, Terminal, Database, Code, RefreshCw, X, Play, ArrowRight } from "lucide-react";
import { Link } from "react-router-dom";
import { PageHeader, SelectField, EmptyState, Panel, StatusChip, CopyableID } from "../components/ui";
import { useDevices } from "../hooks/useArgusData";
import { useRealtime } from "../hooks/useRealtime";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { safeJsonParse, stringifyJson, formatDate } from "../lib/format";

function SessionRequiredPrompt({ title, description }: { title: string; description: string }) {
  return (
    <div className="onboarding-shell">
      <div className="onboarding-inner" style={{ maxWidth: 500 }}>
        <div className="onboarding-mark"><Play size={32} strokeWidth={1.5} /></div>
        <h2 className="onboarding-title" style={{ fontSize: 32 }}>{title}</h2>
        <p className="onboarding-sub">{description}</p>
        <Link to="/workspaces" className="btn-inverse auth-button" style={{ display: "inline-flex", maxWidth: 260, margin: "0 auto" }}>
          Go to Workspaces <ArrowRight size={15} strokeWidth={1.5} />
        </Link>
      </div>
    </div>
  );
}

export function TelemetryPage() {
  const { activeWorkspace, workspaceDevices } = useWorkspaceContext();
  const devices = useDevices();
  const realtime = useRealtime();
  const [deviceID, setDeviceID] = useState("");
  const [activeTab, setActiveTab] = useState<"metrics" | "logs" | "raw">("metrics");
  const [showRawPayload, setShowRawPayload] = useState(false);

  const selectedDevice = workspaceDevices.find(d => d.id === deviceID);
  const liveTelemetry = deviceID ? realtime.telemetryByDevice[deviceID] || [] : [];
  const latestTelemetry = liveTelemetry[0];
  const latestMetrics = (latestTelemetry?.metrics as any) || {};

  const metricKeys = useMemo(() => {
    return Object.keys(latestMetrics).filter(key => {
      const val = latestMetrics[key];
      return typeof val === "number" || typeof val === "string";
    });
  }, [latestMetrics]);

  if (!activeWorkspace) {
    return <SessionRequiredPrompt title="Workspace Required" description="You must select a workspace to view telemetry data." />;
  }

  return (
    <>
      <PageHeader
        title="Telemetry Explorer"
        description="Drill down into device diagnostics, raw telemetry payloads, and metrics."
      />

      <div style={{ maxWidth: 900 }}>
        <div style={{ marginBottom: 32, padding: "24px", background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
          <h3 style={{ fontSize: 14, fontWeight: 500, margin: "0 0 16px" }}>Select Device</h3>
          <SelectField value={deviceID} onChange={setDeviceID} label="">
            <option value="">Select a device...</option>
            {workspaceDevices.map(d => (
              <option key={d.id} value={d.id}>{d.name} ({d.id.slice(0,8)})</option>
            ))}
          </SelectField>
        </div>

        {!deviceID ? (
          <EmptyState title="No device selected" description="Select a device above to begin exploring telemetry." />
        ) : (
          <>
            <div className="tabs" style={{ display: "flex", gap: 24, borderBottom: "1px solid var(--border)", marginBottom: 24 }}>
              {[
                { id: "metrics", label: "Metrics", icon: Activity },
                { id: "logs", label: "Logs", icon: Terminal },
                { id: "raw", label: "Raw Telemetry", icon: Code }
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

            {activeTab === "metrics" && (
              <div className="grid" style={{ gridTemplateColumns: "1fr 1fr", gap: 16 }}>
                {metricKeys.length === 0 ? (
                  <div style={{ gridColumn: "1 / -1" }}>
                    <EmptyState title="No metrics received" description="Waiting for telemetry payloads from this device." />
                  </div>
                ) : (
                  metricKeys.map(key => (
                    <div key={key} style={{ padding: 16, border: "1px solid var(--border)", borderRadius: "var(--radius-md)", background: "var(--surface)", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                      <span style={{ fontFamily: "var(--font-mono)", fontSize: 13, color: "var(--text-muted)" }}>{key}</span>
                      <span style={{ fontWeight: 600 }}>{latestMetrics[key]}</span>
                    </div>
                  ))
                )}
              </div>
            )}

            {activeTab === "logs" && (
              <div style={{ padding: 24, background: "var(--background)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", fontFamily: "var(--font-mono)", fontSize: 12 }}>
                {liveTelemetry.length === 0 ? (
                  <span className="muted">Waiting for logs...</span>
                ) : (
                  liveTelemetry.slice(0, 50).map((t, i) => (
                    <div key={i} style={{ display: "flex", gap: 16, borderBottom: "1px solid var(--border)", padding: "8px 0" }}>
                      <span className="muted" style={{ width: 140 }}>{formatDate(t.recorded_at)}</span>
                      <span style={{ color: "var(--text-primary)" }}>Received telemetry packet with {Object.keys(t.metrics as any || {}).length} keys</span>
                    </div>
                  ))
                )}
              </div>
            )}

            {activeTab === "raw" && (
              <div style={{ position: "relative" }}>
                <pre className="code-block" style={{ margin: 0, minHeight: 300, fontSize: 12 }}>
                  {latestTelemetry ? stringifyJson(latestTelemetry) : "Waiting for telemetry..."}
                </pre>
              </div>
            )}
          </>
        )}
      </div>
    </>
  );
}
