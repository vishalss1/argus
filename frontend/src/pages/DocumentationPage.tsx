import { useEffect, useState } from "react";

const sections = [
  {
    id: "introduction",
    label: "Introduction",
    body: (
      <>
        <p>
          ARGUS is an IoT fleet orchestration platform focused on device lifecycle
          management, telemetry processing, OTA delivery and fleet-scale operations.
        </p>
        <p>
          The platform is designed for engineering teams that need to operate fleets
          of connected devices across unreliable networks without sacrificing visibility
          or control.
        </p>
      </>
    )
  },
  {
    id: "device-registry",
    label: "Device Registry",
    body: (
      <>
        <p>
          The device registry is the inventory of every connected device. Each device
          carries identity, metadata, firmware version, capability declarations and a
          last-seen timestamp updated by heartbeat.
        </p>
        <ul>
          <li>Register devices with custom hardware and capability metadata.</li>
          <li>Track online and offline state via heartbeat ingestion.</li>
          <li>Query by tag, region, firmware version or capability.</li>
          <li>Decommission devices cleanly with state retention.</li>
        </ul>
      </>
    )
  },
  {
    id: "telemetry",
    label: "Telemetry Pipeline",
    body: (
      <>
        <p>
          Telemetry is ingested over MQTT and HTTP. Ingested packets are validated,
          tagged with device identity, then routed through an event bus to rules,
          storage and the operator dashboard.
        </p>
        <p>
          The pipeline is partitioned by tenant and device, so processing scales
          horizontally with the fleet.
        </p>
      </>
    )
  },
  {
    id: "digital-twin",
    label: "Digital Twin",
    body: (
      <>
        <p>
          Every device carries an explicit digital twin: a desired state and a reported
          state. The orchestration layer continuously reconciles them.
        </p>
        <ul>
          <li>Desired state: target configuration set by an operator.</li>
          <li>Reported state: actual configuration reported by the device.</li>
          <li>Drift: the difference, surfaced as an explicit condition.</li>
        </ul>
      </>
    )
  },
  {
    id: "rules-engine",
    label: "Rule Engine",
    body: (
      <>
        <p>
          Rules evaluate conditions over the telemetry stream. When a condition
          matches, the engine fires an alert or triggers a downstream automation.
        </p>
        <p>
          Rules are expressed in a small operator DSL. They run at the edge of the
          stream, so evaluation is bounded by partition lag, never by database load.
        </p>
      </>
    )
  },
  {
    id: "ota",
    label: "Firmware Lifecycle",
    body: (
      <>
        <p>
          Firmware is uploaded as a binary with a SHA-256 checksum. A rollout is
          declared as a deployment manifest targeting devices, regions or tags.
        </p>
        <ul>
          <li>Chunked upload with progress tracking.</li>
          <li>Staged rollouts with automatic pause on failure.</li>
          <li>Per-device ACK and NACK with retry policy.</li>
          <li>Rollback to a previous known-good version at any time.</li>
        </ul>
      </>
    )
  },
  {
    id: "commands",
    label: "Commands",
    body: (
      <>
        <p>
          Commands are typed payloads sent to a specific device. Each command has a
          bounded retry policy, an idempotency key and a delivery deadline.
        </p>
        <p>
          The platform tracks ACK and NACK responses and surfaces stalled commands
          as operator alerts.
        </p>
      </>
    )
  },
  {
    id: "alerts",
    label: "Alerts",
    body: (
      <>
        <p>
          Alerts are first-class events. They carry severity, source rule, affected
          device and a deduplication key.
        </p>
        <p>
          The operator dashboard surfaces active alerts and a historical timeline
          for post-incident review.
        </p>
      </>
    )
  },
  {
    id: "system-architecture",
    label: "System Architecture",
    body: (
      <>
        <p>
          ARGUS is a modular monolith with a Go core. The core domain is decoupled
          from infrastructure adapters: MQTT, HTTP, PostgreSQL, Redis, MinIO and
          Redpanda.
        </p>
        <p>
          See the <a href="/architecture" className="t-link">architecture page</a> for
          the full system design.
        </p>
      </>
    )
  },
  {
    id: "deployment",
    label: "Deployment",
    body: (
      <>
        <p>
          ARGUS ships as a single Go binary with a docker-compose stack for local
          development and a Kubernetes-ready deployment for production.
        </p>
        <ul>
          <li>Single binary, no external runtime dependencies.</li>
          <li>Compose stack for development with Postgres, Redis, MinIO and Redpanda.</li>
          <li>Helm chart for production deployment on any Kubernetes cluster.</li>
        </ul>
      </>
    )
  }
];

export function DocumentationPage() {
  const [active, setActive] = useState(sections[0].id);
  const [query, setQuery] = useState("");

  useEffect(() => {
    const onScroll = () => {
      const offsets = sections.map((s) => {
        const el = document.getElementById(s.id);
        return el ? { id: s.id, top: el.getBoundingClientRect().top } : null;
      }).filter(Boolean) as { id: string; top: number }[];
      const current = offsets.find((o) => o.top > 80) ?? offsets[offsets.length - 1];
      if (current) setActive(current.id);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  const filtered = sections.filter((s) =>
    s.label.toLowerCase().includes(query.toLowerCase())
  );

  return (
    <section style={{ paddingTop: 64 }}>
      <div className="docs-shell">
        <aside className="docs-sidebar">
          <input
            className="docs-search"
            type="text"
            placeholder="Search docs"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          {filtered.map((s) => (
            <a
              key={s.id}
              href={`#${s.id}`}
              className={active === s.id ? "active" : ""}
            >
              {s.label}
            </a>
          ))}
        </aside>

        <main className="docs-content">
          <div className="docs-page">
            {sections.map((s) => (
              <section key={s.id} id={s.id}>
                <h2>{s.label}</h2>
                {s.body}
              </section>
            ))}
          </div>
        </main>

        <aside className="docs-toc">
          <p className="t-eyebrow">On this page</p>
          {sections.map((s) => (
            <a
              key={s.id}
              href={`#${s.id}`}
              className={active === s.id ? "active" : ""}
            >
              {s.label}
            </a>
          ))}
        </aside>
      </div>
    </section>
  );
}
