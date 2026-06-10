import { Link } from "react-router-dom";
import heroNetwork from "../assets/images/hero-network.webp";

/* ── System flow stages (left → right) ── */
const flowStages: { label: string; sub: string; icon: "node" | "grid" | "stack" | "stream" | "rules" | "store" | "dash" }[] = [
  { label: "Edge Nodes",   sub: "Collect & send",     icon: "node"   },
  { label: "MQTT Broker",  sub: "Ingest & route",     icon: "grid"   },
  { label: "Gateway",      sub: "Validate &",         icon: "stack"  },
  { label: "Redpanda",     sub: "Stream & persist",   icon: "stream" },
  { label: "Rules Engine", sub: "Evaluate &",         icon: "rules"  },
  { label: "Storage",      sub: "Store & index",      icon: "store"  },
  { label: "Dashboard",    sub: "Visualize & act",    icon: "dash"   }
];

export function LandingPage() {
  return (
    <>
      <section className="hero">
        <div className="hero-content">
          <h1 className="lp-hero-heading">
            <span className="lp-hero-line">The platform for</span>
            <span className="lp-hero-line lp-sketch">IoT Fleet</span>
            <span className="lp-hero-line">orchestration.</span>
          </h1>
          <p className="lp-hero-sub">
            Device management, telemetry ingestion, OTA delivery,
            rule evaluation and fleet operations at infrastructure scale.
          </p>
          <div className="lp-hero-cta">
            <Link className="lp-btn" to="/dashboard">Open Platform</Link>
            <a className="lp-btn" href="#system-flow">Read Architecture</a>
          </div>
        </div>
        <div className="hero-canvas-container" aria-hidden>
          <img className="hero-image" src={heroNetwork} alt="" />
        </div>
      </section>

      <section className="lp-system-flow" id="system-flow">
        <div className="lp-container">
          <p className="lp-eyebrow">SYSTEM FLOW</p>
          <h2 className="lp-h1">How data moves.</h2>
          <p className="lp-sub">
            Every event follows a single, observable path from edge to storage.
          </p>
          <div className="lp-flow-wrap">
            <SystemFlow />
          </div>
        </div>
      </section>

      <section className="lp-section">
        <div className="lp-container">
          <p className="lp-eyebrow">PILLARS</p>
          <h2 className="lp-h1">Built for unreliable networks.</h2>
          <p className="lp-sub lp-sub-wide">
            ARGUS is designed around intermittent connectivity, event-driven processing
            and fleet-scale device management.
          </p>
          <div className="lp-cols-3">
            <div>
              <h3>Device State</h3>
              <p>Desired vs reported state synchronization across heterogeneous firmware.</p>
            </div>
            <div>
              <h3>OTA Delivery</h3>
              <p>Versioned firmware rollout with chunked uploads, SHA-256 verification and ACK tracking.</p>
            </div>
            <div>
              <h3>Event Processing</h3>
              <p>Stream telemetry through an event bus with rule evaluation at infrastructure scale.</p>
            </div>
          </div>
        </div>
      </section>

      <section className="lp-section">
        <div className="lp-container">
          <p className="lp-eyebrow">CAPABILITIES</p>
          <h2 className="lp-h1">What ARGUS does.</h2>
          <div className="lp-dl">
            <div className="lp-dl-row">
              <div>
                <span className="lp-dl-num">01</span>
                <div className="lp-dl-name">Device Registry</div>
              </div>
              <div className="lp-dl-desc">
                Central source of truth for every device, firmware version, capability and health state.
              </div>
            </div>
            <div className="lp-dl-row">
              <div>
                <span className="lp-dl-num">02</span>
                <div className="lp-dl-name">Telemetry Pipeline</div>
              </div>
              <div className="lp-dl-desc">
                High-throughput ingestion through MQTT and HTTP gateways, routed to durable event streams.
              </div>
            </div>
            <div className="lp-dl-row">
              <div>
                <span className="lp-dl-num">03</span>
                <div className="lp-dl-name">Digital Twin</div>
              </div>
              <div className="lp-dl-desc">
                Desired and reported state reconciliation. Tracks drift, version history and convergence.
              </div>
            </div>
            <div className="lp-dl-row">
              <div>
                <span className="lp-dl-num">04</span>
                <div className="lp-dl-name">Command &amp; Control</div>
              </div>
              <div className="lp-dl-desc">
                Reliable command dispatch with ACK/NACK tracking, idempotency and bounded retry.
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="lp-section">
        <div className="lp-container">
          <p className="lp-eyebrow">PRINCIPLES</p>
          <h2 className="lp-h1">Engineering principles.</h2>
          <div className="lp-principles">
            {[
              { n: "01", t: "Event Driven",        d: "Every state change is a published event. No silent updates, no missed transitions." },
              { n: "02", t: "Failure Aware",       d: "Networks fail. Buffers, retries and shadow state are first-class, not afterthoughts." },
              { n: "03", t: "State First",         d: "Desired and reported state are explicit. The system converges, it does not assume." },
              { n: "04", t: "Horizontal Scale",    d: "Workers, partitions and event topics scale independently. No monolithic bottlenecks." },
              { n: "05", t: "Operational Simplicity", d: "Deploy, observe, recover. A small surface area that does not require a dedicated team." },
              { n: "06", t: "Infrastructure Agnostic", d: "Runs on bare metal, VMs or containers. Cloud, on-prem and edge are all supported." }
            ].map((p) => (
              <div className="lp-principle" key={p.t}>
                <span className="lp-principle-num">{p.n}</span>
                <h3>{p.t}</h3>
                <p>{p.d}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section className="lp-section">
        <div className="lp-container">
          <p className="lp-eyebrow">TECHNOLOGY</p>
          <h2 className="lp-h1">Built with.</h2>
          <div className="lp-tech">
            <span className="lp-tech-prompt">$</span>
            <span className="lp-tech-name">go</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">postgresql</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">redis</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">minio</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">redpanda</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">mqtt</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">nats</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">prometheus</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">grafana</span>
            <span className="lp-tech-sep">·</span>
            <span className="lp-tech-name">loki</span>
          </div>
        </div>
      </section>

      <section className="lp-final-cta">
        <div className="lp-container">
          <h2 className="lp-final-cta-h">
            Operate fleets.
            <br />
            Not spreadsheets.
          </h2>
          <p className="lp-final-cta-sub">
            Manage devices, telemetry, firmware and automation
            from a single control plane.
          </p>
          <div className="lp-hero-cta" style={{ justifyContent: "center" }}>
            <Link className="lp-btn" to="/dashboard">Open Platform</Link>
            <Link className="lp-btn" to="/documentation">Documentation</Link>
          </div>
        </div>
      </section>

      <footer className="lp-footer">
        <div className="lp-container">
          <div className="lp-footer-inner">
            <span>ARGUS</span>
            <span>IoT Fleet Orchestration Platform</span>
            <span>v2.0</span>
          </div>
        </div>
      </footer>
    </>
  );
}

/* ────────────────────────────────────────────────────────────────────────
   System flow — horizontal architectural timeline
   ──────────────────────────────────────────────────────────────────────── */
function SystemFlow() {
  return (
    <div className="lp-flow-cards">
      {flowStages.map((s, i) => (
        <div className="lp-flow-card" key={s.label}>
          <span className="lp-flow-card-num">{String(i + 1).padStart(2, '0')}</span>
          <span className="lp-flow-card-label">{s.label}</span>
          <span className="lp-flow-card-sub">{s.sub}.</span>
        </div>
      ))}
    </div>
  );
}
