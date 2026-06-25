import React, { useState, useEffect, useMemo, useRef } from "react";
import { Link } from "react-router-dom";
import { motion, useMotionValue, useTransform, animate, useInView } from "framer-motion";
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
            <Link className="lp-btn" to="/dashboard">Open Dashboard</Link>
            <a className="lp-btn" href="/docs">Read Docs</a>
          </div>
        </div>
        <div className="hero-canvas-container" aria-hidden>
          <img className="hero-image" src={heroNetwork} alt="" />
        </div>
      </section>

      <ProofSection />

      <div className="story-wrapper" id="fleet-intelligence">
        {/* Section 1: Fleet Intelligence */}
        <StorySection
          label="FLEET INTELLIGENCE"
          title="Turn telemetry into understanding."
          description="Surface patterns, identify anomalies and explore fleet behaviour using AI-powered insights and natural language."
          illustration={<FleetIntelligenceIllustration />}
          reverse={false}
        />

        {/* Section 2: Software Delivery */}
        <StorySection
          label="SOFTWARE DELIVERY"
          title="Ship firmware with confidence."
          description="Deliver secure OTA updates across devices without interrupting operations. Firmware integrity and version management stay automated in the background."
          illustration={<SoftwareDeliveryIllustration />}
          reverse={true}
        />

        {/* Section 3: Live Visibility */}
        <StorySection
          label="LIVE VISIBILITY"
          title="See the fleet in motion."
          description="Monitor telemetry streams and understand what is happening as events unfold."
          illustration={<LiveVisibilityIllustration />}
          reverse={false}
        />

        {/* Section 4: Automation */}
        <StorySection
          label="AUTOMATION"
          title="Let the platform react."
          description="Build rules that automatically respond to changing conditions and reduce manual intervention."
          illustration={<AutomationIllustration />}
          reverse={true}
        />

        {/* Section 5: Operational Memory */}
        <StorySection
          label="OPERATIONAL MEMORY"
          title="Every session leaves context behind."
          description="Historical sessions, artifacts and AI summaries preserve operational knowledge for future analysis."
          illustration={<OperationalMemoryIllustration />}
          reverse={false}
        />

        {/* Section 6: Security */}
        <StorySection
          label="SECURITY"
          title="Built for trusted delivery."
          description="Secure firmware distribution uses TLS transport and SHA-256 verification to ensure devices receive authentic software."
          illustration={<SecurityIllustration />}
          reverse={true}
        />
      </div>

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

/* ── STORYTELLING COMPONENTS ─────────────────────────────────────────── */

interface StorySectionProps {
  label: string;
  title: string;
  description: string;
  illustration: React.ReactNode;
  reverse?: boolean;
}

function StorySection({ label, title, description, illustration, reverse }: StorySectionProps) {
  return (
    <motion.section
      className={`story-section ${reverse ? "reverse" : ""}`}
      initial={{ opacity: 0, y: 60 }}
      whileInView={{ opacity: 1, y: 0 }}
      transition={{ duration: 1.2, ease: [0.22, 1, 0.36, 1] }}
      viewport={{ once: true, amount: 0.3 }}
    >
      <div className="story-text-container">
        <span className="story-label">{label}</span>
        <h2 className="story-title">{title}</h2>
        <p className="story-description">{description}</p>
      </div>
      <div className="story-illustration-container">
        {illustration}
      </div>
    </motion.section>
  );
}

/* ── ABSTRACT ILLUSTRATION COMPONENTS ────────────────────────────────── */

