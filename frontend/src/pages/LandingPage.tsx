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
   System flow — 7 stages joined by a glowing wavy river path
   ──────────────────────────────────────────────────────────────────────── */
function SystemFlow() {
  const stages = flowStages;
  const w = 1200;
  const h = 220;
  const padX = 40;
  const cy = 110;                            // vertical centerline of the path
  const colW = (w - padX * 2) / stages.length;
  const hex = 22;                            // hex half-size

  /* Build a wavy cubic-bezier path through every stage */
  const pathD = (() => {
    const pts = stages.map((_, i) => ({
      x: padX + colW * i + colW / 2,
      y: cy
    }));
    let d = `M ${pts[0].x},${pts[0].y}`;
    for (let i = 0; i < pts.length - 1; i++) {
      const a = pts[i];
      const b = pts[i + 1];
      const dx = b.x - a.x;
      const wave = 38;
      const c1x = a.x + dx * 0.35;
      const c1y = a.y - wave;
      const c2x = b.x - dx * 0.35;
      const c2y = b.y + wave;
      d += ` C ${c1x},${c1y} ${c2x},${c2y} ${b.x},${b.y}`;
    }
    return d;
  })();

  return (
    <div className="lp-flow">
      <svg
        className="lp-flow-svg"
        viewBox={`0 0 ${w} ${h}`}
        preserveAspectRatio="none"
        role="img"
        aria-label="System data flow"
      >
        <defs>
          <filter id="lp-flow-bloom" x="-10%" y="-50%" width="120%" height="200%">
            <feGaussianBlur stdDeviation="3" result="b1" />
            <feMerge>
              <feMergeNode in="b1" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
          <linearGradient id="lp-flow-grad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%"   stopColor="rgba(255,255,255,0.15)" />
            <stop offset="50%"  stopColor="rgba(255,255,255,0.55)" />
            <stop offset="100%" stopColor="rgba(255,255,255,0.15)" />
          </linearGradient>
        </defs>

        {/* Outer glow path */}
        <path
          d={pathD}
          fill="none"
          stroke="rgba(255,255,255,0.12)"
          strokeWidth="6"
          strokeLinecap="round"
          filter="url(#lp-flow-bloom)"
        />
        {/* Mid line */}
        <path
          d={pathD}
          fill="none"
          stroke="rgba(255,255,255,0.35)"
          strokeWidth="1.2"
          strokeLinecap="round"
        />
        {/* Highlight crest */}
        <path
          d={pathD}
          fill="none"
          stroke="url(#lp-flow-grad)"
          strokeWidth="0.8"
          strokeLinecap="round"
        />
      </svg>

      <div className="lp-flow-stages">
        {stages.map((s, i) => {
          const leftPct = ((padX + colW * i + colW / 2) / w) * 100;
          return (
            <div className="lp-flow-stage" key={s.label} style={{ left: `${leftPct}%` }}>
              <div className="lp-flow-icon" style={{ width: hex * 2, height: hex * 2 }}>
                <FlowIcon kind={s.icon} />
              </div>
              <div className="lp-flow-label">{s.label}</div>
              <div className="lp-flow-sub">{s.sub}</div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/* ── Inline glyphs for the 7 stages (monochrome, ~24px) ── */
function FlowIcon({ kind }: { kind: "node" | "grid" | "stack" | "stream" | "rules" | "store" | "dash" }) {
  const common = {
    width: 22,
    height: 22,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.4,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true
  };
  switch (kind) {
    case "node":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="2.2" />
          <circle cx="5"  cy="6"  r="1.4" />
          <circle cx="19" cy="6"  r="1.4" />
          <circle cx="5"  cy="18" r="1.4" />
          <circle cx="19" cy="18" r="1.4" />
          <path d="M6 6.8 L10.2 10.6 M18 6.8 L13.8 10.6 M6 17.2 L10.2 13.4 M18 17.2 L13.8 13.4" />
        </svg>
      );
    case "grid":
      return (
        <svg {...common}>
          <circle cx="6"  cy="6"  r="1" />
          <circle cx="12" cy="6"  r="1" />
          <circle cx="18" cy="6"  r="1" />
          <circle cx="6"  cy="12" r="1" />
          <circle cx="12" cy="12" r="1" />
          <circle cx="18" cy="12" r="1" />
          <circle cx="6"  cy="18" r="1" />
          <circle cx="12" cy="18" r="1" />
          <circle cx="18" cy="18" r="1" />
        </svg>
      );
    case "stack":
      return (
        <svg {...common}>
          <path d="M4 7 L12 4 L20 7 L12 10 Z" />
          <path d="M4 12 L12 9 L20 12 L12 15 Z" />
          <path d="M4 17 L12 14 L20 17 L12 20 Z" />
        </svg>
      );
    case "stream":
      return (
        <svg {...common}>
          <path d="M3 8 C 7 4, 11 12, 15 8 S 21 12, 21 8" />
          <path d="M3 13 C 7 9, 11 17, 15 13 S 21 17, 21 13" />
          <path d="M3 18 C 7 14, 11 22, 15 18 S 21 22, 21 18" />
        </svg>
      );
    case "rules":
      return (
        <svg {...common}>
          <path d="M4 6 H20" />
          <path d="M4 12 H14" />
          <path d="M4 18 H18" />
          <circle cx="18" cy="12" r="1.4" fill="currentColor" />
        </svg>
      );
    case "store":
      return (
        <svg {...common}>
          <ellipse cx="12" cy="6" rx="7" ry="2.2" />
          <path d="M5 6 V14 C 5 15.2 8 16 12 16 S 19 15.2 19 14 V6" />
          <path d="M5 11 C 5 12.2 8 13 12 13 S 19 12.2 19 11" />
        </svg>
      );
    case "dash":
      return (
        <svg {...common}>
          <rect x="3" y="4" width="18" height="14" rx="0" />
          <path d="M3 9 H21" />
          <path d="M7 14 H11" />
          <path d="M7 16 H13" />
        </svg>
      );
  }
}
