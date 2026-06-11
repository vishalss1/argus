import { Activity, RefreshCw } from "lucide-react";
import { EmptyState, ErrorState, PageHeader, Panel, StatCard, StatusChip } from "../components/ui";
import { useHealth, useMetrics } from "../hooks/useArgusData";
import { parsePrometheusMetrics } from "../services/api";

export function ObservabilityPage() {
  const health = useHealth();
  const metrics = useMetrics();
  const samples = metrics.data ? parsePrometheusMetrics(metrics.data) : [];
  const httpSamples = samples.filter((sample) => sample.name.includes("http"));

  return (
    <>
      <PageHeader
        eyebrow="Observe"
        title="Observability"
        description="Inspect API service metrics, health statistics, and metrics telemetry logs."
        actions={<button className="button secondary" onClick={() => { void health.refetch(); void metrics.refetch(); }}><RefreshCw size={15} />Refresh</button>}
      />
      <div className="stat-grid">
        <StatCard label="API Health" value={health.isError ? "Offline" : health.isLoading ? "Checking" : "Active"} tone={health.isError ? "danger" : health.isLoading ? "warning" : "success"} />
        <StatCard label="Metric Samples" value={samples.length} detail="Active metric samples" />
        <StatCard label="HTTP Samples" value={httpSamples.length} />
        <StatCard label="System" value={health.isError ? "Offline" : "Active"} tone={health.isError ? "danger" : "success"} />
      </div>
      <div className="split">
        <Panel title="API Status" subtitle="Backend application service status">
          {health.isError ? <ErrorState message={(health.error as Error).message} onRetry={() => void health.refetch()} /> : (
            <div className="settings-row">
              <div><strong>ARGUS API</strong><p className="muted">Exposes the system health state and availability status.</p></div>
              <StatusChip value={health.isLoading ? "checking" : "healthy"} />
            </div>
          )}
        </Panel>
        <Panel title="Service Status" subtitle="API service health">
          <div className="settings-list">
            <div className="settings-row"><span><strong>API Service</strong><p className="muted">Core API and fleet management service.</p></span></div>
            <div className="settings-row"><span><strong>Ingestion Service</strong><p className="muted">Telemetry ingestion and event processing.</p></span></div>
          </div>
        </Panel>
      </div>
      <div style={{ marginTop: 18 }}>
        <Panel title="Prometheus Samples" subtitle="Parsed time-series metric readings">
          {metrics.isError ? <ErrorState message={(metrics.error as Error).message} onRetry={() => void metrics.refetch()} /> : samples.length === 0 ? (
            <EmptyState title="No metrics loaded" description="Metrics will render when the metrics endpoint is reachable and returns samples." />
          ) : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Metric</th><th>Labels</th><th>Value</th></tr></thead>
                <tbody>
                  {samples.slice(0, 50).map((sample, index) => <tr key={`${sample.name}-${index}`}><td><Activity size={14} /> {sample.name}</td><td className="mono">{JSON.stringify(sample.labels)}</td><td>{sample.value}</td></tr>)}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </>
  );
}