function FleetIntelligenceIllustration() {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    let anim = requestAnimationFrame(function run() {
      setTick((t) => t + 1);
      anim = requestAnimationFrame(run);
    });
    return () => cancelAnimationFrame(anim);
  }, []);

  const x1 = 120 + Math.sin(tick * 0.01) * 15;
  const y1 = 120 + Math.cos(tick * 0.012) * 15;

  const x2 = 280 + Math.cos(tick * 0.008) * 20;
  const y2 = 100 + Math.sin(tick * 0.014) * 15;

  const x3 = 200 + Math.sin(tick * 0.011) * 18;
  const y3 = 240 + Math.cos(tick * 0.009) * 18;

  const x4 = 360 + Math.cos(tick * 0.013) * 15;
  const y4 = 220 + Math.sin(tick * 0.01) * 15;

  const x5 = 150 + Math.sin(tick * 0.007) * 20;
  const y5 = 320 + Math.cos(tick * 0.011) * 15;

  const x6 = 300 + Math.cos(tick * 0.012) * 15;
  const y6 = 340 + Math.sin(tick * 0.008) * 15;

  const p1 = (tick % 150) / 150;
  const px1 = x1 + (x3 - x1) * p1;
  const py1 = y1 + (y3 - y1) * p1;

  const p2 = ((tick + 75) % 150) / 150;
  const px2 = x4 + (x6 - x4) * p2;
  const py2 = y4 + (y6 - y4) * p2;

  return (
    <svg viewBox="0 0 500 500" fill="none">
      <defs>
        <filter id="glow-fi" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="6" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      <line x1={x1} y1={y1} x2={x2} y2={y2} stroke="#ffffff" strokeOpacity={0.12 + Math.sin(tick * 0.02) * 0.04} strokeWidth="1.5" />
      <line x1={x1} y1={y1} x2={x3} y2={y3} stroke="#ffffff" strokeOpacity={0.18 + Math.cos(tick * 0.015) * 0.05} strokeWidth="1.5" />
      <line x1={x2} y1={y2} x2={x3} y2={y3} stroke="#ffffff" strokeOpacity={0.15 + Math.sin(tick * 0.01) * 0.03} strokeWidth="1.5" />
      <line x1={x2} y1={y2} x2={x4} y2={y4} stroke="#ffffff" strokeOpacity={0.13 + Math.cos(tick * 0.025) * 0.04} strokeWidth="1.5" />
      <line x1={x3} y1={y3} x2={x5} y2={y5} stroke="#ffffff" strokeOpacity={0.2 + Math.sin(tick * 0.018) * 0.06} strokeWidth="1.5" />
      <line x1={x3} y1={y3} x2={x6} y2={y6} stroke="#ffffff" strokeOpacity={0.14 + Math.cos(tick * 0.012) * 0.04} strokeWidth="1.5" />
      <line x1={x4} y1={y4} x2={x6} y2={y6} stroke="#ffffff" strokeOpacity={0.17 + Math.sin(tick * 0.022) * 0.05} strokeWidth="1.5" />
      <line x1={x5} y1={y5} x2={x6} y2={y6} stroke="#ffffff" strokeOpacity={0.16 + Math.cos(tick * 0.008) * 0.03} strokeWidth="1.5" />

      <circle cx={px1} cy={py1} r="4" fill="#ffffff" filter="url(#glow-fi)" />
      <circle cx={px2} cy={py2} r="4" fill="#ffffff" filter="url(#glow-fi)" />

      <circle cx={x1} cy={y1} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
      <circle cx={x2} cy={y2} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
      <circle cx={x3} cy={y3} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
      <circle cx={x4} cy={y4} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
      <circle cx={x5} cy={y5} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
      <circle cx={x6} cy={y6} r="6" fill="#000000" stroke="#ffffff" strokeWidth="2" />
    </svg>
  );
}

