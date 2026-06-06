import React, { useEffect, useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { api } from "../services/api";
import { SemanticEvent, Incident, ReasoningResponse } from "../types/api";
import { Panel, PageHeader, StatusChip, CopyableID, EmptyState, StatCard, Pagination } from "../components/ui";
import { Brain, Search, Clock, AlertCircle, CheckCircle, ArrowRight, ShieldCheck, Zap, Activity, Heart, AlertTriangle, Play } from "lucide-react";
import { useDeviceStatus, useSessionActiveIncidents, useDevices, useSessions } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";

function SessionRequiredPrompt({ title, description }: { title: string; description: string }) {
  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "60vh", padding: "40px 20px" }}>
      <div style={{ maxWidth: 500, width: "100%", textAlign: "center" }}>
        <div style={{
          width: 80, height: 80, borderRadius: "20px", margin: "0 auto 28px",
          background: "linear-gradient(135deg, rgba(239,68,68,0.15), rgba(245,158,11,0.1))",
          border: "1px solid rgba(239,68,68,0.3)",
          display: "flex", alignItems: "center", justifyContent: "center"
        }}>
          <Play size={36} style={{ color: "var(--danger)" }} />
        </div>

        <h2 style={{ fontSize: "24px", fontWeight: 700, margin: "0 0 12px", letterSpacing: "-0.5px" }}>
          {title}
        </h2>
        <p className="muted" style={{ fontSize: "15px", lineHeight: 1.6, margin: "0 0 28px" }}>
          {description}
        </p>

        <Link
          to="/workspaces"
          className="button primary"
          style={{ padding: "12px 24px", fontSize: "15px", fontWeight: 600, display: "inline-flex", alignItems: "center", gap: "8px", textDecoration: "none" }}
        >
          Go to Workspaces
          <ArrowRight size={16} />
        </Link>
      </div>
    </div>
  );
}

