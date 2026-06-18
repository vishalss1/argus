import { useMemo, useState, useCallback } from "react";
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
  FileSpreadsheet,
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
  AlertCircle,
  Disc,
  Layers,
  Table2
} from "lucide-react";
import { API_BASE_URL } from "../services/http";
import { 
  useSession, 
  useSessionStatistics, 
  useSessionArtifact 
} from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard, LoadingRows, ErrorState, StatusChip } from "../components/ui";
import { SessionTelemetryViewer } from "../components/SessionTelemetryViewer";
import { api } from "../services/api";
import type { MetricAggregate, SessionArtifact } from "../types/api";

type TabID = "overview" | "devices" | "ai" | "aggregates" | "hourly" | "raw";

type CategoryMetrics = {
  label: string;
  icon: React.ReactNode;
  keys: string[];
  unit: string;
  decimals: number;
};

const METRIC_CATEGORIES: CategoryMetrics[] = [
  { label: "Battery Metrics", icon: <Battery size={18} strokeWidth={1.5} />, keys: ["battery"], unit: "%", decimals: 1 },
  { label: "Temperature Metrics", icon: <Thermometer size={18} strokeWidth={1.5} />, keys: ["temp"], unit: "°C", decimals: 1 },
  { label: "CPU Metrics", icon: <Cpu size={18} strokeWidth={1.5} />, keys: ["cpu", "load"], unit: "%", decimals: 1 },
  { label: "RSSI Metrics", icon: <Wifi size={18} strokeWidth={1.5} />, keys: ["rssi", "signal", "wifi"], unit: "dBm", decimals: 1 },
];

function computeCategoryMetrics(
  metrics_aggregates: Record<string, Record<string, MetricAggregate>>,
  categoryKeys: string[]
): { avg: number; min: number; max: number; count: number } | null {
  let totalWeightedAvg = 0;
  let totalCount = 0;
  let globalMin = Infinity;
  let globalMax = -Infinity;

  for (const devMetrics of Object.values(metrics_aggregates)) {
    for (const [metricName, agg] of Object.entries(devMetrics)) {
      if (categoryKeys.some(key => metricName.toLowerCase().includes(key))) {
        totalWeightedAvg += agg.average * agg.count;
        totalCount += agg.count;
        if (agg.min < globalMin) globalMin = agg.min;
        if (agg.max > globalMax) globalMax = agg.max;
      }
    }
  }

  if (totalCount === 0) return null;
  return {
    avg: totalWeightedAvg / totalCount,
    min: globalMin,
    max: globalMax,
    count: totalCount,
  };
}

function computeFleetStats(artifact: SessionArtifact) {
  const deviceCount = Object.keys(artifact.device_summaries || {}).length;
  let totalSamples = 0;
  let totalWarning = 0;
  let totalCritical = 0;
  for (const ds of Object.values(artifact.device_summaries || {})) {
    totalSamples += ds.sample_count;
    totalWarning += ds.warning_incidents_count;
    totalCritical += ds.critical_incidents_count;
  }
  return { deviceCount, totalSamples, totalWarning, totalCritical };
}

