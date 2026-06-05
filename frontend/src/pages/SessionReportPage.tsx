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
  XCircle,
  Activity,
  MessageSquare,
  TrendingUp,
  Clock,
  Wifi,
  Info,
  Calendar,
  AlertCircle
} from "lucide-react";
import { 
  useSession, 
  useSessionStatistics, 
  useSessionArtifact 
} from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard, LoadingRows, ErrorState } from "../components/ui";
import { api } from "../services/api";

type TabID = "overview" | "devices" | "ai" | "aggregates";

export function SessionReportPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabID>("overview");
  const [exportingFormat, setExportingFormat] = useState<"json" | "csv" | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data: session, isLoading: sessionLoading, error: sessionErr } = useSession(sessionID ?? "");
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");
  const { data: artifact, isLoading: artifactLoading, error: artifactErr } = useSessionArtifact(sessionID ?? "");

  const handleExport = async (format: "json" | "csv") => {
    if (!sessionID || !artifact) return;
    setExportingFormat(format);
    setExportError(null);
    try {
      if (format === "json") {
        const dataStr = "data:text/json;charset=utf-8," + encodeURIComponent(JSON.stringify(artifact, null, 2));
        const link = document.createElement("a");
        link.href = dataStr;
        link.setAttribute("download", `session_report_${sessionID}.json`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
      } else if (format === "csv") {
        let csvContent = "";
        
        // 1. Device Summaries
        csvContent += "=== DEVICE SUMMARIES ===\n";
        csvContent += "Device ID,First Seen,Last Seen,Sample Count,Warning Incidents Count,Critical Incidents Count,Active At End\n";
        Object.values(artifact.device_summaries || {}).forEach((summary: any) => {
          csvContent += `${summary.device_id},${summary.first_seen},${summary.last_seen},${summary.sample_count},${summary.warning_incidents_count},${summary.critical_incidents_count},${summary.active_at_end}\n`;
        });
        csvContent += "\n";

        // 2. Incidents Archive
        csvContent += "=== INCIDENTS ARCHIVE ===\n";
        csvContent += "Device ID,Metric,Incident Type,Severity,Start Time,Resolved At,Occurrences,Peak Score,Summary\n";
        artifact.incidents_archive?.forEach((inc: any) => {
          const resolvedAt = inc.resolved_at ? inc.resolved_at : "Still Open";
          csvContent += `${inc.device_id},${inc.metric},${inc.incident_type},${inc.severity},${inc.start_time},${resolvedAt},${inc.occurrences},${inc.peak_score},"${inc.summary?.replace(/"/g, '""') || ""}"\n`;
        });
        csvContent += "\n";

        // 3. Metrics Aggregates
        csvContent += "=== METRICS AGGREGATES ===\n";
        csvContent += "Device ID,Metric,Count,Min,Max,Average,Variance\n";
        if (artifact.metrics_aggregates) {
          Object.entries(artifact.metrics_aggregates).forEach(([devID, metrics]: [string, any]) => {
            Object.entries(metrics).forEach(([mName, agg]: [string, any]) => {
              csvContent += `${devID},${mName},${agg.count},${agg.min},${agg.max},${agg.average},${agg.variance}\n`;
            });
          });
        }

        const dataStr = "data:text/csv;charset=utf-8," + encodeURIComponent(csvContent);
        const link = document.createElement("a");
        link.href = dataStr;
        link.setAttribute("download", `session_report_${sessionID}.csv`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
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

  const isLoading = sessionLoading || statsLoading || artifactLoading;
  const hasError = sessionErr || statsErr || artifactErr;

  if (isLoading) {
    return (
      <div style={{ padding: "24px" }}>
        <LoadingRows rows={10} />
      </div>
    );
  }

  if (hasError || !session || !stats || !artifact) {
    return (
      <ErrorState 
        message="Failed to load session report data. Ensure the session was completed or failed successfully." 
        onRetry={() => navigate("/workspaces")} 
      />
    );
  }

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

  const tabs = [
    { id: "overview", label: "Overview", icon: <Activity size={15} /> },
    { id: "devices", label: "Devices", icon: <Cpu size={15} /> },
    { id: "ai", label: "AI Incidents", icon: <MessageSquare size={15} /> },
    { id: "aggregates", label: "Metric Aggregates", icon: <TrendingUp size={15} /> }
  ];

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
        title={`Session Report v2`}
        description={`Comprehensive operational archive for session ${sessionID}`}
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

      {/* Tabs Navigation */}
      <div style={{
        display: "flex",
        gap: "4px",
        borderBottom: "1px solid var(--line)",
        marginBottom: "24px",
        overflowX: "auto",
        paddingBottom: "1px"
      }}>
        {tabs.map((tab) => {
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as TabID)}
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: "8px",
                padding: "12px 18px",
                background: isActive ? "rgba(255, 255, 255, 0.03)" : "transparent",
                border: "none",
                borderBottom: isActive ? "2px solid var(--accent)" : "2px solid transparent",
                color: isActive ? "var(--accent)" : "var(--muted)",
                cursor: "pointer",
                fontWeight: isActive ? 600 : 500,
                fontSize: "14px",
                whiteSpace: "nowrap",
                transition: "all 0.2s ease"
              }}
            >
              {tab.icon}
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Render active tab content */}
      {activeTab === "overview" && (
        <div className="grid three" style={{ gap: "24px" }}>
          {/* Metadata Sidebar */}
          <div style={{ gridColumn: "span 1", display: "flex", flexDirection: "column", gap: "20px" }}>
            <Panel title="Metadata Overview">
              <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
                <div>
                  <span className="muted" style={{ fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.5px" }}>Session ID</span>
                  <div className="mono" style={{ fontSize: "13px", marginTop: "4px" }}>{session.id}</div>
                </div>
                
                <div>
                  <span className="muted" style={{ fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.5px" }}>Status</span>
                  <div style={{ display: "flex", alignItems: "center", gap: "6px", marginTop: "4px", fontWeight: 600 }}>
                    <span style={{ 
                      width: "8px", 
                      height: "8px", 
                      borderRadius: "50%", 
                      background: getStatusColor(session.status) 
                    }} />
                    <span style={{ color: getStatusColor(session.status), fontSize: "14px" }}>{session.status}</span>
                  </div>
                </div>

                <div>
                  <span className="muted" style={{ fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.5px" }}>Started At</span>
                  <div style={{ fontSize: "13px", marginTop: "4px" }}>{formatTime(session.started_at ?? undefined)}</div>
                </div>

                <div>
                  <span className="muted" style={{ fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.5px" }}>Ended At</span>
                  <div style={{ fontSize: "13px", marginTop: "4px" }}>{formatTime(session.ended_at ?? undefined)}</div>
                </div>

                <div>
                  <span className="muted" style={{ fontSize: "11px", textTransform: "uppercase", letterSpacing: "0.5px" }}>Report Version</span>
                  <div className="mono" style={{ fontSize: "13px", marginTop: "4px" }}>v{artifact.report_version || "2.0"}</div>
                </div>
              </div>
            </Panel>

            <Panel title="Archive Policy Status">
              <div style={{ display: "flex", alignItems: "center", gap: "12px", padding: "4px 0" }}>
                <CheckCircle2 size={24} style={{ color: "var(--success)", flexShrink: 0 }} />
                <div>
                  <div style={{ fontWeight: 600, fontSize: "13px", color: "var(--text)" }}>Telemetry Cleaned</div>
                  <div className="muted" style={{ fontSize: "12px", marginTop: "2px" }}>Raw logs & Redis keys have been safely cleared from storage.</div>
                </div>
              </div>
            </Panel>
          </div>

          {/* Main summary info */}
          <div style={{ gridColumn: "span 2", display: "flex", flexDirection: "column", gap: "24px" }}>
            <Panel title="Session Summary">
              <blockquote style={{ 
                margin: 0, 
                padding: "16px", 
                background: "rgba(255,255,255,0.01)", 
                borderLeft: "4px solid var(--accent)", 
                borderRadius: "4px",
                fontStyle: "italic",
                color: "var(--text)",
                fontSize: "14px",
                lineHeight: 1.5,
                marginBottom: "20px"
              }}>
                "{artifact.session_summary || "No summary text generated for this session."}"
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
                    <h4 style={{ margin: 0, fontSize: "14px" }}>Battery Metrics</h4>
                  </div>
                  <div style={{ display: "flex", flexDirection: "column", gap: "12px", fontSize: "13px" }}>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Average Battery</span>
                      <span style={{ fontWeight: 600 }}>{stats.average_battery !== undefined && stats.average_battery > 0 ? `${stats.average_battery.toFixed(1)}%` : "-"}</span>
                    </div>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Minimum Battery</span>
                      <span className="mono">{stats.minimum_battery !== undefined && stats.minimum_battery > 0 ? `${stats.minimum_battery.toFixed(1)}%` : "-"}</span>
                    </div>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Maximum Battery</span>
                      <span className="mono">{stats.maximum_battery !== undefined && stats.maximum_battery > 0 ? `${stats.maximum_battery.toFixed(1)}%` : "-"}</span>
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
                    <h4 style={{ margin: 0, fontSize: "14px" }}>Temperature Metrics</h4>
                  </div>
                  <div style={{ display: "flex", flexDirection: "column", gap: "12px", fontSize: "13px" }}>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Average Temp</span>
                      <span style={{ fontWeight: 600 }}>{stats.average_temperature !== undefined && stats.average_temperature > 0 ? `${stats.average_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Minimum Temp</span>
                      <span className="mono">{stats.minimum_temperature !== undefined && stats.minimum_temperature > 0 ? `${stats.minimum_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                    <div style={{ display: "flex", justifyContent: "space-between" }}>
                      <span className="muted">Maximum Temp</span>
                      <span className="mono">{stats.maximum_temperature !== undefined && stats.maximum_temperature > 0 ? `${stats.maximum_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                  </div>
                </div>

              </div>

              {/* Counter Metrics (Distance, Commands, Alerts, Anomalies) */}
              <div className="grid four" style={{ gap: "16px", marginTop: "20px" }}>
                <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.015)", borderRadius: "6px" }}>
                  <Navigation size={18} style={{ color: "var(--accent)", marginBottom: "4px" }} />
                  <div className="muted" style={{ fontSize: "11px" }}>Distance</div>
                  <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>
                    {stats.distance_travelled !== undefined ? `${stats.distance_travelled.toFixed(3)} km` : "0 km"}
                  </div>
                </div>

                <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.015)", borderRadius: "6px" }}>
                  <AlertTriangle size={18} style={{ color: stats.alerts_count && stats.alerts_count > 0 ? "var(--warning)" : "var(--faint)", marginBottom: "4px" }} />
                  <div className="muted" style={{ fontSize: "11px" }}>Alerts Count</div>
                  <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.alerts_count ?? 0}</div>
                </div>

                <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.015)", borderRadius: "6px" }}>
                  <XCircle size={18} style={{ color: stats.anomaly_count && stats.anomaly_count > 0 ? "var(--danger)" : "var(--faint)", marginBottom: "4px" }} />
                  <div className="muted" style={{ fontSize: "11px" }}>AI Anomalies</div>
                  <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.anomaly_count ?? 0}</div>
                </div>

                <div style={{ textAlign: "center", padding: "12px", background: "rgba(255,255,255,0.015)", borderRadius: "6px" }}>
                  <Terminal size={18} style={{ color: "var(--accent)", marginBottom: "4px" }} />
                  <div className="muted" style={{ fontSize: "11px" }}>Commands Sent</div>
                  <div className="mono" style={{ fontSize: "14px", fontWeight: 600, marginTop: "4px" }}>{stats.command_count ?? 0}</div>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      )}

      {activeTab === "devices" && (
        <Panel title={`Participating Devices (${Object.keys(artifact.device_summaries || {}).length})`}>
          {Object.keys(artifact.device_summaries || {}).length === 0 ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No participating devices recorded for this session.</p>
          ) : (
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Device ID</th>
                    <th>First Seen</th>
                    <th>Last Seen</th>
                    <th>Samples</th>
                    <th>Warning Incidents</th>
                    <th>Critical Incidents</th>
                    <th>Active At End</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(artifact.device_summaries).map(([devId, info]) => (
                    <tr key={devId}>
                      <td className="mono" style={{ fontSize: "12px", fontWeight: 600, color: "var(--text)" }}>{devId}</td>
                      <td className="muted" style={{ fontSize: "11px" }}>{formatTime(info.first_seen)}</td>
                      <td className="muted" style={{ fontSize: "11px" }}>{formatTime(info.last_seen)}</td>
                      <td className="mono" style={{ fontSize: "12px" }}>{info.sample_count}</td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        <span className="badge" style={{ 
                          background: info.warning_incidents_count > 0 ? "rgba(245,158,11,0.15)" : "transparent",
                          color: info.warning_incidents_count > 0 ? "var(--warning)" : "var(--muted)",
                          padding: "2px 6px",
                          borderRadius: "4px"
                        }}>
                          {info.warning_incidents_count}
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        <span className="badge" style={{ 
                          background: info.critical_incidents_count > 0 ? "rgba(239,68,68,0.15)" : "transparent",
                          color: info.critical_incidents_count > 0 ? "var(--danger)" : "var(--muted)",
                          padding: "2px 6px",
                          borderRadius: "4px"
                        }}>
                          {info.critical_incidents_count}
                        </span>
                      </td>
                      <td style={{ fontSize: "13px" }}>
                        <span style={{ 
                          fontWeight: 600,
                          color: info.active_at_end ? "var(--danger)" : "var(--success)"
                        }}>
                          {info.active_at_end ? "Critical/Warning Active" : "Healthy"}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      )}

      {activeTab === "ai" && (
        <Panel title={`AI Incidents Archive (${artifact.incidents_archive?.length ?? 0})`}>
          {(!artifact.incidents_archive || artifact.incidents_archive.length === 0) ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No AI incidents recorded for this session.</p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
              {artifact.incidents_archive.map((inc, i) => (
                <div key={i} style={{ 
                  padding: "16px",
                  background: "var(--surface-2)",
                  border: "1px solid var(--line)",
                  borderRadius: "8px",
                  display: "flex",
                  flexDirection: "column",
                  gap: "12px"
                }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "8px" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
                      <span className="badge" style={{
                        background: inc.severity === "critical" ? "rgba(239, 68, 68, 0.15)" : "rgba(245, 158, 11, 0.15)",
                        color: inc.severity === "critical" ? "var(--danger)" : "var(--warning)",
                        padding: "3px 8px",
                        borderRadius: "4px",
                        fontWeight: 600,
                        fontSize: "11px",
                        textTransform: "uppercase"
                      }}>{inc.severity}</span>
                      <span className="mono" style={{ fontSize: "12px", fontWeight: 600 }}>{inc.metric} ({inc.incident_type})</span>
                      <span className="mono muted" style={{ fontSize: "12px" }}>Device: {inc.device_id}</span>
                    </div>
                    <span style={{ 
                      fontWeight: 600,
                      fontSize: "12px",
                      color: inc.resolved_at ? "var(--success)" : "var(--warning)"
                    }}>
                      {inc.resolved_at ? "RESOLVED" : "ACTIVE AT END"}
                    </span>
                  </div>

                  <p style={{ margin: 0, fontSize: "14px", lineHeight: 1.5, color: "var(--text)" }}>
                    {inc.summary}
                  </p>

                  <div className="grid four" style={{ gap: "12px", fontSize: "12px", marginTop: "4px" }}>
                    <div>
                      <span className="muted">Started:</span> <span className="mono">{formatTime(inc.start_time)}</span>
                    </div>
                    <div>
                      <span className="muted">Resolved:</span> <span className="mono">{inc.resolved_at ? formatTime(inc.resolved_at) : "-"}</span>
                    </div>
                    <div>
                      <span className="muted">Occurrences:</span> <span className="mono">{inc.occurrences}</span>
                    </div>
                    <div>
                      <span className="muted">Peak Anomaly Score:</span> <span className="mono">{inc.peak_score.toFixed(4)}</span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      )}

      {activeTab === "aggregates" && (
        <Panel title="Monitored Metric Aggregates">
          {Object.keys(artifact.metrics_aggregates || {}).length === 0 ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No metric aggregates recorded for this session.</p>
          ) : (
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Device ID</th>
                    <th>Metric</th>
                    <th>Count</th>
                    <th>Min</th>
                    <th>Max</th>
                    <th>Average</th>
                    <th>Variance</th>
                    <th>Std Dev (σ)</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(artifact.metrics_aggregates).flatMap(([devId, metrics]) =>
                    Object.entries(metrics).map(([metricName, agg]) => (
                      <tr key={`${devId}-${metricName}`}>
                        <td className="mono" style={{ fontSize: "12px", fontWeight: 600, color: "var(--text)" }}>{devId}</td>
                        <td className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{metricName}</td>
                        <td className="mono" style={{ fontSize: "12px" }}>{agg.count}</td>
                        <td className="mono" style={{ fontSize: "12px" }}>{agg.min.toFixed(4)}</td>
                        <td className="mono" style={{ fontSize: "12px" }}>{agg.max.toFixed(4)}</td>
                        <td className="mono" style={{ fontSize: "12px", fontWeight: 600 }}>{agg.average.toFixed(4)}</td>
                        <td className="mono" style={{ fontSize: "12px" }}>{agg.variance.toFixed(4)}</td>
                        <td className="mono" style={{ fontSize: "12px" }}>{Math.sqrt(agg.variance).toFixed(4)}</td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      )}
    </>
  );
}
