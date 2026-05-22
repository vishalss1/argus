import { PageHeader, Panel } from "../components/ui";

export function AboutPage() {
  return (
    <>
      <section className="hero">
        <div className="hero-inner">
          <span className="eyebrow">About ARGUS</span>
          <h1>
            The platform behind <span className="accent-text">modern fleet intelligence</span>
          </h1>
          <p>
            ARGUS is a distributed IoT fleet intelligence platform by Vishal Shetagar,
            designed as an engineering-grade system for learning, operation, and extension.
          </p>
        </div>
      </section>
      <section className="section">
        <PageHeader
          title="What ARGUS Solves"
          description="The project focuses on concrete operational problems in fleet management rather than a decorative demo surface."
        />
        <div className="grid two">
          <Panel title="No Unified Control Plane">
            <p className="muted">Device registry, telemetry, commands, OTA, shadows, rules, and alerts are exposed through one API and frontend.</p>
          </Panel>
          <Panel title="Unreliable Connectivity">
            <p className="muted">Heartbeat state, command acknowledgements, OTA results, and shadow drift keep operators aware of fleet condition.</p>
          </Panel>
          <Panel title="Firmware Risk">
            <p className="muted">MinIO-backed firmware artifacts include version, size, checksum, object key, and deployment manifests.</p>
          </Panel>
          <Panel title="Scale Observability">
            <p className="muted">Prometheus, Grafana, Loki, and Promtail provide the observability foundation for API and infrastructure behavior.</p>
          </Panel>
        </div>
      </section>
      <section className="section">
        <Panel title="Creator">
          <div className="settings-row">
            <div>
              <strong>Vishal Shetagar</strong>
              <p className="muted">Created and developed ARGUS, a distributed IoT fleet intelligence platform.</p>
            </div>
            <span className="status-chip tone-info">ARGUS</span>
          </div>
        </Panel>
      </section>
    </>
  );
}
