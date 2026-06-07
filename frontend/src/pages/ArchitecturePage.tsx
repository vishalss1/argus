const layers = [
  {
    name: "Interfaces",
    desc: "Operator dashboard, public APIs and machine-to-machine endpoints. All access goes through this layer.",
    tech: ["REST", "WebSockets", "Webhooks"]
  },
  {
    name: "Ingestion",
    desc: "Stateless gateways that accept device telemetry over MQTT and HTTP. Buffers and rate-limits at the edge.",
    tech: ["MQTT", "HTTP", "gRPC"]
  },
  {
    name: "Orchestration",
    desc: "Core domain. Validates device behavior, manages firmware lifecycles and reconciles state.",
    tech: ["Go", "Domain Events", "CQRS"]
  },
  {
    name: "Rules Engine",
    desc: "Evaluates conditions over streaming telemetry. Triggers alerts and downstream automations.",
    tech: ["Operator DSL", "Windowed Aggregates", "Event Sourcing"]
  },
  {
    name: "Persistence",
    desc: "Durable storage for telemetry, configuration history, device shadows and firmware artifacts.",
    tech: ["PostgreSQL", "TimescaleDB", "Redis", "MinIO"]
  },
  {
    name: "Observability",
    desc: "Metrics, traces and logs across every layer. Production-grade visibility from the start.",
    tech: ["Prometheus", "Grafana", "Loki"]
  }
];

const lifecycle = ["Device", "MQTT", "Gateway", "Event Bus", "Rule Engine", "Storage", "Dashboard"];

const reliability = [
  { name: "Retries", desc: "Bounded retry policies with exponential backoff. No infinite loops, no silent drops." },
  { name: "Buffering", desc: "Local buffers at every gateway. Devices can lose connectivity and still deliver." },
  { name: "Shadow State", desc: "Desired vs reported state tracked explicitly. Convergence is observable, not assumed." },
  { name: "Idempotency", desc: "Commands and events are idempotent. Replay is safe. Ordering is preserved." },
  { name: "OTA Acknowledgements", desc: "Every device confirms installation. Stalled rollouts are detected and surfaced." }
];

const scaling = [
  { name: "Horizontal workers", desc: "All processing is partitioned and stateless. Add workers, get throughput." },
  { name: "Event processing", desc: "Backpressure-aware consumers. Lag is a first-class metric, not hidden." },
  { name: "Redis caching", desc: "Hot paths read from Redis. PostgreSQL stays the system of record, never the hot path." },
  { name: "PostgreSQL indexing", desc: "Query patterns are designed first. Indexes are not retrofitted at scale." },
  { name: "Redpanda partitioning", desc: "Topics are partitioned by tenant and device. Consumers scale per partition." }
];

export function ArchitecturePage() {
  return (
    <>
      <section className="section">
        <div className="container">
          <p className="t-eyebrow">System design</p>
          <h1 className="t-h1" style={{ maxWidth: 880, marginBottom: 32 }}>
            System architecture.
          </h1>
          <p className="t-body-lg" style={{ maxWidth: 720 }}>
            ARGUS separates orchestration logic from infrastructure adapters to support
            resilient fleet operations across unreliable networks.
          </p>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Layers</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Vertical stack.
          </h2>
          <div className="stack">
            {layers.map((l, idx) => (
              <div className="stack-row" key={l.name}>
                <span className="stack-num">{String(idx + 1).padStart(2, "0")}</span>
                <div className="stack-title">{l.name}</div>
                <div className="stack-desc">{l.desc}</div>
                <div className="stack-tech">{l.tech.join(" / ")}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Lifecycle</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 24 }}>
            Telemetry journey.
          </h2>
          <p className="t-body" style={{ maxWidth: 640, marginBottom: 64 }}>
            A single packet, from device to dashboard. Every hop is observable.
          </p>
          <LifecycleDiagram />
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Reliability</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Built for failure.
          </h2>
          <div className="dl-list">
            {reliability.map((r, idx) => (
              <div className="dl-row" key={r.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name">{r.name}</div>
                </div>
                <div className="dl-desc">{r.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Scaling</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Scaling characteristics.
          </h2>
          <div className="dl-list">
            {scaling.map((s, idx) => (
              <div className="dl-row" key={s.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name">{s.name}</div>
                </div>
                <div className="dl-desc">{s.desc}</div>
              </div>
            ))}
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

function LifecycleDiagram() {
  const stages = lifecycle;
  const w = 1100;
  const h = 100;
  const padX = 40;
  const cy = h / 2;
  const colW = (w - padX * 2) / stages.length;
  const nodeW = 100;
  const nodeH = 36;

  return (
    <svg className="flow-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Telemetry lifecycle">
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
