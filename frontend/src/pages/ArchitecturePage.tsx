import { PageHeader, Panel } from "../components/ui";

const layers = [
  ["Transport", "Chi HTTP routes expose devices, telemetry, commands, shadows, OTA, rules, alerts, metrics, and Swagger UI."],
  ["Domain", "Services validate fleet behavior and keep business logic independent from persistence and messaging."],
  ["Infrastructure", "PostgreSQL repositories, Redis shadows, MinIO firmware storage, MQTT ingestion, and Redpanda-compatible publishing."],
  ["Operations", "Prometheus metrics, Loki logs, Grafana dashboards, and local Docker Compose infrastructure."]
];

export function ArchitecturePage() {
  return (
    <section className="section">
      <PageHeader
        eyebrow="Architecture"
        title="Modular monolith with operational integrations"
        description="ARGUS keeps the core domain clean while integrating real storage, message brokers, cache, object storage, and observability adapters at the edges."
      />
      <div className="grid two">
        {layers.map(([title, text]) => (
          <Panel key={title} title={title}>
            <p className="muted">{text}</p>
          </Panel>
        ))}
      </div>
      <div style={{ marginTop: 18 }}>
        <Panel title="Core Data Flow">
          <div className="pipeline">
            {["Device", "MQTT/HTTP", "API", "Rules", "Storage", "Dashboard"].map((node) => (
              <div className="pipeline-node" key={node}>
                <strong>{node}</strong>
                <span>ARGUS runtime stage</span>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </section>
  );
}
