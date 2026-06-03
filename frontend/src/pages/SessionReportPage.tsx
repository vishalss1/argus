import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { 
  ArrowLeft, 
  Download, 
  Cpu, 
  Battery, 
  Thermometer, 
  AlertTriangle, 
  Navigation, 
  FileJson, 
  Terminal, 
  CheckCircle2, 
  XCircle 
} from "lucide-react";
import { 
  useSession, 
  useSessionStatistics, 
  useSessionReport 
} from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard, LoadingRows, ErrorState } from "../components/ui";
import { api } from "../services/api";

export function SessionReportPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const [exportingFormat, setExportingFormat] = useState<"json" | "csv" | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data: session, isLoading: sessionLoading, error: sessionErr } = useSession(sessionID ?? "");
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");
  const { data: report, isLoading: reportLoading, error: reportErr } = useSessionReport(sessionID ?? "");

  const handleExport = async (format: "json" | "csv") => {
    if (!sessionID) return;
    setExportingFormat(format);
    setExportError(null);
    try {
      const resp = await api.sessions.export(sessionID, format);
      if (resp && resp.download_url) {
        // Trigger file download
        const link = document.createElement("a");
        link.href = resp.download_url;
        link.setAttribute("download", `session_report_${sessionID}.${format}`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
      } else {
        throw new Error("Invalid export response from server");
      }
    } catch (err: any) {
      console.error(err);
      setExportError(`Failed to export as ${format.toUpperCase()}: ${err.message || err}`);
    } finally {
      setExportingFormat(null);
    }
  };

  if (!sessionID) {
    return <ErrorState message="No Session ID provided." onRetry={() => navigate("/workspaces")} />;
  }

  const isLoading = sessionLoading || statsLoading || reportLoading;
  const hasError = sessionErr || statsErr || reportErr;

  if (isLoading) {
    return (
      <div style={{ padding: "24px" }}>
        <LoadingRows rows={8} />
      </div>
    );
  }

  if (hasError || !session || !stats) {
    return (
      <ErrorState 
        message="Failed to load session report data. Ensure the session was completed or failed successfully." 
        onRetry={() => navigate("/workspaces")} 
      />
    );
  }

  // Parse report_json safely
  let reportData: any = {};
  if (report && report.report_json) {
    try {
      reportData = typeof report.report_json === "string" 
        ? JSON.parse(report.report_json) 
        : report.report_json;
    } catch (e) {
      console.error("Failed to parse report_json", e);
    }
  }

  const summary = reportData.summary || "No summary text generated for this session.";
  const reportVersion = reportData.report_version || "1.0";
  const metrics = reportData.aggregated_metrics || {};
  const devicesDetail = metrics.devices_detail || {};

  // Formatter helpers
  const formatSeconds = (sec: number) => {
    if (sec < 60) return `${sec}s`;
    const min = Math.floor(sec / 60);
    const s = sec % 60;
    if (min < 60) return `${min}m ${s}s`;
    const hr = Math.floor(min / 60);
    const m = min % 60;
    return `${hr}h ${m}m ${s}s`;
  };

  const formatTime = (isoString?: string) => {
    if (!isoString) return "-";
    return new Date(isoString).toLocaleString();
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case "COMPLETED": return "var(--success)";
      case "FAILED": return "var(--danger)";
      default: return "var(--faint)";
    }
  };

  return (
    <>
      <div style={{ marginBottom: "16px" }}>
        <button 
          onClick={() => navigate("/workspaces")} 
          style={{ 
            display: "inline-flex", 
            alignItems: "center", 
            gap: "6px", 
            background: "none", 
            border: "none", 
            color: "var(--accent)", 
            cursor: "pointer", 
            padding: 0,
            fontSize: "14px"
          }}
        >
          <ArrowLeft size={16} /> Back to Workspaces
        </button>
      </div>

      <PageHeader
        title={`Session Report`}
        description={`Archived statistical outcome for session ${sessionID}`}
        actions={
          <div style={{ display: "flex", gap: "12px" }}>
            <button 
              className="button secondary compact" 
              onClick={() => handleExport("json")}
              disabled={exportingFormat !== null}
            >
              <FileJson size={14} style={{ marginRight: 6 }} />
              {exportingFormat === "json" ? "Exporting..." : "Export JSON"}
            </button>
            <button 
              className="button primary compact" 
              onClick={() => handleExport("csv")}
              disabled={exportingFormat !== null}
            >
              <Download size={14} style={{ marginRight: 6 }} />
              {exportingFormat === "csv" ? "Exporting..." : "Export CSV"}
            </button>
          </div>
        }
      />

      {exportError && (
        <div style={{ 
          padding: "12px 16px", 
          background: "rgba(239, 68, 68, 0.1)", 
          border: "1px solid rgba(239, 68, 68, 0.2)", 
          borderRadius: "6px", 
          color: "var(--danger)",
          marginBottom: "20px",
          fontSize: "14px"
        }}>
          {exportError}
        </div>
      )}

      {/* Main Grid Layout */}
      <div className="grid three" style={{ gap: "24px", marginBottom: "24px" }}>
        
        {/* Left Column - Summary & Overview */}
        <div style={{ gridColumn: "span 1", display: "flex", flexDirection: "column", gap: "20px" }}>
          <Panel title="Metadata Overview">
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
              <div>
                <span className="muted" style={{ fontSize: "12px", textTransform: "uppercase" }}>Session ID</span>
                <div className="mono" style={{ fontSize: "14px", marginTop: "4px" }}>{session.id}</div>
              </div>
              
              <div>
                <span className="muted" style={{ fontSize: "12px", textTransform: "uppercase" }}>Status</span>
                <div style={{ display: "flex", alignItems: "center", gap: "6px", marginTop: "4px", fontWeight: 600 }}>
                  <span style={{ 
                    width: "8px", 
                    height: "8px", 
                    borderRadius: "50%", 
                    background: getStatusColor(session.status) 
                  }} />
                  <span style={{ color: getStatusColor(session.status) }}>{session.status}</span>
                </div>
              </div>

              <div>
                <span className="muted" style={{ fontSize: "12px", textTransform: "uppercase" }}>Started At</span>
                <div style={{ fontSize: "14px", marginTop: "4px" }}>{formatTime(session.started_at ?? undefined)}</div>
              </div>

              <div>
                <span className="muted" style={{ fontSize: "12px", textTransform: "uppercase" }}>Ended At</span>
                <div style={{ fontSize: "14px", marginTop: "4px" }}>{formatTime(session.ended_at ?? undefined)}</div>
              </div>

              <div>
                <span className="muted" style={{ fontSize: "12px", textTransform: "uppercase" }}>Report Version</span>
                <div className="mono" style={{ fontSize: "14px", marginTop: "4px" }}>v{reportVersion}</div>
              </div>
            </div>
          </Panel>

          <Panel title="Archive Policy Status">
            <div style={{ display: "flex", alignItems: "center", gap: "12px", padding: "4px 0" }}>
              <CheckCircle2 size={24} style={{ color: "var(--success)", flexShrink: 0 }} />
              <div>
                <div style={{ fontWeight: 600, fontSize: "13px", color: "var(--text)" }}>Raw Telemetry Deleted</div>
                <div className="muted" style={{ fontSize: "12px", marginTop: "2px" }}>All stream logs and Redis keys have been safely cleared.</div>
              </div>
            </div>
          </Panel>
        </div>

        {/* Right Column - Comprehensive Statistics */}
        <div style={{ gridColumn: "span 2", display: "flex", flexDirection: "column", gap: "24px" }}>
          
          <Panel title="Summary & Summary Metrics">
            <blockquote style={{ 
              margin: 0, 
              padding: "16px", 
              background: "rgba(255,255,255,0.02)", 
              borderLeft: "4px solid var(--accent)", 
              borderRadius: "4px",
              fontStyle: "italic",
              color: "var(--text)",
              fontSize: "14px",
              lineHeight: 1.5,
              marginBottom: "20px"
            }}>
              "{summary}"
            </blockquote>

            <div className="stat-grid three" style={{ gap: "16px" }}>
              <StatCard 
                label="Duration" 
                value={formatSeconds(stats.duration_seconds ?? 0)} 
              />
              <StatCard 
                label="Uptime percentage" 
                value={`${(stats.uptime_percentage ?? 100.0).toFixed(1)}%`} 
                tone={stats.uptime_percentage && stats.uptime_percentage < 98 ? "warning" : "success"}
              />
              <StatCard 
                label="Total Samples" 
                value={(stats.messages_processed ?? 0).toLocaleString()} 
              />
            </div>
          </Panel>

          {/* Environmental Vitals */}
          <Panel title="Vitals & Environmental Aggregates">
            <div className="grid two" style={{ gap: "20px" }}>
              
              {/* Battery Stats Card */}
              <div style={{ 
                padding: "16px", 
                background: "var(--surface-2)", 
                border: "1px solid var(--line)", 
                borderRadius: "8px" 
              }}>
                <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "16px" }}>
                  <Battery size={20} style={{ color: "var(--accent)" }} />
                  <h4 style={{ margin: 0, fontSize: "15px" }}>Battery Metrics</h4>
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Average Battery</span>
                    <span style={{ fontWeight: 600 }}>{stats.average_battery !== undefined ? `${stats.average_battery.toFixed(1)}%` : "-"}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Minimum Battery</span>
                    <span className="mono">{stats.minimum_battery !== undefined ? `${stats.minimum_battery.toFixed(1)}%` : "-"}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Maximum Battery</span>
                    <span className="mono">{stats.maximum_battery !== undefined ? `${stats.maximum_battery.toFixed(1)}%` : "-"}</span>
                  </div>
                </div>
              </div>

              {/* Temperature Stats Card */}
              <div style={{ 
                padding: "16px", 
                background: "var(--surface-2)", 
                border: "1px solid var(--line)", 
                borderRadius: "8px" 
              }}>
                <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "16px" }}>
                  <Thermometer size={20} style={{ color: "var(--accent)" }} />
                  <h4 style={{ margin: 0, fontSize: "15px" }}>Temperature Metrics</h4>
                </div>
                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Average Temp</span>
                    <span style={{ fontWeight: 600 }}>{stats.average_temperature !== undefined ? `${stats.average_temperature.toFixed(1)}°C` : "-"}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Minimum Temp</span>
                    <span className="mono">{stats.minimum_temperature !== undefined ? `${stats.minimum_temperature.toFixed(1)}°C` : "-"}</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between" }}>
                    <span className="muted">Maximum Temp</span>
                    <span className="mono">{stats.maximum_temperature !== undefined ? `${stats.maximum_temperature.toFixed(1)}°C` : "-"}</span>
                  </div>
                </div>
              </div>

            </div>

            {/* Other Metrics (Distance, Commands, Alerts, Anomalies) */}
            <div className="grid four" style={{ gap: "16px", marginTop: "20px" }}>
              <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.02)", borderRadius: "6px" }}>
                <Navigation size={18} style={{ color: "var(--accent)", marginBottom: "4px" }} />
                <div className="muted" style={{ fontSize: "11px" }}>Distance</div>
                <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>
                  {stats.distance_travelled !== undefined ? `${stats.distance_travelled.toFixed(3)} km` : "-"}
                </div>
              </div>

              <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.02)", borderRadius: "6px" }}>
                <AlertTriangle size={18} style={{ color: stats.alerts_count && stats.alerts_count > 0 ? "var(--warning)" : "var(--faint)", marginBottom: "4px" }} />
                <div className="muted" style={{ fontSize: "11px" }}>Alerts Count</div>
                <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.alerts_count ?? 0}</div>
              </div>

              <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.02)", borderRadius: "6px" }}>
                <XCircle size={18} style={{ color: stats.anomaly_count && stats.anomaly_count > 0 ? "var(--danger)" : "var(--faint)", marginBottom: "4px" }} />
                <div className="muted" style={{ fontSize: "11px" }}>AI Anomalies</div>
                <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.anomaly_count ?? 0}</div>
              </div>

              <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.02)", borderRadius: "6px" }}>
                <Terminal size={18} style={{ color: "var(--accent)", marginBottom: "4px" }} />
                <div className="muted" style={{ fontSize: "11px" }}>Commands Sent</div>
                <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.command_count ?? 0}</div>
              </div>
            </div>
          </Panel>

        </div>
      </div>

      {/* Device Details Breakdown Panel */}
      <div style={{ marginBottom: "24px" }}>
        <Panel title={`Participating Devices (${stats.device_participation_count ?? 0})`}>
          {Object.keys(devicesDetail).length === 0 ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No participating devices recorded for this session.</p>
          ) : (
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Device ID</th>
                    <th>First Seen</th>
                    <th>Last Seen</th>
                    <th>Uptime (Calculated)</th>
                    <th>Uptime (Reported Range)</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(devicesDetail).map(([devId, info]: [string, any]) => (
                    <tr key={devId}>
                      <td className="mono" style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                        <Cpu size={14} style={{ color: "var(--accent)" }} />
                        <span style={{ fontSize: "12px" }}>{devId}</span>
                      </td>
                      <td className="muted" style={{ fontSize: "12px" }}>
                        {info.first_seen_ts ? new Date(info.first_seen_ts * 1000).toLocaleTimeString() : "-"}
                      </td>
                      <td className="muted" style={{ fontSize: "12px" }}>
                        {info.last_seen_ts ? new Date(info.last_seen_ts * 1000).toLocaleTimeString() : "-"}
                      </td>
                      <td>
                        <span style={{ fontWeight: 500, fontSize: "13px" }}>
                          {info.calculated_uptime_s !== undefined ? formatSeconds(info.calculated_uptime_s) : "-"}
                        </span>
                      </td>
                      <td className="muted" style={{ fontSize: "12px" }}>
                        {info.min_reported_uptime !== undefined && info.max_reported_uptime !== undefined 
                          ? `${formatSeconds(Math.round(info.min_reported_uptime))} - ${formatSeconds(Math.round(info.max_reported_uptime))}`
                          : "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </>
  );
}
