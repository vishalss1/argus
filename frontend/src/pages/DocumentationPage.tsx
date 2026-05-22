import { NavLink } from "react-router-dom";
import { PageHeader, Panel } from "../components/ui";

const nav = [
  ["intro", "Introduction"],
  ["run", "Run Locally"],
  ["devices", "Devices API"],
  ["telemetry", "Telemetry API"],
  ["commands", "Commands API"],
  ["ota", "OTA API"],
  ["rules", "Rules API"],
  ["observability", "Observability"]
];

const endpoints = [
  ["GET", "/devices/", "List registered devices"],
  ["POST", "/devices/", "Create a device"],
  ["PUT", "/devices/{deviceID}/", "Update a device"],
  ["DELETE", "/devices/{deviceID}/", "Delete a device"],
  ["POST", "/devices/{deviceID}/heartbeat", "Record heartbeat state"],
  ["POST", "/devices/{deviceID}/telemetry", "Ingest telemetry metrics"],
  ["GET", "/devices/{deviceID}/commands", "List commands for a device"],
  ["POST", "/devices/{deviceID}/commands", "Send a command"],
  ["POST", "/devices/{deviceID}/commands/{commandID}/ack", "ACK a command"],
  ["POST", "/devices/{deviceID}/commands/{commandID}/nack", "NACK a command"],
  ["GET", "/devices/{deviceID}/shadow", "Read device shadow"],
  ["PUT", "/devices/{deviceID}/shadow/desired", "Update desired shadow"],
  ["PUT", "/devices/{deviceID}/shadow/reported", "Update reported shadow"],
  ["GET", "/ota/firmware/", "List firmware artifacts"],
  ["POST", "/ota/firmware/", "Upload firmware"],
  ["POST", "/devices/{deviceID}/ota", "Create OTA deployment manifest"],
  ["GET", "/rules/", "List telemetry rules"],
  ["POST", "/rules/", "Create telemetry rule"],
  ["GET", "/alerts", "List generated alerts"],
  ["GET", "/healthz", "Health check"],
  ["GET", "/metrics", "Prometheus metrics"]
];

export function DocumentationPage() {
  return (
    <section className="section">
      <PageHeader
        eyebrow="Engineering Docs"
        title="ARGUS documentation"
        description="Frontend documentation generated from the implemented backend routes, handlers, DTOs, and runtime configuration."
      />
      <div className="docs-layout">
        <aside className="docs-sidebar">
          {nav.map(([id, label]) => <a key={id} href={`#${id}`}>{label}</a>)}
        </aside>
        <article className="docs-content">
          <Panel title="Introduction">
            <section id="intro">
              <p className="muted">
                ARGUS is a Go modular monolith for distributed IoT fleet intelligence. It exposes device registry,
                heartbeat, telemetry ingestion, commands, Redis-backed shadows, MinIO-backed OTA firmware,
                threshold rules, alerts, Swagger docs, and Prometheus metrics.
              </p>
            </section>
            <section id="run">
              <h2>Run Locally</h2>
              <pre className="code-block">{`docker compose -f deployments/compose/docker-compose.yml up -d postgres redis minio
go run ./cmd/api/main.go
cd frontend
npm run dev`}</pre>
            </section>
            <section id="devices">
              <h2>Devices API</h2>
              <p className="muted">Devices are the central resource. Heartbeats update status and last_seen.</p>
              <pre className="code-block">{`POST /devices/
{
  "name": "Line Controller",
  "type": "gateway",
  "firmware_version": "1.0.0",
  "metadata": {}
}`}</pre>
            </section>
            <section id="telemetry">
              <h2>Telemetry API</h2>
              <p className="muted">The backend currently exposes ingestion only. Rule evaluation happens after persistence.</p>
              <pre className="code-block">{`POST /devices/{deviceID}/telemetry
{
  "metrics": {
    "temperature": 32
  }
}`}</pre>
            </section>
            <section id="commands">
              <h2>Commands API</h2>
              <p className="muted">Commands are scoped to a device and move through pending, acked, or nacked states.</p>
            </section>
            <section id="ota">
              <h2>OTA API</h2>
              <p className="muted">Firmware uploads use multipart form data with version and firmware fields. Deployments return a manifest with a presigned download URL.</p>
            </section>
            <section id="rules">
              <h2>Rules API</h2>
              <p className="muted">Rules define metric, operator, threshold, and enabled state. Alerts are produced when telemetry violates enabled rules.</p>
            </section>
            <section id="observability">
              <h2>Observability</h2>
              <p className="muted">The API serves /healthz and /metrics. Grafana and Prometheus are available through the Docker Compose observability stack.</p>
            </section>
          </Panel>
        </article>
        <aside className="toc">
          <strong>API Reference</strong>
          <a href="/api/docs/index.html" target="_blank" rel="noreferrer">Swagger UI</a>
          <NavLink to="/observability">Observability</NavLink>
        </aside>
      </div>
      <div style={{ marginTop: 22 }}>
        <Panel title="Implemented Endpoints">
          <div className="table-wrap">
            <table>
              <thead><tr><th>Method</th><th>Path</th><th>Purpose</th></tr></thead>
              <tbody>
                {endpoints.map(([method, path, purpose]) => (
                  <tr key={`${method}-${path}`}>
                    <td><span className="status-chip tone-info">{method}</span></td>
                    <td className="mono">{path}</td>
                    <td>{purpose}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>
    </section>
  );
}
