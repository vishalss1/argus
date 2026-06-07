import React, { useEffect, useState, useMemo } from "react";
import { api } from "../services/api";
import { SemanticEvent, ReasoningResponse } from "../types/api";
import { Panel, PageHeader, StatusChip, CopyableID, EmptyState, StatCard, Pagination } from "../components/ui";
import { Brain, Search, Clock, AlertCircle, CheckCircle, ArrowRight, ShieldCheck, Zap, Activity, Heart, AlertTriangle } from "lucide-react";
import { useDeviceStatus, useSessionActiveIncidents, useSessions } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";

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
      const resp = await api.ai.query(query, deviceID);
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
          <Clock className="animate-spin" size={24} strokeWidth={1.5} />
          <h3>Initializing Context</h3>
          <p>Assembling fleet intelligence from operational memory...</p>
        </div>
      </div>
    );
  }

  return (
    <>
      <PageHeader 
        title="AI Operational Intelligence" 
        description={activeSession ? "Real-time operational reasoning, root-cause analysis, and incident correlation." : "Operational health analysis from persisted device state. Start a session to add live anomaly correlation."}
        actions={
          <div className="ai-page-actions">
            <select
              value={deviceID}
              onChange={(e) => setDeviceID(e.target.value)}
              className="ai-device-select"
            >
              <option value="">All Devices</option>
              {workspaceDevices.map(d => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
            <div className="live-badge online"><span className="live-dot" />Local Inference</div>
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
              <span className="ai-panel-title">
                <Brain size={18} strokeWidth={1.5} /> Operational Analysis Engine
              </span>
            }
          >
            <form onSubmit={handleQuery} className="ai-query-form">
              <div className="search-input" style={{ flex: 1, height: 38 }}>
                <Search size={15} strokeWidth={1.5} />
                <input
                  type="text"
                  placeholder="Ask why a device failed, summarize its health, or request remediation..."
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
              </div>
              <button
                type="submit"
                className="btn-inverse"
                disabled={queryLoading}
              >
                {queryLoading ? "Reasoning..." : "Analyze"}
              </button>
            </form>

            {queryError && (
              <div className="form-message error">
                <AlertCircle size={16} style={{ flexShrink: 0 }} />
                <span><strong>Query Error:</strong> {queryError}</span>
              </div>
            )}

            {reasoning && (
              <div className="ai-results">
                <div className="ai-reasoning-panel">
                <div className="ai-reasoning-header">
                  <div>
                    <span className="ai-result-intent">{reasoning.intent.replace(/_/g, " ")}</span>
                    <h3>Operational Analysis</h3>
                  </div>
                  <span className={`ai-confidence-badge ${reasoning.confidence > 0 ? '' : 'low-confidence'}`}>
                    {(reasoning.confidence * 100).toFixed(0)}% CONFIDENCE
                  </span>
                </div>
                
                  <p className="ai-reasoning-summary">{reasoning.summary}</p>
                </div>

                {reasoning.device_summary && (
                  <div className="ai-result-card health">
                    <div className="ai-result-card-header">
                      <span><Heart size={15} /> Device Health Summary</span>
                      <StatusChip value={reasoning.device_summary.severity} />
                    </div>
                    <div className="ai-health-score">
                      <strong>{reasoning.device_summary.healthScore}</strong><span>/100</span>
                      <div><b>{reasoning.device_summary.deviceName}</b><small>{reasoning.device_summary.deviceStatus} · {reasoning.device_summary.openIncidents} open · {reasoning.device_summary.recentIncidents} recent</small></div>
                    </div>
                    <ul className="ai-evidence-list">
                      {reasoning.device_summary.keyFindings.map((finding, i) => <li key={i} className="ai-evidence-item"><Activity size={12} className="ai-evidence-icon" /><span>{finding}</span></li>)}
                    </ul>
                  </div>
                )}

                {reasoning.root_cause_analysis && (
                  <div className="ai-result-card root-cause">
                    <div className="ai-result-card-header"><span><AlertTriangle size={15} /> Root Cause Analysis</span><b>{reasoning.root_cause_analysis.confidence}%</b></div>
                    <p className="ai-primary-cause">{reasoning.root_cause_analysis.primaryCause}</p>
                    <span className="ai-evidence-title">Evidence Chain</span>
                    <ul className="ai-evidence-list">
                      {reasoning.root_cause_analysis.supportingEvidence.map((ev, i) => <li key={i} className="ai-evidence-item"><ShieldCheck size={12} className="ai-evidence-icon" /><span>{ev}</span></li>)}
                    </ul>
                  </div>
                )}

                <div className="ai-result-card actions">
                  <div className="ai-result-card-header"><span><CheckCircle size={15} /> Recommended Actions</span></div>
                  {reasoning.remediations?.map(remediation => (
                    <div className="ai-remediation-reasoning" key={remediation.pattern}>
                      <b>{remediation.pattern}</b>
                      <span>{remediation.reasoning}</span>
                    </div>
                  ))}
                  <div className="ai-suggestions-list">
                    {(reasoning.suggested_actions ?? []).map((action, i) => <div key={i} className="ai-remediation-item"><b>{i + 1}</b><span>{action}</span><ArrowRight size={10} /></div>)}
                  </div>
                </div>

                {reasoning.related_devices && reasoning.related_devices.length > 0 && (
                  <div className="ai-result-card related">
                    <div className="ai-result-card-header"><span><Zap size={15} /> Related Devices</span></div>
                    {reasoning.related_devices.map(device => (
                      <div className="ai-related-device" key={device.deviceId}>
                        <div><b>{device.deviceName}</b><small>{device.sharedPatterns.join(" · ")}</small></div>
                        <strong>{device.similarity}%</strong>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </Panel>

          {/* Semantic Feed */}
          <Panel
            title={
              <span className="ai-panel-title">
                <Zap size={16} strokeWidth={1.5} /> Semantic Operational Feed
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
                          <div className="ai-event-source">
                            {ev.source}
                          </div>
                        </td>
                        <td>
                          <div className="ai-event-title">{ev.title}</div>
                          <div className="ai-event-summary">
                            {ev.summary}
                          </div>
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <StatusChip value={ev.severity} />
                        </td>
                        <td className="mono ai-event-time">
                          {new Date(ev.created_at).toLocaleTimeString()}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {totalEventsPages > 1 && (
              <div className="ai-pagination">
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
              <span className="ai-panel-title">
                <AlertCircle size={14} strokeWidth={1.5} /> Active Incidents
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
                      <Clock size={11} strokeWidth={1.5} />
                      {new Date(inc.start_time).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </time>
                  </div>
                  <h4 className="incident-title">{inc.metric} ({inc.incident_type})</h4>
                  <p className="incident-summary">{inc.summary}</p>
                  <div className="incident-devices">
                    <CopyableID id={inc.device_id} length={6} />
                    <span className="incident-meta-mono">
                      Seen: {inc.occurrences}x · Peak: {inc.peak_score.toFixed(2)}
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
