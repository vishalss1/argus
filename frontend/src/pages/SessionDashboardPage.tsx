import { useParams, useNavigate } from "react-router-dom";
import { Activity, AlertTriangle, CheckCircle, XCircle } from "lucide-react";
import { useStopSession } from "../hooks/useArgusData";
import { PageHeader, Panel, StatCard } from "../components/ui";

export function SessionDashboardPage() {
  const { sessionID } = useParams<{ sessionID: string }>();
  const navigate = useNavigate();
  const stopSession = useStopSession();

  // Session stats - using defaults since the stats endpoint isn't implemented yet
  const stats = {
    uptime_percentage: 100,
    messages_processed: 0,
    critical_events: 0,
    alerts_count: 0
  };

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
          value={`${stats?.uptime_percentage?.toFixed(2) ?? 100}%`}
          detail="Session health"
          tone={stats && stats.uptime_percentage < 95 ? "danger" : "success"}
        />
        <StatCard
          label="Events"
          value={(stats?.messages_processed ?? 0).toLocaleString()}
          detail="Telemetry processed"
        />
        <StatCard
          label="Critical Anomalies"
          value={(stats?.critical_events ?? 0).toLocaleString()}
          detail="AI Detected"
          tone={stats && stats.critical_events > 0 ? "danger" : "neutral"}
        />
        <StatCard
          label="Alerts"
          value={(stats?.alerts_count ?? 0).toLocaleString()}
          detail="Rule violations"
          tone={stats && stats.alerts_count > 0 ? "warning" : "neutral"}
        />
      </div>

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
