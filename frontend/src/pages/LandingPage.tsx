import { useEffect, useRef } from "react";
import { Link } from "react-router-dom";

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
          <ParticleCanvas />
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

/* Hand-plotted node positions (x, y as fractions of container).
   Network sits top-right, dense center mesh, sparse outer ring. */
const RAW: [number, number][] = [
  // Sparse top outliers
  [0.42, 0.06], [0.52, 0.04], [0.63, 0.08], [0.74, 0.05], [0.84, 0.09], [0.94, 0.12],
  // Upper band
  [0.38, 0.14], [0.48, 0.12], [0.57, 0.16], [0.67, 0.13], [0.77, 0.17], [0.87, 0.15], [0.96, 0.20],
  // Upper-mid band — network starts getting dense
  [0.35, 0.22], [0.44, 0.20], [0.53, 0.24], [0.62, 0.21], [0.71, 0.25], [0.80, 0.22], [0.89, 0.26], [0.97, 0.30],
  // Core dense band 1
  [0.37, 0.30], [0.46, 0.28], [0.55, 0.32], [0.64, 0.29], [0.73, 0.33], [0.82, 0.30], [0.91, 0.34], [0.98, 0.38],
  // Core dense band 2 — densest region
  [0.39, 0.38], [0.48, 0.36], [0.57, 0.40], [0.66, 0.37], [0.75, 0.41], [0.84, 0.38], [0.93, 0.42],
  // Core dense band 3
  [0.41, 0.46], [0.50, 0.44], [0.59, 0.48], [0.68, 0.45], [0.77, 0.49], [0.86, 0.46], [0.95, 0.50],
  // Lower-mid — network thins out
  [0.44, 0.54], [0.53, 0.52], [0.62, 0.56], [0.71, 0.53], [0.80, 0.57], [0.89, 0.54],
  // Lower band
  [0.47, 0.62], [0.56, 0.60], [0.65, 0.64], [0.74, 0.61], [0.83, 0.65], [0.92, 0.62],
  // Bottom sparse trailing
  [0.51, 0.70], [0.60, 0.68], [0.69, 0.72], [0.78, 0.69], [0.87, 0.73],
  [0.55, 0.78], [0.64, 0.76], [0.73, 0.80],
];

const HERO_INDICES = new Set([5, 12, 20, 27, 33, 39, 44, 50]);

