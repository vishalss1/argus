import { useParams, useNavigate } from "react-router-dom";
import { Activity, AlertTriangle, CheckCircle, XCircle } from "lucide-react";
import { useSession, useSessionStatistics, useStopSession } from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard } from "../components/ui";

// While a session runs the backend persists only partial statistics
// (alerts, critical events, ...). Uptime and telemetry totals are computed when
// the session stops (session.Manager.StopSession), so show a dash for those
// rather than a misleading 0.
const pendingLabel = "—";

export function SessionDashboardPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const stopSession = useStopSession();
  const { data: stats, isLoading: statsLoading, error: statsErr } = useSessionStatistics(sessionID ?? "");
  const { data: session } = useSession(sessionID ?? "");
  const isRunning = session?.status === "RUNNING";

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
          value={stats && !isRunning ? `${stats.uptime_percentage.toFixed(2)}%` : pendingLabel}
          detail="Session health"
          tone={stats && !isRunning ? (stats.uptime_percentage < 95 ? "danger" : "success") : "neutral"}
        />
        <StatCard
          label="Events"
          value={stats && !isRunning ? stats.messages_processed.toLocaleString() : pendingLabel}
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

      {(isRunning || (!stats && !statsLoading)) && (
        <p className="muted" style={{ marginTop: 12, marginBottom: 0 }}>
          {statsErr
            ? "Session statistics are unavailable right now."
            : "Alert and critical-event counts update about every 30 seconds. Uptime and event totals are computed when the session stops."}
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