function SoftwareDeliveryIllustration() {
  const [progresses, setProgresses] = useState([0, 0, 0]);
  const [versions, setVersions] = useState(["v1.2.0", "v1.8.4", "v2.0.1"]);
  const [success, setSuccess] = useState([false, false, false]);

  useEffect(() => {
    const interval = setInterval(() => {
      setProgresses((prev) =>
        prev.map((p, idx) => {
          let step = 0;
          if (idx === 0) step = 0.8;
          if (idx === 1) step = 0.5;
          if (idx === 2) step = 1.2;

          const next = p + step;
          if (next >= 100) {
            if (!success[idx]) {
              setSuccess((s) => {
                const copy = [...s];
                copy[idx] = true;
                return copy;
              });
              setVersions((v) => {
                const copy = [...v];
                if (idx === 0) copy[0] = copy[0] === "v1.2.0" ? "v1.3.0" : "v1.2.0";
                if (idx === 1) copy[1] = copy[1] === "v1.8.4" ? "v1.9.0" : "v1.8.4";
                if (idx === 2) copy[2] = copy[2] === "v2.0.1" ? "v2.1.0" : "v2.0.1";
                return copy;
              });
            }
            if (next >= 130) {
              setSuccess((s) => {
                const copy = [...s];
                copy[idx] = false;
                return copy;
              });
              return 0;
            }
          }
          return next;
        })
      );
    }, 30);
    return () => clearInterval(interval);
  }, [success]);

  return (
    <svg viewBox="0 0 500 500" fill="none">
      <defs>
        <filter id="glow-sd" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="4" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      {[0, 1, 2].map((idx) => {
        const yOffset = 110 + idx * 110;
        const progVal = Math.min(progresses[idx], 100);
        const isSuccess = success[idx] && progresses[idx] >= 100;

        return (
          <g key={idx} transform={`translate(80, ${yOffset})`}>
            <rect x="0" y="0" width="340" height="70" rx="4" stroke="#ffffff" strokeOpacity="0.08" fill="#050505" strokeWidth="1" />
            
            <text x="20" y="30" fill="#ffffff" fillOpacity="0.8" fontFamily="var(--font-mono)" fontSize="11" letterSpacing="0.05em">
              DEVICE_0{idx + 1}
            </text>

            <text x="20" y="48" fill="#ffffff" fillOpacity="0.4" fontFamily="var(--font-mono)" fontSize="11">
              FIRMWARE: {versions[idx]}
            </text>

            <rect x="150" y="22" width="130" height="4" rx="2" fill="#ffffff" fillOpacity="0.05" />
            <rect x="150" y="22" width={1.3 * progVal} height="4" rx="2" fill="#ffffff" fillOpacity="0.6" />

            <text x="150" y="44" fill="#ffffff" fillOpacity="0.4" fontFamily="var(--font-mono)" fontSize="10">
              {Math.round(progVal)}%
            </text>

            {isSuccess && (
              <g transform="translate(300, 24)">
                <circle cx="8" cy="8" r="8" fill="#ffffff" fillOpacity="0.1" />
                <circle cx="8" cy="8" r="4" fill="#ffffff" filter="url(#glow-sd)" />
                <text x="-32" y="12" fill="#ffffff" fillOpacity="0.9" fontFamily="var(--font-mono)" fontSize="9" letterSpacing="0.05em">
                  OK
                </text>
              </g>
            )}
          </g>
        );
      })}
    </svg>
  );
}