const AIPage: React.FC = () => {
  const [events, setEvents] = useState<SemanticEvent[]>([]);

  const [query, setQuery] = useState("");
  const [reasoning, setReasoning] = useState<ReasoningResponse | null>(null);
  const [queryLoading, setQueryLoading] = useState(false);
  const [queryError, setQueryError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [deviceID, setDeviceID] = useState("");
  
  const [eventsPage, setEventsPage] = useState(1);
  const eventsPerPage = 10;
  
  const { selectedWorkspaceId, workspaceDevices } = useWorkspaceContext();
  const { data: sessions, isLoading: sessionsLoading } = useSessions(selectedWorkspaceId);

  const activeSession = useMemo(() => {
    return sessions?.find(s => s.status === "RUNNING") || null;
  }, [sessions]);

  const { data: statusInfo } = useDeviceStatus(deviceID);
  const { data: activeIncidents } = useSessionActiveIncidents(activeSession?.id);

  const filteredActiveIncidents = useMemo(() => {
    if (!activeIncidents) return [];
    if (!deviceID) return activeIncidents;
    return activeIncidents.filter(inc => inc.device_id === deviceID);
  }, [activeIncidents, deviceID]);

  const activeDeviceIds = useMemo(() => new Set(workspaceDevices.map(d => d.id)), [workspaceDevices]);

  const filteredEvents = useMemo<SemanticEvent[]>(() => {
    return events.filter((ev: SemanticEvent) => activeDeviceIds.has(ev.device_id));
  }, [events, activeDeviceIds]);

  const paginatedEvents = useMemo<SemanticEvent[]>(() => {
    const start = (eventsPage - 1) * eventsPerPage;
    return filteredEvents.slice(start, start + eventsPerPage);
  }, [filteredEvents, eventsPage, eventsPerPage]);

  const totalEventsPages = useMemo(() => {
    return Math.ceil(filteredEvents.length / eventsPerPage);
  }, [filteredEvents, eventsPerPage]);

  // Reset page when workspace changes
  useEffect(() => {
    setEventsPage(1);
  }, [selectedWorkspaceId]);

  useEffect(() => {
    fetchBaseData();
  }, []);

  const fetchBaseData = async () => {
    try {
      // Fetch up to 100 events to ensure we have a good buffer for the workspace devices filter
      const evs = await api.ai.listEvents(100, 0);
      setEvents(evs);
    } catch (err) {
      console.error("Failed to fetch AI data", err);
    } finally {
      setLoading(false);
    }
  };

  const handleQuery = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setQueryLoading(true);
    setReasoning(null);
    setQueryError(null);
    try {
      const resp = await api.ai.query(query);
      setReasoning(resp);
    } catch (err) {
      console.error("Query failed", err);
      setQueryError((err as Error).message || "An error occurred while processing your query.");
    } finally {
      setQueryLoading(false);
    }
  };

  if (loading || sessionsLoading) {
    return (
      <div className="workspace">
        <div className="empty-state">
          <Clock className="animate-spin" size={24} />
          <h3>Initializing Context</h3>
          <p>Assembling fleet intelligence from operational memory...</p>
        </div>
      </div>
    );
  }

  if (!activeSession) {
    return (
      <SessionRequiredPrompt
        title="Session Required for AI Insights"
        description="AI insights, semantic event correlation, and natural language reasoning require an active operational session in this workspace."
      />
    );
  }

  return (
    <>
      <PageHeader 
        title="AI Operational Intelligence" 
        description="Real-time semantic reasoning, statistical anomaly detection, and incident correlation."
        actions={
          <div style={{ display: "flex", gap: "12px", alignItems: "center" }}>
            <select 
              value={deviceID} 
              onChange={(e) => setDeviceID(e.target.value)}
              className="button secondary compact"
              style={{ minWidth: 180, background: "var(--surface-2)" }}
            >
              <option value="">All Devices</option>
              {workspaceDevices.map(d => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
            <div className="live-badge online"><span className="live-dot" />LOCAL INFERENCE ACTIVE</div>
          </div>
        }
      />

      <div className="stat-grid four" style={{ marginBottom: 18 }}>
        <StatCard
          label={deviceID ? "Device Status" : "Active Devices"}
          value={deviceID ? (statusInfo?.status.toUpperCase() || "OFFLINE") : workspaceDevices.length}
          detail={deviceID ? "Current device connection state" : "Monitored devices in session"}
          tone={deviceID ? (statusInfo?.status === "online" ? "success" : "neutral") : "neutral"}
        />
        <StatCard
          label="Worst Severity"
          value={
            deviceID 
              ? (statusInfo?.severity.toUpperCase() || "HEALTHY")
              : (activeIncidents && activeIncidents.some(i => i.severity === "critical") ? "CRITICAL" : (activeIncidents && activeIncidents.some(i => i.severity === "warning") ? "WARNING" : "HEALTHY"))
          }
          detail="Highest active anomaly level"
          tone={
            (deviceID ? statusInfo?.severity : (activeIncidents && activeIncidents.some(i => i.severity === "critical") ? "critical" : (activeIncidents && activeIncidents.some(i => i.severity === "warning") ? "warning" : "healthy"))) === "critical" 
              ? "danger" 
              : ((deviceID ? statusInfo?.severity : (activeIncidents && activeIncidents.some(i => i.severity === "critical") ? "critical" : (activeIncidents && activeIncidents.some(i => i.severity === "warning") ? "warning" : "healthy"))) === "warning" ? "warning" : "success")
          }
        />
        <StatCard
          label="Active Incidents"
          value={deviceID ? (statusInfo?.active_incidents ?? 0) : (activeIncidents?.length ?? 0)}
          detail="Currently open anomaly streams"
          tone={(deviceID ? (statusInfo?.active_incidents ?? 0) : (activeIncidents?.length ?? 0)) > 0 ? "warning" : "success"}
        />
        <StatCard
          label="Warning / Critical Counts"
          value={
            deviceID
              ? `${statusInfo?.open_incidents.filter(inc => inc.severity === "warning").length ?? 0} / ${statusInfo?.open_incidents.filter(inc => inc.severity === "critical").length ?? 0}`
              : `${activeIncidents?.filter(i => i.severity === "warning").length ?? 0} / ${activeIncidents?.filter(i => i.severity === "critical").length ?? 0}`
          }
          detail="Warning vs Critical alerts"
        />
      </div>

      <div className="split">
        <div className="grid">
          {/* Query Engine */}
          <Panel
            title={
              <span style={{ display: "flex", alignItems: "center", gap: "8px", color: "var(--accent)" }}>
                <Brain size={18} /> Semantic Query Engine
              </span>
            }
          >
            <form onSubmit={handleQuery} style={{ display: "flex", gap: "12px" }}>
              <div className="search-input" style={{ flex: 1, height: 38, background: "var(--surface-2)" }}>
                <Search size={16} />
                <input
                  type="text"
                  placeholder="Ask about fleet state... (e.g. 'Recent anomalies on device-4?')"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </div>
              <button 
                type="submit" 
                className="button primary"
                disabled={queryLoading}
                style={{ paddingLeft: 24, paddingRight: 24 }}
              >
                {queryLoading ? "Reasoning..." : "Analyze"}
              </button>
            </form>

            {queryError && (
              <div style={{ marginTop: "12px", padding: "10px 14px", borderRadius: "6px", backgroundColor: "rgba(239, 68, 68, 0.1)", border: "1px solid rgba(239, 68, 68, 0.2)", color: "var(--danger)", fontSize: "14px", display: "flex", alignItems: "center", gap: "8px" }}>
                <AlertCircle size={16} style={{ flexShrink: 0 }} />
                <span><strong>Query Error:</strong> {queryError}</span>
              </div>
            )}

            {reasoning && (
              <div className="ai-reasoning-panel">
                <div className="ai-reasoning-header">
                  <h3>Analysis Result</h3>
                  <span className={`ai-confidence-badge ${reasoning.confidence > 0 ? '' : 'low-confidence'}`}>
                    {(reasoning.confidence * 100).toFixed(0)}% CONFIDENCE
                  </span>
                </div>
                
                {reasoning.summary ? (
                  <>
                    <p className="ai-reasoning-summary">
                      {reasoning.summary}
                    </p>

                    <div className="grid two">
                      <div>
                        <span className="ai-evidence-title">Evidence Chain</span>
                        {reasoning.evidence && reasoning.evidence.length > 0 ? (
                          <ul className="ai-evidence-list">
                            {reasoning.evidence.map((ev, i) => (
                              <li key={i} className="ai-evidence-item">
                                <ShieldCheck size={12} className="ai-evidence-icon" />
                                <span>{ev}</span>
                              </li>
                            ))}
                          </ul>
                        ) : (
                          <span className="muted" style={{ fontSize: "12px" }}>No evidence referenced.</span>
                        )}
                      </div>
                      <div>
                        <span className="ai-suggestions-title">Remediation Suggestions</span>
                        {reasoning.suggested_actions && reasoning.suggested_actions.length > 0 ? (
                          <div className="ai-suggestions-list">
                            {reasoning.suggested_actions.map((act, i) => (
                              <div key={i} className="ai-remediation-item">
                                <span>{act}</span>
                                <ArrowRight size={10} />
                              </div>
                            ))}
                          </div>
                        ) : (
                          <span className="muted" style={{ fontSize: "12px" }}>No suggestions available.</span>
                        )}
                      </div>
                    </div>
                  </>
                ) : (
                  <p className="muted" style={{ margin: 0, fontSize: "14px", fontStyle: "italic" }}>
                    No matching information or anomalies found in current fleet context. Try refining your query.
                  </p>
                )}
              </div>
            )}
          </Panel>

          {/* Semantic Feed */}
          <Panel
            title={
              <span style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Zap size={16} style={{ color: "var(--warning)" }} /> Semantic Operational Feed
              </span>
            }
            subtitle={`${filteredEvents.length} events detected`}
          >
            <div className="table-wrap">
              <table className="ai-table">
                <thead>
                  <tr>
                    <th>Source / Type</th>
                    <th>Detail</th>
                    <th style={{ textAlign: "center" }}>Severity</th>
                    <th style={{ textAlign: "right" }}>Observed</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedEvents.length === 0 ? (
                    <tr>
                      <td colSpan={4}>
                        <EmptyState
                          title="No events detected"
                          description="The semantic feed will populate as devices report telemetry and status updates."
                        />
                      </td>
                    </tr>
                  ) : (
                    paginatedEvents.map((ev: SemanticEvent) => (
                      <tr key={ev.id}>
                        <td>
                          <strong>{ev.type.replace('_', ' ')}</strong>
                          <div className="mono" style={{ fontSize: "10px", color: "var(--faint)", textTransform: "uppercase", marginTop: "2px" }}>
                            {ev.source}
                          </div>
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "var(--text)" }}>{ev.title}</div>
                          <div className="muted" style={{ fontSize: "12px", marginTop: "2px", display: "-webkit-box", WebkitLineClamp: 1, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                            {ev.summary}
                          </div>
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <StatusChip value={ev.severity} />
                        </td>
                        <td className="mono" style={{ textAlign: "right", color: "var(--faint)", fontSize: "11px" }}>
                          {new Date(ev.created_at).toLocaleTimeString()}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {totalEventsPages > 1 && (
              <div style={{ display: "flex", justifyContent: "center", marginTop: "16px" }}>
                <Pagination
                  current={eventsPage}
                  total={totalEventsPages}
                  onChange={(page) => setEventsPage(page)}
                />
              </div>
            )}
          </Panel>
        </div>

        <aside className="grid">
          {/* Active Incidents */}
          <Panel
            title={
              <span style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <AlertCircle size={14} className="text-danger" /> Active Incidents
              </span>
            }
            subtitle={`${filteredActiveIncidents.length} open incidents`}
          >
            <div className="grid" style={{ gap: 12 }}>
              {filteredActiveIncidents.map((inc, index) => (
                <div key={`${inc.device_id}-${inc.metric}-${index}`} className="incident-card">
                  <div className="incident-card-header">
                    <StatusChip value={inc.severity} />
                    <time className="incident-time">
                      <Clock size={10} />
                      {new Date(inc.start_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </time>
                  </div>
                  <h4 className="incident-title">{inc.metric} ({inc.incident_type})</h4>
                  <p className="incident-summary">{inc.summary}</p>
                  <div className="incident-devices">
                    <CopyableID id={inc.device_id} length={6} />
                    <span className="mono" style={{ fontSize: "11px", color: "var(--muted)", marginLeft: "auto" }}>
                      Seen: {inc.occurrences}x | Peak: {inc.peak_score.toFixed(2)}
                    </span>
                  </div>
                </div>
              ))}
              {filteredActiveIncidents.length === 0 && (
                <div className="empty-incidents">
                  No active incidents detected
                </div>
              )}
            </div>
          </Panel>

          {/* Quick Stats */}
          <Panel title="AI Engine Status">
            <div className="ai-stats-list">
              <div className="ai-stat-row">
                <span className="ai-stat-label">Embedding Model</span>
                <span className="ai-stat-val mono">nomic-embed</span>
              </div>
              <div className="ai-stat-row">
                <span className="ai-stat-label">Reasoning Provider</span>
                <span className="ai-stat-val mono">Groq (Llama-3.3)</span>
              </div>
              <div className="ai-stat-row">
                <span className="ai-stat-label">Vector Index</span>
                <span className="ai-stat-val mono text-success">Optimized (HNSW)</span>
              </div>
            </div>
          </Panel>
        </aside>
      </div>
    </>
  );
};

export default AIPage;
