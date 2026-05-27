import { PageHeader, Panel } from "../components/ui";

const layers = [
  ["Ingestion & API Layer", "Secure REST and MQTT ingestion gateways to handle device connections, heartbeat registration, and real-time command dispatch."],
  ["Core Orchestration", "A decoupled core domain validating device behavior, tracking firmware distributions, and evaluating alert rules."],
  ["State Sync & Storage", "A highly resilient persistence layer for telemetry metrics, versioned configuration history, digital twins (device shadows), and firmware packages."],
  ["Observability & Analytics", "Production-grade system health tracking, telemetry log aggregation, and real-time visualization dashboards."]
];

export function ArchitecturePage() {
  return (
    <section className="section">
      <PageHeader
        eyebrow="Architecture"
        title="IoT fleet orchestration engine architecture"
        description="ARGUS separates core logic from infrastructure adapters to deliver robust, scalable, and highly available fleet operations."
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
            {["Device Node", "Ingestion Gateway", "Orchestration Engine", "Rules Engine", "State Storage", "Operator Portal"].map((node) => (
              <div className="pipeline-node" key={node}>
                <strong>{node}</strong>
                <span>Data processing stage</span>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </section>
  );
}
