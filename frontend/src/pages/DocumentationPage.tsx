import { ExternalLink, ChevronRight } from "lucide-react";


const SDK_GITHUB_URL = "https://github.com/vishalss1/argus/tree/master/argus_sdk";

/* ─── Static content ─────────────────────────────── */

const howItWorks = [
  {
    step: "01",
    title: "Register & create a workspace",
    desc: "Sign up for an account and create a workspace. A workspace is the top-level organisational unit — it groups your devices, sessions, OTA pipelines and alert rules together.",
  },
  {
    step: "02",
    title: "Flash the SDK onto your ESP32",
    desc: "Download the Argus C++ SDK from GitHub. Fill in argus_config.h with your workspace credentials, server address, and TLS CA certificate. Build and flash using Arduino IDE or PlatformIO.",
  },
  {
    step: "03",
    title: "Device comes online",
    desc: "On boot the SDK establishes a secure TLS connection, publishes a heartbeat over MQTT, and registers itself with the core service. The device appears in your Fleet Overview within seconds.",
  },
  {
    step: "04",
    title: "Stream telemetry & receive commands",
    desc: "The SDK periodically publishes sensor readings to the telemetry service. Meanwhile the command topic is subscribed — any command you dispatch from the dashboard lands on the device with full ACK/NACK tracking.",
  },
  {
    step: "05",
    title: "Deploy firmware over-the-air",
    desc: "Upload a firmware binary through the Deployments page. Argus stores it in MinIO, generates a SHA-256 checksum and signs it with Ed25519. When the device checks in it downloads, verifies, and installs the update — rolling back automatically on failure.",
  },
];

const architecture = [
  {
    name: "core-service",
    tag: "Go REST API",
    desc: "Primary HTTP API responsible for device registration, command dispatch, shadow state (desired ↔ reported), OTA manifests, and rule evaluation. Backed by PostgreSQL and Redis.",
  },
  {
    name: "telemetry-service",
    tag: "Go ingestion pipeline",
    desc: "Dedicated high-throughput service for ingesting telemetry payloads over HTTP and MQTT. Forwards events to Redpanda/Kafka for downstream processing and AI analysis.",
  },
  {
    name: "frontend",
    tag: "React 18 · Vite · TypeScript",
    desc: "The web dashboard — Fleet Overview, Device Management, OTA Deployments, Telemetry charts, AI Insights, Command Center, and Automation rules.",
  },
  {
    name: "argus_sdk",
    tag: "C++ · ESP32 · Arduino",
    desc: "Official device SDK. Handles WiFi, TLS, MQTT, HTTP, telemetry publishing, command subscription, OTA download & cryptographic verification, rollback, and diagnostics.",
  },
  {
    name: "infrastructure",
    tag: "Docker Compose",
    desc: "A complete local stack: PostgreSQL, Redis, Mosquitto MQTT, Redpanda (Kafka API), MinIO object storage, Prometheus, Grafana, Loki, and Promtail.",
  },
];

const sdkModules = [
  {
    file: "argus_network",
    desc: "WiFi connection management, reconnection logic, and network health monitoring.",
  },
  {
    file: "argus_mqtt",
    desc: "MQTT client over TLS. Manages heartbeat publishing, telemetry topics, command subscriptions, and shadow sync.",
  },
  {
    file: "argus_http",
    desc: "Secure HTTP client for OTA manifest polling, firmware download, and REST API calls.",
  },
  {
    file: "argus_ota",
    desc: "Full OTA pipeline — manifest fetch, firmware streaming, SHA-256 checksum verification, Ed25519 signature validation, and flash partition management.",
  },
  {
    file: "argus_rollback",
    desc: "Automatic rollback to the previous partition on repeated boot failures or cryptographic verification errors.",
  },
  {
    file: "argus_security",
    desc: "Cryptographic primitives: Ed25519 public-key verification and SHA-256 hashing used by the OTA pipeline.",
  },
  {
    file: "argus_diag",
    desc: "Device diagnostics — heap usage, uptime, reset reason, and structured log publishing.",
  },
  {
    file: "argus_state_machine",
    desc: "Finite state machine driving the device lifecycle: INIT → CONNECTING → CONNECTED → UPDATING → ERROR.",
  },
  {
    file: "argus_time",
    desc: "NTP-based time synchronisation used to timestamp telemetry payloads and OTA log events.",
  },
];

const otaSteps = [
  "Firmware binary is uploaded via the dashboard and stored in MinIO.",
  "Core-service computes a SHA-256 checksum of the binary.",
  "The checksum is signed with an Ed25519 private key held by the server.",
  "The device polls the OTA manifest endpoint over pinned HTTPS/TLS.",
  "On receiving a new manifest the SDK downloads the binary in chunks.",
  "SHA-256 of the downloaded bytes is compared against the manifest checksum.",
  "The Ed25519 signature is verified using the device's baked-in public key.",
  "If either check fails the update is aborted. Otherwise the new partition is booted.",
  "Rollback triggers automatically if the device cannot reach the server after N reboots.",
];

