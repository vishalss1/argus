import { Link } from "react-router-dom";
import { Bell, Boxes, Database, GitBranch, RadioTower, Rocket } from "lucide-react";

const features = [
  { icon: Boxes, title: "Device Registry", text: "Source of truth for fleet metadata. Track firmware, health, region, capabilities, and last-seen state across thousands of nodes." },
  { icon: RadioTower, title: "Telemetry Pipeline", text: "High-throughput ingestion over HTTP and MQTT. Stream events through Kafka for rule evaluation and persistent storage." },
  { icon: Rocket, title: "OTA Lifecycle", text: "Signed firmware delivery with chunked uploads, SHA-256 checksums, staged rollouts, and ACK/NACK tracking." },
  { icon: GitBranch, title: "Digital Twin", text: "Desired vs reported state sync. Tracks drift, version history, and reconciliation loops for every connected device." },
  { icon: Database, title: "Command & Control", text: "Send typed commands with JSON payloads. Full ACK/NACK delivery guarantees with timeout and retry policies." },
  { icon: Bell, title: "Rules Engine", text: "Reactive automation with operator-based threshold rules. Triggers alerts on metric conditions across the fleet." }
];

const stats = [
  ["HTTP + MQTT", "Telemetry Ingest"],
  ["PostgreSQL", "Device Registry"],
  ["Redis", "Shadow State"],
  ["MinIO", "Firmware Storage"]
];

const pipeline = [
  ["Edge Nodes", "ESP32 / RPi / Linux"],
  ["MQTT Broker", "EMQX / Mosquitto"],
  ["Ingestion API", "HTTP / gRPC"],
  ["Kafka / NATS", "Event Backbone"],
  ["Processing", "Rules + OTA + Cmds"],
  ["Storage", "PG + Redis + MinIO"]
];

export function LandingPage() {
  return (
    <>
      <section className="hero">
        <div className="hero-inner">
          <span className="eyebrow"><span className="green-dot" /> Distributed Fleet Intelligence</span>
          <h1>
            Orchestrate Every <span className="accent-text">Edge Node at Scale</span>
          </h1>
          <p>
            ARGUS is a production-grade IoT fleet orchestration system. Monitor thousands of
            edge devices, stream telemetry, push firmware updates, and automate responses
            with a distributed event-driven architecture.
          </p>
          <div className="hero-actions">
            <Link className="button primary" to="/dashboard">
              Open Dashboard →
            </Link>
            <Link className="button secondary" to="/documentation">
              Read the Docs
            </Link>
          </div>
        </div>
      </section>
      <section className="stats-bar">
        {stats.map(([value, label]) => (
          <div className="stat-item" key={label}>
            <strong>{value}</strong>
            <span>{label}</span>
          </div>
        ))}
      </section>
      <section className="section">
        <h2>Built for Real-World Complexity</h2>
        <p className="section-lead">
          ARGUS handles the operational challenges that production IoT deployments demand.
        </p>
        <div className="grid three">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <article className="feature-card" key={feature.title}>
                <div className="icon-box"><Icon size={18} aria-hidden /></div>
                <h3>{feature.title}</h3>
                <p>{feature.text}</p>
              </article>
            );
          })}
        </div>
      </section>
      <section className="section">
        <h2>From Edge to Cloud</h2>
        <p className="section-lead">
          A layered event-driven backbone connects thousands of edge nodes
          through a distributed message broker to processing services.
        </p>
        <div className="pipeline">
          {pipeline.map(([title, text]) => (
            <div className="pipeline-node" key={title}>
              <strong>{title}</strong>
              <span>{text}</span>
            </div>
          ))}
        </div>
      </section>
      <footer className="site-footer">
        <span>by Vishal Shetagar</span>
        <span>·</span>
        <span>ARGUS Fleet Intelligence Platform</span>
        <span>·</span>
        <span>MIT License</span>
      </footer>
    </>
  );
}