function LiveVisibilityIllustration() {
  const [tick, setTick] = useState(0);
  const [metric, setMetric] = useState(128.5);

  useEffect(() => {
    let anim = requestAnimationFrame(function run() {
      setTick((t) => t + 1);
      anim = requestAnimationFrame(run);
    });
    return () => cancelAnimationFrame(anim);
  }, []);

  useEffect(() => {
    const interval = setInterval(() => {
      setMetric((m) => {
        const delta = (Math.random() - 0.5) * 1.5;
        const next = m + delta;
        return Math.max(120, Math.min(140, next));
      });
    }, 400);
    return () => clearInterval(interval);
  }, []);

  const getPath = (offset: number, amplitude: number, speed: number) => {
    const points = [];
    for (let x = 0; x <= 360; x += 10) {
      const y =
        200 +
        Math.sin(x * 0.02 + tick * speed + offset) * amplitude +
        Math.cos(x * 0.007 + tick * 0.005) * 10;
      points.push(`${x + 70},${y}`);
    }
    return `M ${points.join(" L ")}`;
  };

  const path1 = getPath(0, 35, 0.025);
  const path2 = getPath(Math.PI / 2, 20, 0.015);

  const getParticle = (speed: number, offset: number, amplitude: number, index: number) => {
    const progress = (tick * speed + index * 0.3) % 1.0;
    const px = 70 + progress * 360;
    const xVal = px - 70;
    const py =
      200 +
      Math.sin(xVal * 0.02 + tick * speed + offset) * amplitude +
      Math.cos(xVal * 0.007 + tick * 0.005) * 10;
    return { x: px, y: py };
  };

  const particles = [0, 1, 2].map((i) => getParticle(0.012, 0, 35, i));

  return (
    <svg viewBox="0 0 500 500" fill="none">
      <defs>
        <filter id="glow-lv" x="-20%" y="-20%" width="140%" height="140%">
          <feGaussianBlur stdDeviation="5" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      <line x1="70" y1="120" x2="430" y2="120" stroke="#ffffff" strokeOpacity="0.03" strokeWidth="1" />
      <line x1="70" y1="200" x2="430" y2="200" stroke="#ffffff" strokeOpacity="0.03" strokeWidth="1" />
      <line x1="70" y1="280" x2="430" y2="280" stroke="#ffffff" strokeOpacity="0.03" strokeWidth="1" />

      <path d={path1} stroke="#ffffff" strokeOpacity="0.4" strokeWidth="2" />
      <path d={path2} stroke="#ffffff" strokeOpacity="0.15" strokeWidth="1.5" />

      {particles.map((p, idx) => (
        <circle
          key={idx}
          cx={p.x}
          cy={p.y}
          r="3.5"
          fill="#ffffff"
          filter="url(#glow-lv)"
          fillOpacity="0.8"
        />
      ))}

      <g transform="translate(70, 310)">
        <rect x="0" y="0" width="160" height="50" rx="3" fill="#000000" stroke="#ffffff" strokeOpacity="0.08" strokeWidth="1" />
        <text x="15" y="22" fill="#ffffff" fillOpacity="0.4" fontFamily="var(--font-mono)" fontSize="9" letterSpacing="0.08em">
          RX_STREAM_01
        </text>
        <text x="15" y="38" fill="#ffffff" fontFamily="var(--font-mono)" fontSize="13" fontWeight="500">
          {metric.toFixed(1)} Mb/s
        </text>
        <circle cx="140" cy="28" r="3" fill="#ffffff" fillOpacity="0.6" filter="url(#glow-lv)" />
      </g>
    </svg>
  );
}

