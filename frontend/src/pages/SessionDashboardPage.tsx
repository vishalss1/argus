import { useParams, useNavigate } from "react-router-dom";
import { Activity, AlertTriangle, CheckCircle, XCircle } from "lucide-react";
import { useSessionStatistics, useStopSession } from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard } from "../components/ui";

// Statistics are only computed and persisted once a session stops
// (session.Manager.StopSession -> UpsertStatistics), so a live session has no
// row to read. Show that plainly rather than inventing 100% uptime and zeros.
const pendingLabel = "—";

export function SessionDashboardPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const stopSession = useStopSession();
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");

  const handleStop = async (success: boolean) => {
    if (!sessionID) return;
    await stopSession.mutateAsync({ sessionID, success });
    navigate("/workspaces"); // Navigate back to workspaces
  };

  if (!sessionID) return <div>Invalid Session</div>;

  return (
    <>
      <PageHeader
        title={`Session Dashboard`}
        description={`Live operations for session ${sessionID.split("-")[0]}`}
        actions={
          <div style={{ display: "flex", gap: "12px" }}>
            <button className="button danger" onClick={() => handleStop(false)}>
              <XCircle size={16} /> Fail Mission
            </button>
            <button className="button primary" onClick={() => handleStop(true)}>
              <CheckCircle size={16} /> Complete Mission
            </button>
          </div>
        }
      />

      <div className="stat-grid four">
        <StatCard
          label="Uptime"
          value={stats ? `${stats.uptime_percentage.toFixed(2)}%` : pendingLabel}
          detail="Session health"
          tone={stats ? (stats.uptime_percentage < 95 ? "danger" : "success") : "neutral"}
        />
        <StatCard
          label="Events"
          value={stats ? stats.messages_processed.toLocaleString() : pendingLabel}
          detail="Telemetry processed"
        />
        <StatCard
          label="Critical Anomalies"
          value={stats ? stats.critical_events.toLocaleString() : pendingLabel}
          detail="AI Detected"
          tone={stats && stats.critical_events > 0 ? "danger" : "neutral"}
        />
        <StatCard
          label="Alerts"
          value={stats ? stats.alerts_count.toLocaleString() : pendingLabel}
          detail="Rule violations"
          tone={stats && stats.alerts_count > 0 ? "warning" : "neutral"}
        />
      </div>

      {!stats && !statsLoading && (
        <p className="muted" style={{ marginTop: 12, marginBottom: 0 }}>
          {statsErr
            ? "Session statistics are unavailable right now."
            : "Session statistics are computed when the session stops. Until then no uptime, event or alert totals are reported."}
        </p>
      )}

      <div className="grid two" style={{ marginTop: "24px" }}>
        <Panel title={<span><Activity size={18} style={{ marginRight: 8, verticalAlign: "middle" }} /> Live Telemetry Feed</span>}>
          <p className="muted">Live telemetry events will be streamed here via WebSocket.</p>
        </Panel>

        <Panel title={<span><AlertTriangle size={18} style={{ marginRight: 8, verticalAlign: "middle" }} /> Active Alerts</span>}>
           <p className="muted">Session-specific alerts will appear here.</p>
        </Panel>
      </div>
    </>
  );
}
