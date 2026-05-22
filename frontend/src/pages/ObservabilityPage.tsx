import { Activity, ExternalLink, RefreshCw } from "lucide-react";
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
        description="Reads the backend health endpoint and Prometheus metrics exposed by the ARGUS API."
        actions={<button className="button secondary" onClick={() => { void health.refetch(); void metrics.refetch(); }}><RefreshCw size={15} />Refresh</button>}
      />
      <div className="stat-grid">
        <StatCard label="API Health" value={health.isError ? "Down" : health.isLoading ? "Checking" : "Healthy"} tone={health.isError ? "danger" : "success"} />
        <StatCard label="Metric Samples" value={samples.length} detail="/metrics output" />
        <StatCard label="HTTP Samples" value={httpSamples.length} />
        <StatCard label="Grafana" value=":3000" detail="Local compose port" tone="info" />
      </div>
      <div className="split">
        <Panel title="API Status" subtitle="GET /healthz">
          {health.isError ? <ErrorState message={(health.error as Error).message} onRetry={() => void health.refetch()} /> : (
            <div className="settings-row">
              <div><strong>ARGUS API</strong><p className="muted">Health endpoint returns HTTP 204 when available.</p></div>
              <StatusChip value={health.isLoading ? "checking" : "healthy"} />
            </div>
          )}
        </Panel>
        <Panel title="External Consoles" subtitle="Local Docker Compose">
          <div className="settings-list">
            <a className="settings-row" href="http://localhost:3000" target="_blank" rel="noreferrer"><span><strong>Grafana</strong><p className="muted">Dashboards and datasources provisioned under deployments/compose.</p></span><ExternalLink size={16} /></a>
            <a className="settings-row" href="http://localhost:9090" target="_blank" rel="noreferrer"><span><strong>Prometheus</strong><p className="muted">Scrapes ARGUS API metrics when compose observability is running.</p></span><ExternalLink size={16} /></a>
          </div>
        </Panel>
      </div>
      <div style={{ marginTop: 18 }}>
        <Panel title="Prometheus Samples" subtitle="GET /metrics">
          {metrics.isError ? <ErrorState message={(metrics.error as Error).message} onRetry={() => void metrics.refetch()} /> : samples.length === 0 ? (
            <EmptyState title="No metrics loaded" description="Metrics will render when the API /metrics endpoint is reachable and returns samples." />
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