function AutomationIllustration() {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    let anim = requestAnimationFrame(function run() {
      setTick((t) => t + 1);
      anim = requestAnimationFrame(run);
    });
    return () => cancelAnimationFrame(anim);
  }, []);

  const pulseProg = (tick % 160) / 160;

  let pulseX = 100;
  let pulseY = 250;
  if (pulseProg < 0.5) {
    const segmentProg = pulseProg / 0.5;
    pulseX = 100 + (250 - 100) * segmentProg;
  } else {
    const segmentProg = (pulseProg - 0.5) / 0.5;
    pulseX = 250 + (400 - 250) * segmentProg;
  }

  const isCondActive = pulseProg >= 0.0 && pulseProg < 0.1;
  const isRuleActive = pulseProg >= 0.45 && pulseProg < 0.55;
  const isActActive = pulseProg >= 0.9 && pulseProg < 1.0;

  return (
    <svg viewBox="0 0 500 500" fill="none">
      <defs>
        <filter id="glow-au" x="-30%" y="-30%" width="160%" height="160%">
          <feGaussianBlur stdDeviation="8" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      <line x1="100" y1="250" x2="250" y2="250" stroke="#ffffff" strokeOpacity="0.12" strokeWidth="2" strokeDasharray="4 4" />
      <line x1="250" y1="250" x2="400" y2="250" stroke="#ffffff" strokeOpacity="0.12" strokeWidth="2" strokeDasharray="4 4" />

      {pulseProg < 0.95 && (
        <circle cx={pulseX} cy={pulseY} r="5" fill="#ffffff" filter="url(#glow-au)" />
      )}

      <g transform="translate(100, 250)">
        <path
          d="M 0,-24 L 24,0 L 0,24 L -24,0 Z"
          fill="#000000"
          stroke="#ffffff"
          strokeOpacity={isCondActive ? 0.9 : 0.25}
          strokeWidth="1.5"
          style={{ transition: "stroke-opacity 0.2s" }}
        />
        {isCondActive && <circle cx="0" cy="0" r="12" fill="#ffffff" fillOpacity="0.08" filter="url(#glow-au)" />}
        <text x="0" y="4" fill="#ffffff" fillOpacity={isCondActive ? 1.0 : 0.4} fontFamily="var(--font-mono)" fontSize="10" textAnchor="middle" style={{ transition: "fill-opacity 0.2s" }}>
          IF
        </text>
        <text x="0" y="-36" fill="#ffffff" fillOpacity="0.3" fontFamily="var(--font-mono)" fontSize="9" textAnchor="middle">
          CONDITION
        </text>
      </g>

      <g transform="translate(250, 250)">
        <rect
          x="-28"
          y="-20"
          width="56"
          height="40"
          rx="3"
          fill="#000000"
          stroke="#ffffff"
          strokeOpacity={isRuleActive ? 0.9 : 0.25}
          strokeWidth="1.5"
          style={{ transition: "stroke-opacity 0.2s" }}
        />
        {isRuleActive && <rect x="-28" y="-20" width="56" height="40" rx="3" fill="#ffffff" fillOpacity="0.08" filter="url(#glow-au)" />}
        <text x="0" y="4" fill="#ffffff" fillOpacity={isRuleActive ? 1.0 : 0.4} fontFamily="var(--font-mono)" fontSize="10" textAnchor="middle" style={{ transition: "fill-opacity 0.2s" }}>
          AND
        </text>
        <text x="0" y="-36" fill="#ffffff" fillOpacity="0.3" fontFamily="var(--font-mono)" fontSize="9" textAnchor="middle">
          RULE
        </text>
      </g>

      <g transform="translate(400, 250)">
        <path
          d="M -14,-24 L 14,-24 L 28,0 L 14,24 L -14,24 L -28,0 Z"
          fill="#000000"
          stroke="#ffffff"
          strokeOpacity={isActActive ? 0.9 : 0.25}
          strokeWidth="1.5"
          style={{ transition: "stroke-opacity 0.2s" }}
        />
        {isActActive && <circle cx="0" cy="0" r="14" fill="#ffffff" fillOpacity="0.08" filter="url(#glow-au)" />}
        <text x="0" y="4" fill="#ffffff" fillOpacity={isActActive ? 1.0 : 0.4} fontFamily="var(--font-mono)" fontSize="10" textAnchor="middle" style={{ transition: "fill-opacity 0.2s" }}>
          RUN
        </text>
        <text x="0" y="-36" fill="#ffffff" fillOpacity="0.3" fontFamily="var(--font-mono)" fontSize="9" textAnchor="middle">
          ACTION
        </text>
      </g>
    </svg>
  );
}

