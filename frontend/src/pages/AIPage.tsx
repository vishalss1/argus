import React, { useEffect, useState } from "react";
import { api } from "../services/api";
import { SemanticEvent, Incident, ReasoningResponse } from "../types/api";
import { Panel, PageHeader, StatusChip, CopyableID, EmptyState } from "../components/ui";
import { Brain, Search, Clock, AlertCircle, CheckCircle, ArrowRight, ShieldCheck, Zap } from "lucide-react";

const AIPage: React.FC = () => {
  const [events, setEvents] = useState<SemanticEvent[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [query, setQuery] = useState("");
  const [reasoning, setReasoning] = useState<ReasoningResponse | null>(null);
  const [queryLoading, setQueryLoading] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchBaseData();
  }, []);

  const fetchBaseData = async () => {
    try {
      const [evs, incs] = await Promise.all([
        api.ai.listEvents(),
        api.ai.listIncidents()
      ]);
      setEvents(evs);
      setIncidents(incs);
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
    try {
      const resp = await api.ai.query(query);
      setReasoning(resp);
    } catch (err) {
      console.error("Query failed", err);
    } finally {
      setQueryLoading(false);
    }
  };

  if (loading) {
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

  return (
    <>
      <PageHeader 
        title="AI Operational Intelligence" 
        description="Real-time semantic reasoning, statistical anomaly detection, and incident correlation."
        actions={<div className="live-badge online"><span className="live-dot" />LOCAL INFERENCE ACTIVE</div>}
      />

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

            {reasoning && (
              <div className="ai-reasoning-panel">
                <div className="ai-reasoning-header">
                  <h3>Analysis Result</h3>
                  <span className="ai-confidence-badge">
                    {(reasoning.confidence * 100).toFixed(0)}% CONFIDENCE
                  </span>
                </div>
                
                <p className="ai-reasoning-summary">
                  {reasoning.summary}
                </p>

                <div className="grid two">
                  <div>
                    <span className="ai-evidence-title">Evidence Chain</span>
                    <ul className="ai-evidence-list">
                      {reasoning.evidence?.map((ev, i) => (
                        <li key={i} className="ai-evidence-item">
                          <ShieldCheck size={12} className="ai-evidence-icon" />
                          <span>{ev}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                  <div>
                    <span className="ai-suggestions-title">Remediation Suggestions</span>
                    <div className="ai-suggestions-list">
                      {reasoning.suggested_actions?.map((act, i) => (
                        <div key={i} className="ai-remediation-item">
                          <span>{act}</span>
                          <ArrowRight size={10} />
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            )}
          </Panel>

          {/* Semantic Feed */}
          <Panel
            title={
              <span style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <Zap size={16} className="text-warning" /> Semantic Operational Feed
              </span>
            }
            subtitle={`${events.length} events detected`}
          >
            <div className="table-wrap">
              <table className="ai-table text-sm">
                <thead>
                  <tr>
                    <th>Source / Type</th>
                    <th>Detail</th>
                    <th className="text-center">Severity</th>
                    <th className="text-right">Observed</th>
                  </tr>
                </thead>
                <tbody>
                  {events.length === 0 ? (
                    <tr>
                      <td colSpan={4}>
                        <EmptyState
                          title="No events detected"
                          description="The semantic feed will populate as devices report telemetry and status updates."
                        />
                      </td>
                    </tr>
                  ) : (
                    events.slice(0, 12).map(ev => (
                      <tr key={ev.id}>
                        <td>
                          <div className="flex flex-col">
                            <strong className="text-text">{ev.type.replace('_', ' ')}</strong>
                            <span className="text-[10px] text-faint mono uppercase">{ev.source}</span>
                          </div>
                        </td>
                        <td>
                          <div className="flex flex-col">
                            <span className="font-medium text-muted">{ev.title}</span>
                            <span className="text-xs text-faint line-clamp-1">{ev.summary}</span>
                          </div>
                        </td>
                        <td className="text-center">
                          <StatusChip value={ev.severity} />
                        </td>
                        <td className="text-right text-faint mono text-[11px]">
                          {new Date(ev.created_at).toLocaleTimeString()}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
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
            subtitle={`${incidents.filter(i => i.status === "open").length} open incidents`}
          >
            <div className="grid" style={{ gap: 12 }}>
              {incidents.filter(i => i.status === "open").map(inc => (
                <div key={inc.id} className="incident-card">
                  <div className="incident-card-header">
                    <StatusChip value={inc.severity} />
                    <time className="incident-time">
                      <Clock size={10} />
                      {new Date(inc.started_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                    </time>
                  </div>
                  <h4 className="incident-title">{inc.title}</h4>
                  <p className="incident-summary">{inc.summary}</p>
                  <div className="incident-devices">
                    {inc.device_ids.map(d => (
                      <CopyableID key={d} id={d} length={6} />
                    ))}
                  </div>
                </div>
              ))}
              {incidents.filter(i => i.status === "open").length === 0 && (
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
