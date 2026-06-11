import { Link } from "react-router-dom";
import heroNetwork from "../assets/images/hero-network.webp";

/* ΓöÇΓöÇ System flow stages (left ΓåÆ right) ΓöÇΓöÇ */
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
            <Link className="lp-btn" to="/dashboard">Open Dashboard</Link>
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
            <Link className="lp-btn" to="/dashboard">Open Dashboard</Link>
            <Link className="lp-btn" to="/about">Learn More</Link>
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

/* ΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇ
   System flow ΓÇö horizontal architectural timeline
   ΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇΓöÇ */
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
