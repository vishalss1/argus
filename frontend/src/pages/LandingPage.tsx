import { Link } from "react-router-dom";
import { Bell, Boxes, Database, GitBranch, RadioTower, Rocket } from "lucide-react";

const features = [
  { icon: Boxes, title: "Device Registry", text: "Source of truth for fleet metadata, firmware versions, health, and last-seen state." },
  { icon: RadioTower, title: "Telemetry Ingestion", text: "HTTP and MQTT ingestion paths persist metrics and trigger rule evaluation." },
  { icon: Rocket, title: "OTA Lifecycle", text: "Firmware upload, checksum tracking, manifest generation, and deployment acknowledgement." },
  { icon: GitBranch, title: "Command Control", text: "Send JSON-backed device commands and track ACK or NACK result state." },
  { icon: Bell, title: "Rules and Alerts", text: "Operator-defined thresholds evaluate telemetry and create alert records." },
  { icon: Database, title: "Operational Storage", text: "PostgreSQL, Redis, MinIO, and observability services support the runtime." }
];

const pipeline = [
  ["Edge Nodes", "ESP32, Linux, gateways"],
  ["MQTT or HTTP", "Telemetry and heartbeats"],
  ["API", "Validation and ingestion"],
  ["Rules", "Threshold evaluation"],
  ["Storage", "Postgres, Redis, MinIO"],
  ["Operators", "Dashboard and API docs"]
];

export function LandingPage() {
  return (
    <>
      <section className="hero">
        <div className="hero-inner">
          <span className="eyebrow">Distributed Fleet Intelligence</span>
          <h1>
            Orchestrate every <span className="accent-text">edge node at scale</span>
          </h1>
          <p>
            ARGUS is a production-grade IoT fleet orchestration system for device
            registry, telemetry, commands, OTA delivery, shadow state, and alerting.
          </p>
          <div className="hero-actions">
            <Link className="button primary" to="/dashboard">
              Open Dashboard
            </Link>
            <Link className="button secondary" to="/documentation">
              Read Documentation
            </Link>
          </div>
        </div>
      </section>
      <section className="section">
        <h2>Built for Real Operational Workflows</h2>
        <p className="section-lead">
          The platform surfaces the backend capabilities implemented in the ARGUS API without
          fabricated counters or decorative controls.
        </p>
        <div className="grid three">
          {features.map((feature) => {
            const Icon = feature.icon;
            return (
              <article className="feature-card" key={feature.title}>
                <Icon size={20} aria-hidden />
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
          A layered event-driven backbone connects devices, ingestion, evaluation, storage,
          and operator interfaces.
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
    </>
  );
}