function OperationalMemoryIllustration() {
  const [tick, setTick] = useState(0);
  useEffect(() => {
    let anim = requestAnimationFrame(function run() {
      setTick((t) => t + 1);
      anim = requestAnimationFrame(run);
    });
    return () => cancelAnimationFrame(anim);
  }, []);

  const lineYStart = 80;
  const lineYEnd = 420;

  const points = [
    { y: 120, time: "10:42:15", title: "CRITICAL ALERT RESOLVED", state: "OK" },
    { y: 220, time: "09:15:00", title: "OTA DEPLOYMENT SUCCESSFUL", state: "SUCCESS" },
    { y: 320, time: "08:00:32", title: "AI SUMMARY GENERATED", state: "SUMMARY" }
  ];

  const indicatorY = 120 + ((Math.sin(tick * 0.01) + 1) / 2) * 200;

  return (
    <svg viewBox="0 0 500 500" fill="none">
      <defs>
        <filter id="glow-om" x="-25%" y="-25%" width="150%" height="150%">
          <feGaussianBlur stdDeviation="6" result="blur" />
          <feComposite in="SourceGraphic" in2="blur" operator="over" />
        </filter>
      </defs>

      <line x1="120" y1={lineYStart} x2="120" y2={lineYEnd} stroke="#ffffff" strokeOpacity="0.08" strokeWidth="2" />

      <circle cx="120" cy={indicatorY} r="8" fill="#ffffff" fillOpacity="0.1" filter="url(#glow-om)" />
      <circle cx="120" cy={indicatorY} r="3" fill="#ffffff" />

      {points.map((pt, idx) => {
        const isHovered = Math.abs(indicatorY - pt.y) < 30;
        return (
          <g key={idx} transform={`translate(120, ${pt.y})`}>
            <circle cx="0" cy="0" r="4.5" fill="#000000" stroke="#ffffff" strokeWidth="1.5" strokeOpacity={isHovered ? 0.9 : 0.4} />

            <g transform="translate(30, -22)">
              <rect x="0" y="0" width="220" height="44" rx="2" fill="#000000" stroke="#ffffff" strokeOpacity={isHovered ? 0.15 : 0.05} strokeWidth="1" style={{ transition: "stroke-opacity 0.2s" }} />
              
              <text x="12" y="18" fill="#ffffff" fillOpacity={isHovered ? 0.9 : 0.5} fontFamily="var(--font-mono)" fontSize="10" style={{ transition: "fill-opacity 0.2s" }}>
                {pt.time}
              </text>
              <text x="12" y="32" fill="#ffffff" fillOpacity={isHovered ? 0.6 : 0.3} fontFamily="var(--font-mono)" fontSize="8" letterSpacing="0.05em" style={{ transition: "fill-opacity 0.2s" }}>
                {pt.title}
              </text>
              
              <text x="208" y="24" fill="#ffffff" fillOpacity={isHovered ? 0.8 : 0.3} fontFamily="var(--font-mono)" fontSize="9" textAnchor="end" style={{ transition: "fill-opacity 0.2s" }}>
                {pt.state}
              </text>
            </g>
          </g>
        );
      })}

      {[0, 1, 2, 3, 4].map((i) => {
        const ax = 380 + Math.sin(tick * 0.005 + i * 2) * 15;
        const ay = 150 + i * 60 + Math.cos(tick * 0.008 + i) * 10;
        return <circle key={i} cx={ax} cy={ay} r="1.5" fill="#ffffff" fillOpacity="0.25" />;
      })}
    </svg>
  );
}