function ParticleCanvas() {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  useEffect(() => {
  const canvas = canvasRef.current;
  if (!canvas) return;

  const draw = () => {
    const w = canvas.offsetWidth;
    const h = canvas.offsetHeight;
    if (w === 0 || h === 0) return;

    const dpr = window.devicePixelRatio || 1;
    canvas.width = w * dpr;
    canvas.height = h * dpr;
    const ctx = canvas.getContext('2d')!;
    ctx.scale(dpr, dpr);

    ctx.fillStyle = '#080808';
    ctx.fillRect(0, 0, w, h);

    // Organic scatter — NOT a grid
    // x range: 0.05 to 0.93 (full width)
    // y range: 0.04 to 0.88 (full height)
    const P: [number,number,number][] = [
      // Far-left outliers bleeding toward text
      [0.05,0.42,0.2],[0.10,0.28,0.3],[0.12,0.55,0.2],[0.08,0.38,0.15],

      // Left edge of network
      [0.18,0.20,0.4],[0.16,0.35,0.4],[0.20,0.50,0.4],[0.22,0.65,0.3],
      [0.25,0.14,0.5],[0.28,0.58,0.4],[0.15,0.72,0.25],

      // Left-center
      [0.32,0.22,0.6],[0.30,0.38,0.6],[0.34,0.52,0.6],[0.31,0.68,0.5],
      [0.38,0.12,0.5],[0.36,0.44,0.7],[0.33,0.76,0.4],

      // Center-left — network thickens
      [0.42,0.18,0.7],[0.44,0.30,0.8],[0.40,0.42,0.8],[0.43,0.55,0.7],
      [0.46,0.65,0.6],[0.41,0.78,0.4],[0.45,0.08,0.5],

      // True center — densest
      [0.50,0.14,0.8],[0.52,0.24,0.9],[0.49,0.34,1.0],[0.53,0.44,1.0],
      [0.50,0.54,0.9],[0.52,0.64,0.8],[0.49,0.74,0.6],[0.51,0.82,0.4],

      // Center-right — still dense
      [0.58,0.10,0.7],[0.56,0.20,0.9],[0.60,0.30,1.0],[0.57,0.40,1.0],
      [0.61,0.50,0.9],[0.58,0.60,0.8],[0.60,0.70,0.7],[0.57,0.80,0.5],

      // Right-center
      [0.66,0.16,0.7],[0.64,0.26,0.8],[0.68,0.36,0.9],[0.65,0.46,0.9],
      [0.69,0.56,0.8],[0.66,0.66,0.7],[0.68,0.76,0.5],

      // Right
      [0.74,0.12,0.6],[0.72,0.22,0.7],[0.76,0.32,0.8],[0.73,0.42,0.8],
      [0.77,0.52,0.7],[0.74,0.62,0.6],[0.76,0.72,0.4],

      // Right edge
      [0.82,0.18,0.6],[0.80,0.30,0.7],[0.84,0.40,0.7],[0.81,0.50,0.6],
      [0.85,0.60,0.5],[0.82,0.70,0.4],

      // Far right sparse outliers
      [0.88,0.14,0.5],[0.90,0.26,0.5],[0.92,0.38,0.5],[0.89,0.50,0.4],
      [0.93,0.62,0.4],[0.91,0.74,0.3],

      // Top sparse
      [0.35,0.06,0.3],[0.55,0.04,0.4],[0.72,0.06,0.3],[0.88,0.06,0.3],

      // Bottom sparse
      [0.40,0.88,0.2],[0.60,0.86,0.3],[0.78,0.84,0.2],
    ];

    // Hero nodes — large bloom anchors, scattered organically
    const HEROES = new Set([28, 29, 35, 36, 42, 43, 49, 56]);

// Deterministic "random" using index so it's stable every render
const BIG_NODES = new Set([4,8,12,16,20,24,28,32,36,40,44,48,52,56,60,3,9,15,21,62]);
const nodes = P.map(([px, py], i) => {
  const depth = Math.max(0.15, 1.0 - (px * 0.65));
  // Front nodes (low x = high depth) get bigger, plus a few background ones
  const isBig = BIG_NODES.has(i);
  return { x: px * w, y: py * h, depth, isBig };
});

    // Scale network up from its centroid
    const cx = nodes.reduce((s,n) => s + n.x, 0) / nodes.length;
    const cy = nodes.reduce((s,n) => s + n.y, 0) / nodes.length;
    nodes.forEach(n => {
      n.x = cx + (n.x - cx) * 1.08;
      n.y = cy + (n.y - cy) * 1.32; // stretch vertically
    });

    // Clamp nodes to stay within canvas with 4% padding
    nodes.forEach(n => {
      n.x = Math.max(w * 0.02, Math.min(w * 0.98, n.x));
      n.y = Math.max(h * 0.02, Math.min(h * 0.98, n.y));
    });

    const T = w * 0.19; // connection threshold

    // Draw ambient glow inside triangle faces
    // Find triangles by checking triplets of close nodes
    ctx.shadowBlur = 0;
    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const dij = Math.hypot(nodes[i].x-nodes[j].x, nodes[i].y-nodes[j].y);
        if (dij > T * 0.55) continue;
        for (let k = j + 1; k < nodes.length; k++) {
          const dik = Math.hypot(nodes[i].x-nodes[k].x, nodes[i].y-nodes[k].y);
          const djk = Math.hypot(nodes[j].x-nodes[k].x, nodes[j].y-nodes[k].y);
          if (dik > T * 0.55 || djk > T * 0.55) continue;
          // Valid triangle — fill with ambient light scaled by avg depth
          const avgDepth = (nodes[i].depth + nodes[j].depth + nodes[k].depth) / 3;
          const opacity = avgDepth * 0.004;
          ctx.beginPath();
          ctx.moveTo(nodes[i].x, nodes[i].y);
          ctx.lineTo(nodes[j].x, nodes[j].y);
          ctx.lineTo(nodes[k].x, nodes[k].y);
          ctx.closePath();
          ctx.fillStyle = `rgba(255,255,255,${opacity})`;
          ctx.fill();
        }
      }
    }

    // DRAW EDGES — fire/torch non-uniform glow effect
    ctx.shadowBlur = 0;
    for (let i = 0; i < nodes.length; i++) {
      const a = nodes[i];
      const nbrs: {j:number,d:number}[] = [];
      for (let j = i + 1; j < nodes.length; j++) {
        const b = nodes[j];
        const d = Math.hypot(a.x - b.x, a.y - b.y);
        if (d < T) nbrs.push({j, d});
      }
      nbrs.sort((x, y) => x.d - y.d);

      // Organic disconnection — outer/background nodes randomly have fewer connections
      const maxConn = a.isBig ? 6 :
                      a.depth > 0.65 ? 4 :
                      a.depth > 0.40 ? 3 :
                      (i % 2 === 0) ? 2 : 1;

      // Randomly skip some edges entirely for outer nodes — creates isolated segments
      const candidates = nbrs.filter((_, idx) => {
        // Force disconnection by index — creates organic isolated clusters
        if (a.isBig) return true;
        if (a.depth > 0.8) return ((i + idx) % 7) !== 0; // skip ~14%
        if (a.depth > 0.55) return ((i * 3 + idx * 7) % 5) !== 0; // skip ~20%
        if (a.depth > 0.35) return ((i * 11 + idx * 5) % 3) !== 0; // skip ~33%
        return ((i + idx) % 2) === 0; // outer nodes: skip HALF their edges
      });

      for (const {j, d} of candidates.slice(0, maxConn)) {
        const b = nodes[j];
        const avgDepth = (a.depth + b.depth) / 2;
        const distFactor = 1 - d / T;

        // Base faint edge
        const grad = ctx.createLinearGradient(a.x, a.y, b.x, b.y);
        grad.addColorStop(0, `rgba(255,255,255,${Math.min(distFactor * a.depth * 0.30, 0.35)})`);
        grad.addColorStop(1, `rgba(255,255,255,${Math.min(distFactor * b.depth * 0.30, 0.35)})`);
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.strokeStyle = grad;
        ctx.lineWidth = 0.4 + avgDepth * 0.4;
        ctx.shadowBlur = 0;
        ctx.stroke();

        // Fire glow — only on edges connected to big nodes or high-depth
        const shouldGlow = (a.isBig || b.isBig) || avgDepth > 0.72;
        if (shouldGlow) {
          // Draw 3 segments along the edge with varying glow intensity
          // simulates non-uniform torch light — brightest near the big node
          const steps = 5;
          for (let s = 0; s < steps; s++) {
            const t0 = s / steps;
            const t1 = (s + 1) / steps;
            const x0 = a.x + (b.x - a.x) * t0;
            const y0 = a.y + (b.y - a.y) * t0;
            const x1 = a.x + (b.x - a.x) * t1;
            const y1 = a.y + (b.y - a.y) * t1;

            // Intensity peaks near whichever endpoint is bigger/brighter
            const tMid = (t0 + t1) / 2;
            const nearA = 1 - tMid;
            const nearB = tMid;
            const intensityA = a.isBig ? nearA * 0.9 : nearA * a.depth * 0.5;
            const intensityB = b.isBig ? nearB * 0.9 : nearB * b.depth * 0.5;
            const segIntensity = Math.max(intensityA, intensityB) * distFactor;

            ctx.beginPath();
            ctx.moveTo(x0, y0);
            ctx.lineTo(x1, y1);
            ctx.strokeStyle = `rgba(255,255,255,${Math.min(segIntensity * 0.6, 0.55)})`;
            ctx.lineWidth = 0.5 + segIntensity * 1.5;
            ctx.shadowBlur = segIntensity * 18;
            ctx.shadowColor = `rgba(255,255,255,${segIntensity * 0.8})`;
            ctx.stroke();
            ctx.shadowBlur = 0;
          }
        }
      }
    }

    // DRAW REGULAR NODES — size and glow scaled by depth
    for (let i = 0; i < nodes.length; i++) {
      if (HEROES.has(i)) continue;
      const {x, y, depth, isBig} = nodes[i];

      if (isBig) {
        // NO filled circles at large radius — that creates the ring look
        // Instead: draw only the core, let shadowBlur do ALL the work

        // Pass 1 — extreme outer bloom (pure shadow, invisible fill)
        ctx.beginPath();
        ctx.arc(x, y, 1.5, 0, Math.PI * 2);
        ctx.shadowBlur = 80 + depth * 40;
        ctx.shadowColor = `rgba(255,255,255,${0.25 + depth * 0.2})`;
        ctx.fillStyle = 'rgba(255,255,255,0)'; // transparent — only shadow renders
        ctx.fill();

        // Pass 2 — mid bloom
        ctx.beginPath();
        ctx.arc(x, y, 1.5, 0, Math.PI * 2);
        ctx.shadowBlur = 35 + depth * 20;
        ctx.shadowColor = `rgba(255,255,255,${0.5 + depth * 0.3})`;
        ctx.fillStyle = 'rgba(255,255,255,0)';
        ctx.fill();

        // Pass 3 — tight inner glow
        ctx.beginPath();
        ctx.arc(x, y, 2.0 + depth * 0.8, 0, Math.PI * 2);
        ctx.shadowBlur = 16 + depth * 10;
        ctx.shadowColor = '#ffffff';
        ctx.fillStyle = `rgba(255,255,255,${0.6 + depth * 0.4})`;
        ctx.fill();

        // Pass 4 — pinpoint hot core
        ctx.beginPath();
        ctx.arc(x, y, 1.2, 0, Math.PI * 2);
        ctx.shadowBlur = 6;
        ctx.shadowColor = '#ffffff';
        ctx.fillStyle = '#ffffff';
        ctx.fill();

        ctx.shadowBlur = 0;
      } else {
        // Normal node
        const r = 1.2 + depth * 1.6;
        ctx.beginPath();
        ctx.arc(x, y, r, 0, Math.PI * 2);
        ctx.shadowBlur = 10 + depth * 20;
        ctx.shadowColor = `rgba(255,255,255,${0.4 + depth * 0.6})`;
        ctx.fillStyle = '#ffffff';
        ctx.fill();
        ctx.shadowBlur = 0;
      }
    }

    // DRAW HERO NODES — 3-layer bloom
    ctx.shadowBlur = 0;
    for (const i of HEROES) {
      if (!nodes[i]) continue;
      const {x, y, depth} = nodes[i];

      // Outer halo
      ctx.beginPath();
      ctx.arc(x, y, 6 + depth * 4, 0, Math.PI * 2);
      ctx.shadowBlur = 25 + depth * 40;
      ctx.shadowColor = `rgba(255,255,255,${0.3 + depth * 0.4})`;
      ctx.fillStyle = 'rgba(255,255,255,0.0)';
      ctx.fill();

      // Mid glow
      ctx.beginPath();
      ctx.arc(x, y, 3.5, 0, Math.PI * 2);
      ctx.shadowBlur = 18 + depth * 20;
      ctx.shadowColor = `rgba(255,255,255,${0.6 + depth * 0.4})`;
      ctx.fillStyle = `rgba(255,255,255,${depth * 0.25})`;
      ctx.fill();

      // Inner glow ring
      ctx.beginPath();
      ctx.arc(x, y, 2.8, 0, Math.PI * 2);
      ctx.shadowBlur = 16;
      ctx.shadowColor = 'rgba(255,255,255,0.95)';
      ctx.fillStyle = 'rgba(255,255,255,0.25)';
      ctx.fill();

      // Bright core
      ctx.beginPath();
      ctx.arc(x, y, 1.8 + depth * 0.7, 0, Math.PI * 2);
      ctx.shadowBlur = 14 + depth * 10;
      ctx.shadowColor = '#ffffff';
      ctx.fillStyle = '#ffffff';
      ctx.fill();
    }
    ctx.shadowBlur = 0;
  };

  draw();
  window.addEventListener('resize', draw);
  return () => window.removeEventListener('resize', draw);
}, []);

  return <canvas ref={canvasRef} className="lp-canvas" />;
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
