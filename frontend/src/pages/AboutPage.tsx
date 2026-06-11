const problems = [
  {
    name: "Fragmented Tooling",
    desc: "Device fleets are typically managed across vendor SDKs, ad-hoc MQTT brokers and homegrown dashboards. There is no single source of truth and no shared operational model."
  },
  {
    name: "Operational Complexity",
    desc: "At scale, device lifecycle spans registration, configuration, telemetry, OTA, rollback and decommissioning. Each stage has its own failure mode and its own operational reality."
  },
  {
    name: "Firmware Management",
    desc: "Deploying firmware to thousands of heterogeneous devices is a risk surface. Without checksums, staged rollouts and ACK tracking, every update is a chance for a fleet-wide incident."
  },
  {
    name: "Observability Gap",
    desc: "When a device misbehaves, the operator needs the packet, the rule that fired, the shadow state at that moment and the deployment history. Most stacks cannot answer any of these."
  }
];

const philosophy = [
  { name: "Simple interfaces", desc: "Predictable APIs, minimal cognitive load. Operators should not need a manual." },
  { name: "Reliable systems", desc: "Bounded retries, explicit state, observable failures. Defaults that do not silently drop." },
  { name: "Operational visibility", desc: "Metrics, traces, logs and shadow state on every action. No black boxes." }
];

export function AboutPage() {
  return (
    <>
      <section className="section">
        <div className="container">
          <p className="t-eyebrow">About</p>
          <h1 className="t-h1" style={{ maxWidth: 880, marginBottom: 32 }}>
            Infrastructure for connected devices.
          </h1>
          <p className="t-body-lg" style={{ maxWidth: 720 }}>
            ARGUS is an IoT fleet orchestration platform focused on device lifecycle
            management, telemetry processing, OTA delivery and fleet-scale operations.
          </p>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Motivation</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Why ARGUS exists.
          </h2>
          <div className="dl-list">
            {problems.map((p, idx) => (
              <div className="dl-row" key={p.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name">{p.name}</div>
                </div>
                <div className="dl-desc">{p.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="section">
        <div className="container">
          <p className="t-eyebrow">Philosophy</p>
          <h2 className="t-h1" style={{ maxWidth: 720, marginBottom: 96 }}>
            Design philosophy.
          </h2>
          <div className="dl-list">
            {philosophy.map((p, idx) => (
              <div className="dl-row" key={p.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name">{p.name}</div>
                </div>
                <div className="dl-desc">{p.desc}</div>
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
