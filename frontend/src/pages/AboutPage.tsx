import { PageHeader, Panel } from "../components/ui";

const techStack = [
  {
    category: "Backend",
    color: "var(--accent)",
    items: ["Go 1.26", "chi Router", "golang-migrate"]
  },
  {
    category: "Storage",
    color: "var(--accent)",
    items: ["PostgreSQL 16", "Redis 7", "MinIO"]
  },
  {
    category: "Messaging",
    color: "var(--accent)",
    items: ["Redpanda", "MQTT (Paho)", "NATS (roadmap)"]
  },
  {
    category: "Observability",
    color: "var(--accent)",
    items: ["Prometheus", "Grafana", "Loki + Promtail"]
  }
];

export function AboutPage() {
  return (
    <>
      <section className="hero">
        <div className="hero-inner">
          <span className="eyebrow">About ARGUS</span>
          <h1>
            The Platform Behind <span className="accent-text">Modern Fleet Intelligence</span>
          </h1>
          <p>
            ARGUS started as a student project and grew into a production-grade distributed
            IoT orchestration system — built to learn, designed to last.
          </p>
        </div>
      </section>
      <section className="section">
        <PageHeader
          title="What Problem ARGUS Solves"
        />
        <div className="grid two">
          <Panel title="No Unified Control Plane">
            <p className="muted">IoT fleets typically lack a single interface for telemetry, OTA, and remote commands. ARGUS unifies them.</p>
          </Panel>
          <Panel title="Unreliable Connectivity">
            <p className="muted">Edge devices drop off constantly. ARGUS handles buffering, retry queues, and shadow state sync gracefully.</p>
          </Panel>
          <Panel title="Firmware Risk">
            <p className="muted">OTA updates on embedded hardware are dangerous without rollback. ARGUS adds checksums, staged rollouts, and ACK/NACK tracking.</p>
          </Panel>
          <Panel title="Scale Observability">
            <p className="muted">Monitoring thousands of heterogeneous nodes is complex. ARGUS pipelines telemetry through Redpanda with rule evaluation.</p>
          </Panel>
        </div>
      </section>
      <section className="section">
        <h2>Technology Stack</h2>
        <div className="tech-grid">
          {techStack.map((stack) => (
            <div className="tech-card" key={stack.category}>
              <h4>{stack.category}</h4>
              <ul>
                {stack.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </section>
      <section className="section">
        <div className="creator-card">
          <div className="creator-avatar">VS</div>
          <div className="creator-info">
            <strong>Vishal Shetagar</strong>
            <p>Created & Developed ARGUS — Distributed IoT Fleet Intelligence Platform</p>
            <small>Manipal Institute of Technology Bengaluru · 2028</small>
          </div>
        </div>
      </section>
    </>
  );
}
