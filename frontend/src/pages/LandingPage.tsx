import { Link } from "react-router-dom";

const architecturePillars = [
  {
    title: "Device State",
    text: "Desired vs reported state synchronization across heterogeneous firmware."
  },
  {
    title: "OTA Delivery",
    text: "Versioned firmware rollout with chunked uploads, SHA-256 verification and ACK tracking."
  },
  {
    title: "Event Processing",
    text: "Stream telemetry through an event bus with rule evaluation at infrastructure scale."
  }
];

const capabilities = [
  {
    name: "Device Registry",
    desc: "Central source of truth for every device, firmware version, capability and health state."
  },
  {
    name: "Telemetry Pipeline",
    desc: "High-throughput ingestion through MQTT and HTTP gateways, routed to durable event streams."
  },
  {
    name: "Digital Twin",
    desc: "Desired and reported state reconciliation. Tracks drift, version history and convergence."
  },
  {
    name: "Command & Control",
    desc: "Reliable command dispatch with ACK/NACK tracking, idempotency and bounded retry."
  }
];

const principles = [
  {
    name: "Event Driven",
    desc: "Every state change is a published event. No silent updates, no missed transitions."
  },
  {
    name: "Failure Aware",
    desc: "Networks fail. Buffers, retries and shadow state are first-class, not afterthoughts."
  },
  {
    name: "State First",
    desc: "Desired and reported state are explicit. The system converges, it does not assume."
  },
  {
    name: "Horizontal Scale",
    desc: "Workers, partitions and event topics scale independently. No monolithic bottlenecks."
  },
  {
    name: "Operational Simplicity",
    desc: "Deploy, observe, recover. A small surface area that does not require a dedicated team."
  },
  {
    name: "Infrastructure Agnostic",
    desc: "Runs on bare metal, VMs or containers. Cloud, on-prem and edge are all supported."
  }
];

const techStack = [
  "Go", "PostgreSQL", "Redis", "MinIO", "Redpanda", "MQTT", "NATS",
  "Prometheus", "Grafana", "Loki"
];

const flowSteps = ["Devices", "MQTT", "Gateway", "Redpanda", "Rules", "Storage"];

export function LandingPage() {
  return (
    <>
      <section className="hero">
        <div className="container">
          <div className="hero-grid">
            <div>
              <h1 className="hero-heading">
                The platform for
                <br />
                <span className="sketch">IoT Fleet</span>
                <br />
                orchestration.
              </h1>
              <p className="hero-sub">
                Device management, telemetry ingestion, OTA delivery,
                rule evaluation and fleet operations at infrastructure scale.
              </p>
              <div className="hero-cta">
                <Link className="btn-outline" to="/dashboard">
                  Open Platform
                </Link>
                <Link className="btn-outline" to="/architecture">
                  Read Architecture
                </Link>
              </div>
            </div>
            <div>
              <HeroWireframe />
            </div>
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Architecture</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 32 }}>
            Built for unreliable networks.
          </h2>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 96 }}>
            ARGUS is designed around intermittent connectivity, event-driven processing
            and fleet-scale device management.
          </p>
          <div className="cols-3">
            {architecturePillars.map((p) => (
              <div key={p.title}>
                <h3>{p.title}</h3>
                <p>{p.text}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Capabilities</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            What ARGUS does.
          </h2>
          <div className="dl-list">
            {capabilities.map((c, idx) => (
              <div className="dl-row" key={c.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name">{c.name}</div>
                </div>
                <div className="dl-desc">{c.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">System flow</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 24 }}>
            How data moves.
          </h2>
          <p className="t-body" style={{ maxWidth: 640, marginBottom: 64 }}>
            A single request path from edge to storage. Every stage is observable, durable
            and independently scalable.
          </p>
          <SystemFlow />
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Principles</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Engineering principles.
          </h2>
          <div className="principles">
            {principles.map((p, idx) => (
              <div className="principle" key={p.name}>
                <span className="principle-num">{String(idx + 1).padStart(2, "0")}</span>
                <h3>{p.name}</h3>
                <p>{p.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Technology</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 64 }}>
            Built with.
          </h2>
          <div className="tech-cluster">
            <span className="tech-prompt">$</span>
            <span className="tech-name">go</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">postgresql</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">redis</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">minio</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">redpanda</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">mqtt</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">nats</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">prometheus</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">grafana</span>
            <span className="tech-sep">·</span>
            <span className="tech-name">loki</span>
          </div>
        </div>
      </section>

      <section className="final-cta">
        <div className="container">
          <h2>
            Operate fleets.
            <br />
            Not spreadsheets.
          </h2>
          <p className="final-cta-sub">
            Manage devices, telemetry, firmware and automation
            from a single control plane.
          </p>
          <div className="hero-cta" style={{ justifyContent: "center" }}>
            <Link className="btn-outline" to="/dashboard">
              Open Platform
            </Link>
            <Link className="btn-outline" to="/documentation">
              Documentation
            </Link>
          </div>
        </div>
      </section>

      <footer className="site-footer">
        <div className="container">
          <div className="site-footer-inner">
            <span>ARGUS</span>
            <span>IoT Fleet Orchestration Platform</span>
            <span>v2.0</span>
          </div>
        </div>
      </footer>
    </>
  );
}

/* ── Hero right column: animated architectural wireframe ── */
function HeroWireframe() {
  const stages = flowSteps;
  const w = 520;
  const h = 520;
  const cx = w / 2;
  const startY = 30;
  const endY = h - 30;
  const gap = (endY - startY) / (stages.length - 1);
  const nodeR = 22;

  return (
    <svg className="wireframe-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="System architecture wireframe">
      {stages.map((label, i) => {
        const y = startY + i * gap;
        return (
          <g key={label}>
            <line
              className="wf-line"
              x1={cx}
              y1={i === 0 ? y + nodeR : y - nodeR}
              x2={cx}
              y2={i === stages.length - 1 ? y - nodeR : y + nodeR}
            />
            <circle
              className="wf-node"
              cx={cx}
              cy={y}
              r={nodeR}
            />
            <text
              className="wf-label"
              x={cx + nodeR + 16}
              y={y + 4}
            >
              {label}
            </text>
            {i < stages.length - 1 && (
              <text
                className="wf-label"
                x={cx - nodeR - 16}
                y={y + 4}
                textAnchor="end"
                style={{ fill: "var(--text-muted)" }}
              >
                ↓
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

/* ── System flow section: horizontal SVG ── */
function SystemFlow() {
  const stages = flowSteps;
  const w = 1100;
  const h = 120;
  const padX = 60;
  const cy = h / 2;
  const colW = (w - padX * 2) / stages.length;
  const nodeW = 90;
  const nodeH = 36;

  return (
    <svg className="flow-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="System data flow">
      {stages.map((label, i) => {
        const cx = padX + colW * i + colW / 2;
        const x = cx - nodeW / 2;
        return (
          <g key={label}>
            {i < stages.length - 1 && (
              <line
                className="flow-line"
                x1={cx + nodeW / 2 + 4}
                y1={cy}
                x2={cx + colW - nodeW / 2 - 4}
                y2={cy}
              />
            )}
            <rect
              className="flow-node"
              x={x}
              y={cy - nodeH / 2}
              width={nodeW}
              height={nodeH}
            />
            <text
              className="flow-label"
              x={cx}
              y={cy + 4}
              textAnchor="middle"
            >
              {label}
            </text>
          </g>
        );
      })}
    </svg>
  );
}
