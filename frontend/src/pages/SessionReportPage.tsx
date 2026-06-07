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
import { PageHeader, Panel, StatCard, LoadingRows, ErrorState, StatusChip } from "../components/ui";
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

  const getStatusTone = (status: string): "success" | "danger" | "neutral" => {
    switch (status) {
      case "COMPLETED": return "success";
      case "FAILED": return "danger";
      default: return "neutral";
    }
  };

  const tabs = [
    { id: "overview", label: "Overview", icon: <Activity size={15} strokeWidth={1.5} /> },
    { id: "devices", label: "Devices", icon: <Cpu size={15} strokeWidth={1.5} /> },
    { id: "ai", label: "AI Incidents", icon: <MessageSquare size={15} strokeWidth={1.5} /> },
    { id: "aggregates", label: "Metric Aggregates", icon: <TrendingUp size={15} strokeWidth={1.5} /> }
  ];

  return (
    <>
      <div className="report-back-link">
        <button
          onClick={() => navigate("/workspaces")}
          className="t-link"
          type="button"
        >
          <ArrowLeft size={14} strokeWidth={1.5} /> Back to Workspaces
        </button>
      </div>

      <PageHeader
        title={`Session Report v2`}
        description={`Comprehensive operational archive for session ${sessionID}`}
        actions={
          <div className="report-actions">
            <button
              className="button secondary compact"
              onClick={() => handleExport("json")}
              disabled={exportingFormat !== null}
            >
              <FileJson size={13} strokeWidth={1.5} />
              {exportingFormat === "json" ? "Exporting..." : "Export JSON"}
            </button>
            <button
              className="btn-inverse"
              onClick={() => handleExport("csv")}
              disabled={exportingFormat !== null}
            >
              <Download size={13} strokeWidth={1.5} />
              {exportingFormat === "csv" ? "Exporting..." : "Export CSV"}
            </button>
          </div>
        }
      />

      {exportError && (
        <div className="form-message error report-export-error">
          {exportError}
        </div>
      )}

      {/* Tabs Navigation */}
      <div className="report-tabs">
        {tabs.map((tab) => {
          const isActive = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as TabID)}
              className={`report-tab ${isActive ? "active" : ""}`}
              type="button"
            >
              {tab.icon}
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Render active tab content */}
      {activeTab === "overview" && (
        <div className="grid three report-overview">
          {/* Metadata Sidebar */}
          <div className="report-meta-col">
            <Panel title="Metadata Overview">
              <div className="report-meta-stack">
                <div>
                  <span className="report-meta-label">Session ID</span>
                  <div className="mono report-meta-value">{session.id}</div>
                </div>

                <div>
                  <span className="report-meta-label">Status</span>
                  <div className="report-status-row">
                    <span className="status-dot" />
                    <span><StatusChip value={session.status} /></span>
                  </div>
                </div>

                <div>
                  <span className="report-meta-label">Started At</span>
                  <div className="report-meta-value">{formatTime(session.started_at ?? undefined)}</div>
                </div>

                <div>
                  <span className="report-meta-label">Ended At</span>
                  <div className="report-meta-value">{formatTime(session.ended_at ?? undefined)}</div>
                </div>

                <div>
                  <span className="report-meta-label">Report Version</span>
                  <div className="mono report-meta-value">v{artifact.report_version || "2.0"}</div>
                </div>
              </div>
            </Panel>

            <Panel title="Archive Policy Status">
              <div className="report-archive-row">
                <CheckCircle2 size={22} strokeWidth={1.5} />
                <div>
                  <div className="report-archive-title">Telemetry Cleaned</div>
                  <div className="muted report-archive-desc">Raw logs & Redis keys have been safely cleared from storage.</div>
                </div>
              </div>
            </Panel>
          </div>

          {/* Main summary info */}
          <div className="report-summary-col">
            <Panel title="Session Summary">
              <blockquote className="report-blockquote">
                "{artifact.session_summary || "No summary text generated for this session."}"
              </blockquote>

              <div className="stat-grid three">
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
              <div className="grid two report-vitals-grid">

                {/* Battery Stats Card */}
                <div className="report-vital-card">
                  <div className="report-vital-header">
                    <Battery size={18} strokeWidth={1.5} />
                    <h4>Battery Metrics</h4>
                  </div>
                  <div className="report-vital-list">
                    <div className="report-vital-row">
                      <span className="muted">Average Battery</span>
                      <span>{stats.average_battery !== undefined && stats.average_battery > 0 ? `${stats.average_battery.toFixed(1)}%` : "-"}</span>
                    </div>
                    <div className="report-vital-row">
                      <span className="muted">Minimum Battery</span>
                      <span className="mono">{stats.minimum_battery !== undefined && stats.minimum_battery > 0 ? `${stats.minimum_battery.toFixed(1)}%` : "-"}</span>
                    </div>
                    <div className="report-vital-row">
                      <span className="muted">Maximum Battery</span>
                      <span className="mono">{stats.maximum_battery !== undefined && stats.maximum_battery > 0 ? `${stats.maximum_battery.toFixed(1)}%` : "-"}</span>
                    </div>
                  </div>
                </div>

                {/* Temperature Stats Card */}
                <div className="report-vital-card">
                  <div className="report-vital-header">
                    <Thermometer size={18} strokeWidth={1.5} />
                    <h4>Temperature Metrics</h4>
                  </div>
                  <div className="report-vital-list">
                    <div className="report-vital-row">
                      <span className="muted">Average Temp</span>
                      <span>{stats.average_temperature !== undefined && stats.average_temperature > 0 ? `${stats.average_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                    <div className="report-vital-row">
                      <span className="muted">Minimum Temp</span>
                      <span className="mono">{stats.minimum_temperature !== undefined && stats.minimum_temperature > 0 ? `${stats.minimum_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                    <div className="report-vital-row">
                      <span className="muted">Maximum Temp</span>
                      <span className="mono">{stats.maximum_temperature !== undefined && stats.maximum_temperature > 0 ? `${stats.maximum_temperature.toFixed(1)}°C` : "-"}</span>
                    </div>
                  </div>
                </div>

              </div>

              {/* Counter Metrics (Distance, Commands, Alerts, Anomalies) */}
              <div className="grid four report-counter-grid">
                <div className="report-counter-card">
                  <Navigation size={16} strokeWidth={1.5} />
                  <div className="muted report-counter-label">Distance</div>
                  <div className="mono report-counter-value">
                    {stats.distance_travelled !== undefined ? `${stats.distance_travelled.toFixed(3)} km` : "0 km"}
                  </div>
                </div>

                <div className="report-counter-card">
                  <AlertTriangle size={16} strokeWidth={1.5} />
                  <div className="muted report-counter-label">Alerts Count</div>
                  <div className="mono report-counter-value">{stats.alerts_count ?? 0}</div>
                </div>

                <div className="report-counter-card">
                  <XCircle size={16} strokeWidth={1.5} />
                  <div className="muted report-counter-label">AI Anomalies</div>
                  <div className="mono report-counter-value">{stats.anomaly_count ?? 0}</div>
                </div>

                <div className="report-counter-card">
                  <Terminal size={16} strokeWidth={1.5} />
                  <div className="muted report-counter-label">Commands Sent</div>
                  <div className="mono report-counter-value">{stats.command_count ?? 0}</div>
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
                        <span className={`inline-chip ${info.warning_incidents_count > 0 ? "tone-warning" : ""}`}>
                          {info.warning_incidents_count}
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        <span className={`inline-chip ${info.critical_incidents_count > 0 ? "tone-danger" : ""}`}>
                          {info.critical_incidents_count}
                        </span>
                      </td>
                      <td style={{ fontSize: "13px" }}>
                        <span className={info.active_at_end ? "incident-title" : "report-healthy"}>
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
            <p className="muted report-empty-msg">No AI incidents recorded for this session.</p>
          ) : (
            <div className="report-incidents-stack">
              {artifact.incidents_archive.map((inc, i) => (
                <div key={i} className="report-incident-card">
                  <div className="report-incident-header">
                    <div className="report-incident-meta">
                      <span className={`inline-chip ${inc.severity === "critical" ? "tone-danger" : "tone-warning"}`}>
                        {inc.severity}
                      </span>
                      <span className="mono report-incident-metric">{inc.metric} ({inc.incident_type})</span>
                      <span className="mono muted report-incident-device">Device: {inc.device_id}</span>
                    </div>
                    <span className={inc.resolved_at ? "report-healthy" : "report-warning"}>
                      {inc.resolved_at ? "RESOLVED" : "ACTIVE AT END"}
                    </span>
                  </div>

                  <p className="report-incident-summary">
                    {inc.summary}
                  </p>

                  <div className="grid four report-incident-grid">
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
            <p className="muted report-empty-msg">No metric aggregates recorded for this session.</p>
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