/* ─── Component ──────────────────────────────────── */

export function DocumentationPage() {
  return (
    <>
      {/* ── Hero ── */}
      <section className="section" style={{ borderTop: "none" }}>
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Documentation</p>
          <h1 className="t-h1" style={{ maxWidth: 800, marginBottom: 32 }}>
            Everything you need to connect, monitor, and control your devices.
          </h1>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 48 }}>
            ARGUS is a full-stack IoT fleet orchestration platform. This page covers the
            system architecture, how the pieces fit together, and how to get your ESP32
            talking to the platform in under fifteen minutes.
          </p>

          {/* SDK Download CTA */}
          <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
            <a
              href={SDK_GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn-inverse"
              id="sdk-download-btn"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38
                  0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52
                  -.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07
                  -1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12
                  0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04
                  2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87
                  3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38
                  A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
                />
              </svg>
              Download SDK on GitHub
              <ExternalLink size={13} aria-hidden />
            </a>
          </div>
        </div>
      </section>

      {/* ── How it works ── */}
      <section className="section" id="how-it-works">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Getting started</p>
          <h2 className="t-h1" style={{ maxWidth: 680, marginBottom: 96 }}>
            How the system works.
          </h2>

          <div className="dl-list">
            {howItWorks.map((item) => (
              <div className="dl-row" key={item.step}>
                <div>
                  <span className="dl-num">{item.step}</span>
                  <div className="dl-name">{item.title}</div>
                </div>
                <div className="dl-desc">{item.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Architecture ── */}
      <section className="section" id="architecture">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Architecture</p>
          <h2 className="t-h1" style={{ maxWidth: 680, marginBottom: 48 }}>
            System components.
          </h2>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 96 }}>
            ARGUS is a microservice-oriented system. Each component owns a well-defined
            responsibility and communicates over REST, MQTT, or Kafka.
          </p>

          <div className="dl-list">
            {architecture.map((item, idx) => (
              <div className="dl-row" key={item.name}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                  <div className="dl-name" style={{ fontFamily: "var(--font-mono)", fontSize: 13 }}>
                    {item.name}
                  </div>
                  <div style={{
                    fontFamily: "var(--font-mono)",
                    fontSize: 11,
                    color: "var(--text-muted)",
                    marginTop: 4,
                    letterSpacing: "0.06em",
                    textTransform: "uppercase",
                  }}>
                    {item.tag}
                  </div>
                </div>
                <div className="dl-desc">{item.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── SDK Reference ── */}
      <section className="section" id="sdk">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>SDK Reference</p>
          <h2 className="t-h1" style={{ maxWidth: 680, marginBottom: 48 }}>
            The Argus C++ SDK.
          </h2>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 64 }}>
            The SDK runs on any ESP32 board with the Arduino core. It handles the entire
            device lifecycle so you only need to wire up your sensors and implement your
            application logic.
          </p>

          {/* Quick install block */}
          <div style={{
            background: "var(--surface)",
            border: "1px solid var(--border)",
            borderRadius: "var(--radius)",
            padding: "24px 28px",
            marginBottom: 80,
            maxWidth: 680,
          }}>
            <p className="t-eyebrow" style={{ marginBottom: 16 }}>Quick install</p>
            <p className="t-body" style={{ marginBottom: 20 }}>
              Clone the repository and copy the <code style={{ fontFamily: "var(--font-mono)", fontSize: 12, color: "var(--text-primary)", background: "var(--surface-2)", padding: "2px 6px", borderRadius: 4 }}>argus_sdk/</code> folder
              into your Arduino libraries directory, or add it as a Git submodule in your
              PlatformIO project.
            </p>
            <div style={{
              background: "var(--surface-2)",
              border: "1px solid var(--border)",
              borderRadius: "var(--radius-sm)",
              padding: "14px 18px",
              fontFamily: "var(--font-mono)",
              fontSize: 13,
              color: "var(--text-secondary)",
              lineHeight: 1.8,
            }}>
              <span style={{ color: "var(--text-muted)" }}># Clone the full repo</span><br />
              git clone https://github.com/vishalss1/argus.git<br />
              <br />
              <span style={{ color: "var(--text-muted)" }}># Copy SDK to your Arduino libraries</span><br />
              cp -r argus/argus_sdk ~/Arduino/libraries/ArgusSDK<br />
              <br />
              <span style={{ color: "var(--text-muted)" }}># Or use as a PlatformIO lib_dep</span><br />
              lib_deps = https://github.com/vishalss1/argus.git#master
            </div>

            <div style={{ marginTop: 20 }}>
              <a
                href={SDK_GITHUB_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="btn-outline"
                id="sdk-github-link"
                style={{ fontSize: 13 }}
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38
                    0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52
                    -.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07
                    -1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12
                    0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04
                    2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87
                    3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38
                    A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
                  />
                </svg>
                View SDK source on GitHub
                <ExternalLink size={12} aria-hidden />
              </a>
            </div>
          </div>

          {/* Modules table */}
          <p className="t-eyebrow" style={{ marginBottom: 32 }}>Modules</p>
          <div className="dl-list">
            {sdkModules.map((mod) => (
              <div className="dl-row" key={mod.file}>
                <div>
                  <div className="dl-name" style={{ fontFamily: "var(--font-mono)", fontSize: 13 }}>
                    {mod.file}
                    <span style={{ color: "var(--text-muted)", fontWeight: 400 }}>.h</span>
                  </div>
                </div>
                <div className="dl-desc">{mod.desc}</div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── OTA Security ── */}
      <section className="section" id="ota-security">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Security</p>
          <h2 className="t-h1" style={{ maxWidth: 680, marginBottom: 48 }}>
            OTA security model.
          </h2>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 96 }}>
            Every firmware update passes through a cryptographic verification chain. A single
            failed check aborts the installation and keeps the current partition running.
          </p>

          <div className="dl-list">
            {otaSteps.map((step, idx) => (
              <div className="dl-row" key={idx}>
                <div>
                  <span className="dl-num">{String(idx + 1).padStart(2, "0")}</span>
                </div>
                <div className="dl-desc">{step}</div>
              </div>
            ))}
          </div>

          {/* Algo callout */}
          <div style={{
            marginTop: 80,
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
            gap: 1,
            background: "var(--border)",
            borderRadius: "var(--radius)",
            overflow: "hidden",
          }}>
            {[
              { label: "Transport", value: "TLS 1.2 / 1.3", sub: "pinned CA certificate" },
              { label: "Integrity", value: "SHA-256", sub: "full payload checksum" },
              { label: "Authenticity", value: "Ed25519", sub: "server-signed manifest" },
              { label: "Resilience", value: "Auto-rollback", sub: "on boot failure" },
            ].map((item) => (
              <div key={item.label} style={{
                background: "var(--surface)",
                padding: "28px 32px",
              }}>
                <p className="t-eyebrow" style={{ marginBottom: 10 }}>{item.label}</p>
                <p style={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 18,
                  color: "var(--text-primary)",
                  marginBottom: 6,
                  letterSpacing: "-0.01em",
                }}>
                  {item.value}
                </p>
                <p style={{ fontSize: 13, color: "var(--text-muted)" }}>{item.sub}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ── Configuration reference ── */}
      <section className="section" id="configuration">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Configuration</p>
          <h2 className="t-h1" style={{ maxWidth: 680, marginBottom: 48 }}>
            argus_config.h reference.
          </h2>
          <p className="t-body-lg" style={{ maxWidth: 680, marginBottom: 64 }}>
            All device-side configuration lives in a single header file. Credentials are
            never stored in flash beyond what is required — the CA certificate is embedded
            at compile time.
          </p>

          <div style={{
            background: "var(--surface)",
            border: "1px solid var(--border)",
            borderRadius: "var(--radius)",
            padding: "28px 32px",
            maxWidth: 760,
            fontFamily: "var(--font-mono)",
            fontSize: 13,
            lineHeight: 2,
            color: "var(--text-secondary)",
          }}>
            <div style={{ color: "var(--text-muted)" }}>// argus_config.h — fill this in before flashing</div>
            <br />
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_WIFI_SSID&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>"your-network"</span></div>
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_WIFI_PASSWORD&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>"your-password"</span></div>
            <br />
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_SERVER_HOST&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>"your.argus.server"</span></div>
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_SERVER_PORT&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>8443</span></div>
            <br />
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_DEVICE_ID&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>"device-uuid-from-dashboard"</span></div>
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_API_KEY&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>"workspace-api-key"</span></div>
            <br />
            <div><span style={{ color: "var(--text-muted)" }}>// Paste the CA cert PEM string below</span></div>
            <div><span style={{ color: "var(--text-muted)" }}>#define</span> ARGUS_CA_CERT&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span style={{ color: "var(--text-primary)" }}>R"(-----BEGIN CERTIFICATE-----</span></div>
            <div style={{ color: "var(--text-primary)", paddingLeft: 24 }}>... your CA cert ...</div>
            <div style={{ color: "var(--text-primary)" }}>-----END CERTIFICATE-----)"</div>
          </div>
        </div>
      </section>

      {/* ── Footer CTA ── */}
      <section className="section">
        <div className="container">
          <p className="t-eyebrow" style={{ marginBottom: 24 }}>Ready to build?</p>
          <h2 className="t-h1" style={{ maxWidth: 640, marginBottom: 40 }}>
            Download the SDK and connect your first device.
          </h2>
          <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
            <a
              href={SDK_GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="btn-inverse"
              id="sdk-download-footer-btn"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38
                  0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52
                  -.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07
                  -1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12
                  0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04
                  2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87
                  3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38
                  A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"
                />
              </svg>
              Get the SDK on GitHub
              <ExternalLink size={13} aria-hidden />
            </a>
            <a href="/login" className="btn-outline" id="docs-cta-login-btn">
              Open the dashboard
              <ChevronRight size={13} aria-hidden />
            </a>
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