export function SessionReportPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabID>("overview");
  const [exportingFormat, setExportingFormat] = useState<"artifact" | "telemetry-json" | "telemetry-csv" | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  const { data: session, isLoading: sessionLoading, error: sessionErr } = useSession(sessionID ?? "");
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");
  const { data: artifact, isLoading: artifactLoading, error: artifactErr } = useSessionArtifact(sessionID ?? "");

  const categoryData = useMemo(() => {
    if (!artifact?.metrics_aggregates) return null;
    const result: Record<string, { avg: number; min: number; max: number; count: number } | null> = {};
    for (const cat of METRIC_CATEGORIES) {
      result[cat.label] = computeCategoryMetrics(artifact.metrics_aggregates, cat.keys);
    }
    return result;
  }, [artifact]);

  const fleetStats = useMemo(() => {
    if (!artifact) return null;
    return computeFleetStats(artifact);
  }, [artifact]);

  const downloadBlob = useCallback(async (url: string, filename: string) => {
    try {
      const headers = new Headers();
      const accessToken = localStorage.getItem("argus_access_token");
      if (accessToken) {
        headers.set("Authorization", `Bearer ${accessToken}`);
      }
      const workspaceID = localStorage.getItem("argus_active_workspace_id");
      if (workspaceID) {
        headers.set("X-Workspace-ID", workspaceID);
      }
      const resp = await fetch(`${API_BASE_URL}${url}`, { headers });
      if (!resp.ok) {
        const body = await resp.text();
        let msg = `HTTP ${resp.status}`;
        try { const j = JSON.parse(body); msg = j.error || msg; } catch {}
        throw new Error(msg);
      }
      const blob = await resp.blob();
      const objUrl = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = objUrl;
      link.setAttribute("download", filename);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(objUrl);
    } catch (err: any) {
      throw err;
    }
  }, []);

  const handleExport = async (type: "artifact" | "telemetry-json" | "telemetry-csv") => {
    if (!sessionID) return;
    setExportingFormat(type);
    setExportError(null);
    try {
      if (type === "artifact") {
        const data = await api.sessions.getArtifact(sessionID);
        const dataStr = "data:text/json;charset=utf-8," + encodeURIComponent(JSON.stringify(data, null, 2));
        const link = document.createElement("a");
        link.href = dataStr;
        link.setAttribute("download", `session_artifact_${sessionID}.json`);
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
      } else if (type === "telemetry-json") {
        await downloadBlob(`/sessions/${sessionID}/telemetry/json`, `session_${sessionID}_telemetry.json`);
      } else if (type === "telemetry-csv") {
        await downloadBlob(`/sessions/${sessionID}/telemetry/csv`, `session_${sessionID}_telemetry.csv`);
      }
    } catch (err: any) {
      console.error(err);
      setExportError(`Export failed: ${err.message || err}`);
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
      <div style={{ padding: "24px", display: "flex", flexDirection: "column", gap: 12 }}>
        {Array.from({ length: 8 }).map((_, i) => (
          <div key={i} style={{ height: 20, background: "var(--surface-2)", borderRadius: 0 }} />
        ))}
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
    { id: "aggregates", label: "Metric Aggregates", icon: <TrendingUp size={15} strokeWidth={1.5} /> },
    { id: "hourly", label: "Hourly Summaries", icon: <Clock size={15} strokeWidth={1.5} /> },
    { id: "raw", label: "Raw Telemetry", icon: <Table2 size={15} strokeWidth={1.5} /> }
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
              onClick={() => handleExport("artifact")}
              disabled={exportingFormat !== null}
              title="Download the full session artifact (JSON)"
            >
              <FileJson size={13} strokeWidth={1.5} />
              {exportingFormat === "artifact" ? "Exporting..." : "Artifact JSON"}
            </button>
            {artifact.telemetry_export_paths && !artifact.exports_expired && (
              <>
                <button
                  className="button secondary compact"
                  onClick={() => handleExport("telemetry-json")}
                  disabled={exportingFormat !== null}
                  title="Download raw telemetry JSON archive"
                >
                  <Download size={13} strokeWidth={1.5} />
                  {exportingFormat === "telemetry-json" ? "Exporting..." : "Telemetry JSON"}
                </button>
                <button
                  className="btn-inverse"
                  onClick={() => handleExport("telemetry-csv")}
                  disabled={exportingFormat !== null}
                  title="Download raw telemetry CSV archive"
                >
                  <FileSpreadsheet size={13} strokeWidth={1.5} />
                  {exportingFormat === "telemetry-csv" ? "Exporting..." : "Telemetry CSV"}
                </button>
              </>
            )}
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

            <Panel title="Telemetry Export Status">
              <div className="report-archive-row">
                {artifact.exports_expired ? (
                  <XCircle size={22} strokeWidth={1.5} color="var(--warning)" />
                ) : artifact.telemetry_export_paths ? (
                  <CheckCircle2 size={22} strokeWidth={1.5} />
                ) : (
                  <Info size={22} strokeWidth={1.5} />
                )}
                <div>
                  {artifact.exports_expired ? (
                    <>
                      <div className="report-archive-title">Exports Expired</div>
                      <div className="muted report-archive-desc">Telemetry exports have expired. Statistical summaries remain permanently available.</div>
                    </>
                  ) : artifact.telemetry_export_paths ? (
                    <>
                      <div className="report-archive-title">Export Available</div>
                      <div className="muted report-archive-desc">Full telemetry available for download. Exports expire after 7 days.</div>
                    </>
                  ) : (
                    <>
                      <div className="report-archive-title">No Exports</div>
                      <div className="muted report-archive-desc">Raw telemetry was not exported for this session.</div>
                    </>
                  )}
                </div>
              </div>
            </Panel>

            {fleetStats && (
              <Panel title="Fleet Statistics">
                <div className="report-fleet-stack">
                  <div className="report-fleet-row">
                    <Cpu size={15} strokeWidth={1.5} />
                    <span className="muted">Devices</span>
                    <span className="mono">{fleetStats.deviceCount}</span>
                  </div>
                  <div className="report-fleet-row">
                    <Disc size={15} strokeWidth={1.5} />
                    <span className="muted">Total Samples</span>
                    <span className="mono">{fleetStats.totalSamples.toLocaleString()}</span>
                  </div>
                  <div className="report-fleet-row">
                    <Activity size={15} strokeWidth={1.5} />
                    <span className="muted">Warning Incidents</span>
                    <span className="mono">{fleetStats.totalWarning}</span>
                  </div>
                  <div className="report-fleet-row">
                    <AlertCircle size={15} strokeWidth={1.5} />
                    <span className="muted">Critical Incidents</span>
                    <span className="mono">{fleetStats.totalCritical}</span>
                  </div>
                  <div className="report-fleet-row">
                    <Terminal size={15} strokeWidth={1.5} />
                    <span className="muted">Commands Sent</span>
                    <span className="mono">{stats?.command_count ?? 0}</span>
                  </div>
                </div>
              </Panel>
            )}
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
                  tone={(stats.uptime_percentage ?? 100) < 98 ? "warning" : "success"}
                />
                <StatCard
                  label="Total Samples"
                  value={fleetStats ? fleetStats.totalSamples.toLocaleString() : (stats.messages_processed ?? 0).toLocaleString()}
                />
              </div>
            </Panel>

            {/* Aggregate Metric Cards from metrics_aggregates */}
            <Panel title="Fleet-Wide Metric Aggregates">
              <div className="grid two report-vitals-grid">
                {METRIC_CATEGORIES.map((cat) => {
                  const data = categoryData?.[cat.label];
                  return (
                    <div key={cat.label} className="report-vital-card">
                      <div className="report-vital-header">
                        {cat.icon}
                        <h4>{cat.label}</h4>
                      </div>
                      <div className="report-vital-list">
                        {data ? (
                          <>
                            <div className="report-vital-row">
                              <span className="muted">Average {cat.label.split(" ")[0]}</span>
                              <span>{data.avg.toFixed(cat.decimals)}{cat.unit}</span>
                            </div>
                            <div className="report-vital-row">
                              <span className="muted">Minimum {cat.label.split(" ")[0]}</span>
                              <span className="mono">{data.min.toFixed(cat.decimals)}{cat.unit}</span>
                            </div>
                            <div className="report-vital-row">
                              <span className="muted">Maximum {cat.label.split(" ")[0]}</span>
                              <span className="mono">{data.max.toFixed(cat.decimals)}{cat.unit}</span>
                            </div>
                          </>
                        ) : (
                          <div className="muted report-vital-row">No data</div>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>

              {/* Counter Metrics (Alerts, Anomalies, Commands) */}
              <div className="grid three report-counter-grid">
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

      {activeTab === "hourly" && (
        <Panel title={`Hourly Summaries${artifact.hourly_summaries ? ` (${Object.keys(artifact.hourly_summaries).length} devices)` : ""}`}>
          {!artifact.hourly_summaries || Object.keys(artifact.hourly_summaries).length === 0 ? (
            <div className="report-empty-msg" style={{ padding: 24, textAlign: "center" }}>
              <p className="muted" style={{ marginBottom: 8 }}>No completed hourly windows.</p>
              <p className="muted" style={{ fontSize: 13 }}>
                Current session duration is less than one hour.
              </p>
            </div>
          ) : (
            <div className="hourly-cards-grid">
              {Object.entries(artifact.hourly_summaries).flatMap(([devId, summaries]) =>
                [...new Set(summaries.map((s) => s.hour))].map((hour) => {
                  const hourSums = summaries.filter((s) => s.hour === hour);
                  const totalSamples = hourSums.reduce((a, s) => a + s.sample_count, 0);
                  const avgTemp = hourSums.find((s) => s.metric.toLowerCase().includes("temp"));
                  const avgBattery = hourSums.find((s) => s.metric.toLowerCase().includes("battery"));
                  const startLabel = hour.replace("T", " ").substring(0, 16);
                  const endDate = new Date(hour);
                  endDate.setHours(endDate.getHours() + 1);
                  const endLabel = endDate.toISOString().substring(11, 16);
                  return (
                    <div key={`${devId}-${hour}`} className="hourly-card">
                      <div className="hourly-card-header">
                        <Clock size={14} strokeWidth={1.5} />
                        <span className="mono">{startLabel}–{endLabel}</span>
                        <span className="muted" style={{ fontSize: 11 }}>{devId}</span>
                      </div>
                      <div className="hourly-card-body">
                        <div className="hourly-card-stat">
                          <span className="muted">Samples</span>
                          <span className="mono">{totalSamples.toLocaleString()}</span>
                        </div>
                        {avgTemp && (
                          <div className="hourly-card-stat">
                            <span className="muted">Avg Temp</span>
                            <span className="mono">{avgTemp.average.toFixed(1)}°C</span>
                          </div>
                        )}
                        {avgBattery && (
                          <div className="hourly-card-stat">
                            <span className="muted">Avg Battery</span>
                            <span className="mono">{avgBattery.average.toFixed(1)}%</span>
                          </div>
                        )}
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          )}
        </Panel>
      )}

      {activeTab === "raw" && (
        <Panel title="Raw Telemetry">
          <SessionTelemetryViewer
            sessionID={sessionID}
            artifactExportsExpired={artifact.exports_expired}
            artifactTelemetryExportPaths={artifact.telemetry_export_paths ?? null}
            isActive={session.status === "RUNNING"}
          />
        </Panel>
      )}
    </>
  );
}
