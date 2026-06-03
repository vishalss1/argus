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
import { TelemetryRollup } from "../types/api";

type TabID = "overview" | "devices" | "timeline" | "alerts" | "commands" | "ai" | "rollups";

export function SessionReportPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabID>("overview");
  const [exportingFormat, setExportingFormat] = useState<"json" | "csv" | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);

  // Search/filter states
  const [timelineSearch, setTimelineSearch] = useState("");
  const [timelineFilter, setTimelineFilter] = useState("ALL");
  const [selectedDeviceRollup, setSelectedDeviceRollup] = useState<string>("");
  const [rollupMetric, setRollupMetric] = useState<"battery" | "temp" | "signal">("battery");

  const { data: session, isLoading: sessionLoading, error: sessionErr } = useSession(sessionID ?? "");
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");
  const { data: artifact, isLoading: artifactLoading, error: artifactErr } = useSessionArtifact(sessionID ?? "");

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
    { id: "timeline", label: "Timeline", icon: <Terminal size={15} /> },
    { id: "alerts", label: "Alerts", icon: <AlertTriangle size={15} /> },
    { id: "commands", label: "Commands", icon: <Navigation size={15} /> },
    { id: "ai", label: "AI Findings", icon: <MessageSquare size={15} /> },
    { id: "rollups", label: "Rollups", icon: <TrendingUp size={15} /> }
  ];

  // Set default selected device rollup once data arrives
  const deviceIDs = Object.keys(artifact.device_summaries);
  if (deviceIDs.length > 0 && !selectedDeviceRollup) {
    setSelectedDeviceRollup(deviceIDs[0]);
  }

  // Timeline Filter & Search logic
  const filteredTimeline = (artifact.timeline || []).filter(entry => {
    const matchesSearch = entry.message.toLowerCase().includes(timelineSearch.toLowerCase()) || 
                          entry.type.toLowerCase().includes(timelineSearch.toLowerCase()) ||
                          (entry.device_id && entry.device_id.toLowerCase().includes(timelineSearch.toLowerCase()));
    
    if (timelineFilter === "ALL") return matchesSearch;
    if (timelineFilter === "ANOMALIES") return matchesSearch && entry.type === "Anomaly Detected";
    if (timelineFilter === "ALERTS") return matchesSearch && (entry.type.startsWith("Alert"));
    if (timelineFilter === "COMMANDS") return matchesSearch && (entry.type.startsWith("Command"));
    return matchesSearch;
  });

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
                    <th>Uptime %</th>
                    <th>Samples</th>
                    <th>Battery (Avg/Min/Max)</th>
                    <th>Temp (Avg/Min/Max)</th>
                    <th>Signal (Avg/Min/Max)</th>
                    <th>Distance</th>
                    <th>Alerts</th>
                    <th>Anomalies</th>
                  </tr>
                </thead>
                <tbody>
                  {Object.entries(artifact.device_summaries).map(([devId, info]) => (
                    <tr key={devId}>
                      <td className="mono" style={{ fontSize: "12px", fontWeight: 600, color: "var(--text)" }}>{devId}</td>
                      <td className="muted" style={{ fontSize: "11px" }}>{formatTime(info.first_seen)}</td>
                      <td className="muted" style={{ fontSize: "11px" }}>{formatTime(info.last_seen)}</td>
                      <td>
                        <span style={{ 
                          fontWeight: 600, 
                          fontSize: "13px",
                          color: info.uptime_percentage > 98 ? "var(--success)" : "var(--warning)"
                        }}>
                          {info.uptime_percentage.toFixed(1)}%
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>{info.sample_count}</td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        {info.battery_average > 0 
                          ? `${info.battery_average.toFixed(0)}% / ${info.battery_min.toFixed(0)}% / ${info.battery_max.toFixed(0)}%` 
                          : "-"}
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        {info.temperature_average > 0 
                          ? `${info.temperature_average.toFixed(1)}°C / ${info.temperature_min.toFixed(1)}°C / ${info.temperature_max.toFixed(1)}°C`
                          : "-"}
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        {info.signal_average !== 0 
                          ? `${info.signal_average.toFixed(0)} / ${info.signal_min.toFixed(0)} / ${info.signal_max.toFixed(0)} dBm`
                          : "-"}
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>{info.distance_travelled.toFixed(3)} km</td>
                      <td style={{ fontSize: "13px" }}>
                        <span className="badge" style={{ 
                          background: (info.warning_count + info.critical_count) > 0 ? "rgba(239,68,68,0.15)" : "transparent",
                          color: (info.warning_count + info.critical_count) > 0 ? "var(--danger)" : "var(--muted)",
                          padding: "2px 6px",
                          borderRadius: "4px"
                        }}>
                          {info.warning_count + info.critical_count}
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>
                        <span style={{ color: info.anomalies_detected > 0 ? "var(--danger)" : "var(--muted)" }}>
                          {info.anomalies_detected}
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

      {activeTab === "timeline" && (
        <Panel title="Mission Log Timeline">
          <div style={{ display: "flex", gap: "16px", marginBottom: "20px", flexWrap: "wrap" }}>
            <input 
              type="text" 
              placeholder="Search timeline..."
              value={timelineSearch}
              onChange={(e) => setTimelineSearch(e.target.value)}
              className="mono"
              style={{
                flex: "1",
                minWidth: "200px",
                padding: "8px 12px",
                background: "rgba(255,255,255,0.02)",
                border: "1px solid var(--line)",
                borderRadius: "6px",
                color: "var(--text)",
                fontSize: "14px"
              }}
            />
            
            <select
              value={timelineFilter}
              onChange={(e) => setTimelineFilter(e.target.value)}
              style={{
                padding: "8px 12px",
                background: "var(--surface)",
                border: "1px solid var(--line)",
                borderRadius: "6px",
                color: "var(--text)",
                fontSize: "14px",
                cursor: "pointer"
              }}
            >
              <option value="ALL">All Events</option>
              <option value="ANOMALIES">Anomalies</option>
              <option value="ALERTS">Alerts</option>
              <option value="COMMANDS">Commands</option>
            </select>
          </div>

          {filteredTimeline.length === 0 ? (
            <div style={{ padding: "40px", textAlign: "center", color: "var(--muted)" }}>
              No matching timeline events found.
            </div>
          ) : (
            <div style={{ 
              display: "flex", 
              flexDirection: "column", 
              gap: "0", 
              position: "relative",
              paddingLeft: "24px",
              borderLeft: "2px solid var(--line)"
            }}>
              {filteredTimeline.map((entry, index) => {
                let badgeColor = "var(--accent)";
                let icon = <Info size={12} />;

                if (entry.type === "Anomaly Detected") {
                  badgeColor = "var(--danger)";
                  icon = <XCircle size={12} />;
                } else if (entry.type.startsWith("Alert Triggered")) {
                  badgeColor = "var(--warning)";
                  icon = <AlertCircle size={12} />;
                } else if (entry.type.startsWith("Alert Cleared")) {
                  badgeColor = "var(--success)";
                  icon = <CheckCircle2 size={12} />;
                } else if (entry.type.startsWith("Command Sent")) {
                  badgeColor = "var(--accent)";
                  icon = <Navigation size={12} />;
                } else if (entry.type.startsWith("Command Acknowledged")) {
                  badgeColor = "var(--success)";
                  icon = <CheckCircle2 size={12} />;
                }

                return (
                  <div key={index} style={{ 
                    position: "relative",
                    paddingBottom: "24px"
                  }}>
                    {/* Bullet marker */}
                    <div style={{
                      position: "absolute",
                      left: "-31px",
                      top: "2px",
                      width: "12px",
                      height: "12px",
                      borderRadius: "50%",
                      background: "var(--surface)",
                      border: `3px solid ${badgeColor}`,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center"
                    }} />

                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: "8px" }}>
                      <div>
                        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                          <span style={{ fontWeight: 600, fontSize: "14px", color: "var(--text)" }}>{entry.type}</span>
                          {entry.device_id && (
                            <span className="mono" style={{ fontSize: "11px", background: "rgba(255,255,255,0.03)", padding: "1px 6px", borderRadius: "4px" }}>
                              {entry.device_id}
                            </span>
                          )}
                        </div>
                        <p style={{ margin: "6px 0 0 0", color: "var(--text)", fontSize: "13px", lineHeight: 1.4 }}>
                          {entry.message}
                        </p>
                      </div>
                      <span className="mono muted" style={{ fontSize: "11px" }}>{formatTime(entry.timestamp)}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>
      )}

      {activeTab === "alerts" && (
        <Panel title={`Alerts History Archive (${artifact.alerts?.length ?? 0})`}>
          {(!artifact.alerts || artifact.alerts.length === 0) ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No alert rule violations triggered during this session.</p>
          ) : (
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Timestamp</th>
                    <th>Severity</th>
                    <th>Source Device</th>
                    <th>Alert Type</th>
                    <th>Alert Message</th>
                    <th>Resolution Status</th>
                  </tr>
                </thead>
                <tbody>
                  {artifact.alerts.map((a, i) => (
                    <tr key={i}>
                      <td className="mono muted" style={{ fontSize: "12px" }}>{formatTime(a.timestamp)}</td>
                      <td>
                        <span className="badge" style={{ 
                          background: a.severity === "critical" ? "rgba(239, 68, 68, 0.15)" : "rgba(245, 158, 11, 0.15)",
                          color: a.severity === "critical" ? "var(--danger)" : "var(--warning)",
                          padding: "3px 8px",
                          borderRadius: "4px",
                          fontSize: "11px",
                          fontWeight: 600,
                          textTransform: "uppercase"
                        }}>
                          {a.severity}
                        </span>
                      </td>
                      <td className="mono" style={{ fontSize: "12px" }}>{a.source_device}</td>
                      <td>{a.alert_type}</td>
                      <td style={{ fontSize: "13px" }}>{a.message}</td>
                      <td>
                        <span style={{ 
                          fontWeight: 500,
                          color: a.resolution_state === "Resolved" ? "var(--success)" : "var(--warning)"
                        }}>
                          {a.resolution_state}
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

      {activeTab === "commands" && (
        <Panel title={`Dispatched Commands Archive (${artifact.commands?.length ?? 0})`}>
          {(!artifact.commands || artifact.commands.length === 0) ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No commands dispatched during this session.</p>
          ) : (
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Timestamp</th>
                    <th>Target Device</th>
                    <th>Command</th>
                    <th>Dispatched Status</th>
                    <th>Acknowledgement Time</th>
                  </tr>
                </thead>
                <tbody>
                  {artifact.commands.map((c, i) => (
                    <tr key={i}>
                      <td className="mono muted" style={{ fontSize: "12px" }}>{formatTime(c.timestamp)}</td>
                      <td className="mono" style={{ fontSize: "12px" }}>{c.target_device}</td>
                      <td className="mono" style={{ fontSize: "12px", color: "var(--accent)" }}>{c.command}</td>
                      <td>
                        <span style={{ 
                          fontWeight: 600,
                          color: c.status === "acked" ? "var(--success)" : c.status === "nacked" ? "var(--danger)" : "var(--warning)"
                        }}>
                          {c.status.toUpperCase()}
                        </span>
                      </td>
                      <td className="mono muted" style={{ fontSize: "12px" }}>
                        {c.acknowledgement_time ? formatTime(c.acknowledgement_time) : "-"}
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
        <Panel title={`AI Analytics & Insights (${artifact.ai_findings?.length ?? 0})`}>
          {(!artifact.ai_findings || artifact.ai_findings.length === 0) ? (
            <p className="muted" style={{ fontSize: "14px", margin: 0 }}>No AI recommendations or insights recorded for this session.</p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
              {artifact.ai_findings.map((f, i) => (
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
                        background: f.severity === "high" ? "rgba(239, 68, 68, 0.15)" : "rgba(59, 130, 246, 0.15)",
                        color: f.severity === "high" ? "var(--danger)" : "var(--accent)",
                        padding: "3px 8px",
                        borderRadius: "4px",
                        fontWeight: 600,
                        fontSize: "11px"
                      }}>{f.finding_type}</span>
                      <span className="mono muted" style={{ fontSize: "12px" }}>Device: {f.device_id}</span>
                    </div>
                    <span className="mono muted" style={{ fontSize: "11px" }}>{formatTime(f.timestamp)}</span>
                  </div>

                  <p style={{ margin: 0, fontSize: "14px", lineHeight: 1.5, color: "var(--text)" }}>
                    {f.recommendation}
                  </p>

                  <div style={{ display: "flex", alignItems: "center", gap: "12px", marginTop: "4px" }}>
                    <span className="muted" style={{ fontSize: "12px" }}>Confidence Match Score:</span>
                    <div style={{ 
                      flex: "1", 
                      maxWidth: "200px", 
                      height: "6px", 
                      background: "rgba(255,255,255,0.05)", 
                      borderRadius: "3px", 
                      overflow: "hidden" 
                    }}>
                      <div style={{ 
                        width: `${(f.confidence_score * 100).toFixed(0)}%`, 
                        height: "100%", 
                        background: "var(--accent)" 
                      }} />
                    </div>
                    <span className="mono" style={{ fontSize: "12px", fontWeight: 600 }}>{(f.confidence_score * 100).toFixed(0)}%</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>
      )}

      {activeTab === "rollups" && (
        <div style={{ display: "flex", flexDirection: "column", gap: "24px" }}>
          <Panel title="Telemetry Rollup Selector">
            <div style={{ display: "flex", gap: "16px", flexWrap: "wrap", alignItems: "center" }}>
              <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
                <label className="muted" style={{ fontSize: "12px" }}>Select Device</label>
                <select
                  value={selectedDeviceRollup}
                  onChange={(e) => setSelectedDeviceRollup(e.target.value)}
                  style={{
                    padding: "8px 12px",
                    background: "var(--surface)",
                    border: "1px solid var(--line)",
                    borderRadius: "6px",
                    color: "var(--text)",
                    fontSize: "14px",
                    cursor: "pointer",
                    minWidth: "200px"
                  }}
                >
                  {deviceIDs.map((id) => (
                    <option key={id} value={id}>{id}</option>
                  ))}
                </select>
              </div>

              <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
                <label className="muted" style={{ fontSize: "12px" }}>Select Metric</label>
                <div style={{ display: "flex", gap: "4px", background: "rgba(255,255,255,0.02)", padding: "2px", borderRadius: "6px", border: "1px solid var(--line)" }}>
                  {(["battery", "temp", "signal"] as const).map((m) => (
                    <button
                      key={m}
                      onClick={() => setRollupMetric(m)}
                      style={{
                        padding: "6px 12px",
                        background: rollupMetric === m ? "var(--accent)" : "transparent",
                        border: "none",
                        borderRadius: "4px",
                        color: rollupMetric === m ? "#000" : "var(--text)",
                        cursor: "pointer",
                        fontWeight: 600,
                        fontSize: "12px",
                        textTransform: "capitalize",
                        transition: "all 0.15s ease"
                      }}
                    >
                      {m === "temp" ? "Temperature" : m}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </Panel>

          {selectedDeviceRollup && artifact.telemetry_rollups[selectedDeviceRollup] ? (
            <div style={{ display: "flex", flexDirection: "column", gap: "24px" }}>
              <Panel title={`${rollupMetric.toUpperCase()} Chart Trend`}>
                <TelemetryRollupChart 
                  rollups={artifact.telemetry_rollups[selectedDeviceRollup]} 
                  metric={rollupMetric} 
                />
              </Panel>

              <Panel title="Raw Rollup Minute Aggregates">
                <div className="table-wrap">
                  <table className="ai-table">
                    <thead>
                      <tr>
                        <th>Timestamp (Minute)</th>
                        <th>Battery (Avg/Min/Max)</th>
                        <th>Temp (Avg/Min/Max)</th>
                        <th>Signal (Avg/Min/Max)</th>
                        <th>Sample Count</th>
                      </tr>
                    </thead>
                    <tbody>
                      {artifact.telemetry_rollups[selectedDeviceRollup].map((r, i) => (
                        <tr key={i}>
                          <td className="mono muted" style={{ fontSize: "12px" }}>{formatTime(r.timestamp)}</td>
                          <td className="mono" style={{ fontSize: "12px" }}>
                            {r.battery_avg > 0 
                              ? `${r.battery_avg.toFixed(1)}% / ${r.battery_min.toFixed(0)}% / ${r.battery_max.toFixed(0)}%`
                              : "-"}
                          </td>
                          <td className="mono" style={{ fontSize: "12px" }}>
                            {r.temperature_avg > 0 
                              ? `${r.temperature_avg.toFixed(1)}°C / ${r.temperature_min.toFixed(1)}°C / ${r.temperature_max.toFixed(1)}°C`
                              : "-"}
                          </td>
                          <td className="mono" style={{ fontSize: "12px" }}>
                            {r.signal_avg !== 0 
                              ? `${r.signal_avg.toFixed(1)} / ${r.signal_min.toFixed(0)} / ${r.signal_max.toFixed(0)} dBm`
                              : "-"}
                          </td>
                          <td className="mono" style={{ fontSize: "12px" }}>{r.sample_count}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Panel>
            </div>
          ) : (
            <Panel title="Metric Trends">
              <div style={{ padding: "40px", textAlign: "center", color: "var(--muted)" }}>
                No telemetry rollups found for this device.
              </div>
            </Panel>
          )}
        </div>
      )}
    </>
  );
}

function TelemetryRollupChart({ rollups, metric }: { rollups: TelemetryRollup[]; metric: "battery" | "temp" | "signal" }) {
  if (rollups.length === 0) return <div className="muted" style={{ padding: "40px", textAlign: "center" }}>No data points available</div>;

  const width = 600;
  const height = 240;
  const padding = 45;

  const getValue = (r: TelemetryRollup) => {
    if (metric === "battery") return r.battery_avg;
    if (metric === "temp") return r.temperature_avg;
    return r.signal_avg; // signal
  };

  const values = rollups.map(getValue);
  const minVal = Math.min(...values);
  const maxVal = Math.max(...values);
  
  // Padding/margins for values
  const valRange = maxVal - minVal;
  const valMin = minVal - (valRange * 0.15 || 1);
  const valMax = maxVal + (valRange * 0.15 || 1);

  const getX = (index: number) => padding + (index / (rollups.length - 1 || 1)) * (width - 2 * padding);
  const getY = (val: number) => height - padding - ((val - valMin) / (valMax - valMin || 1)) * (height - 2 * padding);

  let pathD = "";
  let areaD = "";
  if (rollups.length > 0) {
    pathD = `M ${getX(0)} ${getY(values[0])}`;
    for (let i = 1; i < rollups.length; i++) {
      pathD += ` L ${getX(i)} ${getY(values[i])}`;
    }
    areaD = `${pathD} L ${getX(rollups.length - 1)} ${height - padding} L ${getX(0)} ${height - padding} Z`;
  }

  return (
    <div style={{ width: "100%", background: "rgba(0,0,0,0.15)", borderRadius: "8px", padding: "16px" }}>
      <svg viewBox={`0 0 ${width} ${height}`} style={{ width: "100%", height: "auto" }}>
        {/* Y Axis Grid Lines & Labels */}
        {[0, 0.25, 0.5, 0.75, 1].map((ratio) => {
          const v = valMin + ratio * (valMax - valMin);
          const y = getY(v);
          return (
            <g key={ratio}>
              <line x1={padding} y1={y} x2={width - padding} y2={y} stroke="var(--line)" strokeDasharray="4 4" />
              <text x={padding - 8} y={y + 4} fill="var(--muted)" fontSize="9" textAnchor="end">{v.toFixed(1)}</text>
            </g>
          );
        })}

        {/* X Axis labels */}
        {rollups.length > 1 && [0, Math.floor(rollups.length / 2), rollups.length - 1].map((idx) => {
          const r = rollups[idx];
          const x = getX(idx);
          const tStr = new Date(r.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
          return (
            <text key={idx} x={x} y={height - padding + 16} fill="var(--muted)" fontSize="9" textAnchor="middle">{tStr}</text>
          );
        })}

        {/* Area fill with gradient */}
        <defs>
          <linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.2" />
            <stop offset="100%" stopColor="var(--accent)" stopOpacity="0.0" />
          </linearGradient>
        </defs>
        {areaD && <path d={areaD} fill="url(#chartGrad)" />}

        {/* Line path */}
        {pathD && <path d={pathD} fill="none" stroke="var(--accent)" strokeWidth="2" />}

        {/* Data points */}
        {rollups.map((r, i) => (
          <circle key={i} cx={getX(i)} cy={getY(values[i])} r="3" fill="var(--accent)" />
        ))}
      </svg>
    </div>
  );
}