function SecurityIllustration() {
  const [tick, setTick] = useState(0);

  useEffect(() => {
    let anim = requestAnimationFrame(function run() {
      setTick((t) => t + 1);
      anim = requestAnimationFrame(run);
    });
    return () => cancelAnimationFrame(anim);
  }, []);

  const cycleTicks = 420; // 7 seconds cycle duration at 60fps
  const prog = (tick % cycleTicks) / cycleTicks;

  // Track active nodes and calculate particle positions along the pipeline
  let activeNode = -1;
  let mainParticle = null; // { x, y, opacity }
  let pulseProg = 0;

  if (prog < 0.12) {
    // Stage 0: HTTPS active
    activeNode = 0;
    mainParticle = { x: 180, y: 90, opacity: 1.0 };
  } else if (prog < 0.26) {
    // Transition Stage 0 -> Stage 1
    const t = (prog - 0.12) / 0.14;
    mainParticle = { x: 180, y: 90 + t * 80, opacity: 1.0 };
  } else if (prog < 0.36) {
    // Stage 1: Certificate Pin active
    activeNode = 1;
    mainParticle = { x: 180, y: 170, opacity: 1.0 };
  } else if (prog < 0.50) {
    // Transition Stage 1 -> Stage 2
    const t = (prog - 0.36) / 0.14;
    mainParticle = { x: 180, y: 170 + t * 80, opacity: 1.0 };
  } else if (prog < 0.60) {
    // Stage 2: SHA-256 Verify active
    activeNode = 2;
    mainParticle = { x: 180, y: 250, opacity: 1.0 };
  } else if (prog < 0.74) {
    // Transition Stage 2 -> Stage 3
    const t = (prog - 0.60) / 0.14;
    mainParticle = { x: 180, y: 250 + t * 80, opacity: 1.0 };
  } else if (prog < 0.84) {
    // Stage 3: Ed25519 Signature active
    activeNode = 3;
    mainParticle = { x: 180, y: 330, opacity: 1.0 };
  } else if (prog < 0.94) {
    // Transition Stage 3 -> Stage 4
    const t = (prog - 0.84) / 0.10;
    mainParticle = { x: 180, y: 330 + t * 80, opacity: 1.0 };
  } else {
    // Stage 4: Authenticated Firmware (Verified) active with soft pulse
    activeNode = 4;
    mainParticle = { x: 180, y: 410, opacity: 1.0 };
    pulseProg = (prog - 0.94) / 0.06;
  }

  const stages = [
    { x: 180, y: 90, id: 0, meta: "TRANSPORT", label: "HTTPS" },
    { x: 180, y: 170, id: 1, meta: "PINNED", label: "CERT PIN" },
    { x: 180, y: 250, id: 2, meta: "INTEGRITY", label: "SHA-256" },
    { x: 180, y: 330, id: 3, meta: "SIGNATURE", label: "ED25519" },
    { x: 180, y: 410, id: 4, meta: "AUTHENTIC", label: "VERIFIED" }
  ];

  return (
    <div className="security-canvas-container" style={{ width: "100%", height: "100%", maxWidth: "450px", position: "relative" }}>
      <svg viewBox="0 0 500 500" fill="none" style={{ width: "100%", height: "100%", display: "block" }}>
        
        {/* 1. PIPELINE LINE - very thin and low contrast (#262626) */}
        <line x1="180" y1="60" x2="180" y2="440" stroke="#262626" strokeWidth="1" />

        {/* 2. STAGE NODES & MONOSPACE LABELS */}
        {stages.map((st) => {
          const isActive = activeNode === st.id;
          
          return (
            <g key={st.id}>
              {/* Soft white glow behind active stage node */}
              {isActive && (
                <circle cx={st.x} cy={st.y} r="8" fill="#ffffff" fillOpacity="0.08" />
              )}
              {/* Central stage node dot */}
              <circle
                cx={st.x}
                cy={st.y}
                r="3"
                fill={isActive ? "#ffffff" : "#000000"}
                stroke="#ffffff"
                strokeWidth="1.25"
                strokeOpacity={isActive ? 0.9 : 0.2}
                style={{ transition: "stroke-opacity 0.2s, fill 0.2s" }}
              />

              {/* Monospace label card to the right */}
              <g transform={`translate(${st.x + 30}, ${st.y - 20})`}>
                <rect
                  x="0"
                  y="0"
                  width="180"
                  height="40"
                  rx="2"
                  fill="#000000"
                  stroke="#ffffff"
                  strokeOpacity={isActive ? 0.15 : 0.03}
                  strokeWidth="1"
                  style={{ transition: "stroke-opacity 0.2s" }}
                />
                
                {/* Meta label */}
                <text
                  x="12"
                  y="16"
                  fill="#ffffff"
                  fillOpacity={isActive ? 0.8 : 0.25}
                  fontFamily="var(--font-mono)"
                  fontSize="9"
                  style={{ transition: "fill-opacity 0.2s" }}
                >
                  {st.meta}
                </text>
                {/* Monospace uppercase title */}
                <text
                  x="12"
                  y="28"
                  fill="#ffffff"
                  fillOpacity={isActive ? 0.6 : 0.18}
                  fontFamily="var(--font-mono)"
                  fontSize="8"
                  letterSpacing="0.08em"
                  style={{ transition: "fill-opacity 0.2s" }}
                >
                  {st.label}
                </text>
              </g>
            </g>
          );
        })}

        {/* 3. SOFT PULSE AT FINAL DESTINATION NODE */}
        {activeNode === 4 && (
          <circle
            cx={stages[4].x}
            cy={stages[4].y}
            r={6 + pulseProg * 12}
            fill="none"
            stroke="#ffffff"
            strokeWidth="0.75"
            strokeOpacity={0.22 * (1 - pulseProg)}
          />
        )}

        {/* 4. ACTIVE FLOW PARTICLE */}
        {mainParticle && (
          <circle
            cx={mainParticle.x}
            cy={mainParticle.y}
            r="2.5"
            fill="#ffffff"
            fillOpacity={mainParticle.opacity}
          />
        )}

      </svg>
    </div>
  );
}

function AnimatedNumber({ value, suffix = "", prefix = "", decimals = 0 }: { value: number, suffix?: string, prefix?: string, decimals?: number }) {
  const ref = React.useRef(null);
  const isInView = useInView(ref, { once: true, amount: 0.5 });
  const count = useMotionValue(0);
  const [display, setDisplay] = useState("0");

  useEffect(() => {
    return count.on("change", (v) => setDisplay(v.toFixed(decimals)));
  }, [count, decimals]);

  useEffect(() => {
    if (isInView) {
      const controls = animate(count, value, { duration: 2.5, ease: "easeOut" });
      return controls.stop;
    }
  }, [isInView, count, value]);

  return <span ref={ref}>{prefix}{display}{suffix}</span>;
}

function ProofSection() {
  const metrics = [
    { value: 1000, suffix: '+', label: 'CONCURRENT DEVICES', caption: 'Simulated IoT fleet in sustained load test.' },
    { value: 0.00, suffix: '%', decimals: 2, label: 'MESSAGE LOSS', caption: 'Verified across 3.6M+ messages.' },
    { value: 1, prefix: '<', suffix: 'ms', label: 'P50 LATENCY', caption: 'End-to-end message processing time.' },
  ];

  const containerVariants = {
    initial: {},
    whileInView: {
      transition: {
        staggerChildren: 0.08
      }
    }
  };

  const itemVariants = {
    initial: { opacity: 0, y: 40 },
    whileInView: {
      opacity: 1,
      y: 0,
      transition: {
        duration: 0.8,
        ease: [0.22, 1, 0.36, 1] as any
      }
    }
  };

  return (
    <section className="proof-section">
      <div className="proof-content">
        <motion.div
          className="proof-metrics"
          initial="initial"
          whileInView="whileInView"
          viewport={{ once: true, amount: 0.15 }}
          variants={containerVariants}
        >
          {metrics.map((m, i) => (
            <motion.div
              key={i}
              className="proof-metric"
              variants={itemVariants}
            >
              <h3 className="proof-metric-value">
                <AnimatedNumber value={m.value} suffix={m.suffix} prefix={m.prefix} decimals={m.decimals} />
              </h3>
              <div className="proof-metric-label">{m.label}</div>
              <div className="proof-metric-caption">{m.caption}</div>
            </motion.div>
          ))}
        </motion.div>
      </div>
    </section>
  );
}

