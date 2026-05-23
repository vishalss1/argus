import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { NavLink } from "react-router-dom";
import { PageHeader, Panel } from "../components/ui";

const sidebarSections = [
  {
    label: "Getting Started",
    items: [
      { id: "intro", text: "Introduction" },
      { id: "architecture-overview", text: "Architecture Overview" },
      { id: "run", text: "Quick Start" }
    ]
  },
  {
    label: "Core Concepts",
    items: [
      { id: "devices", text: "Device Registry" },
      { id: "shadow", text: "Digital Twin / Shadow" },
      { id: "telemetry", text: "Telemetry Pipeline" }
    ]
  },
  {
    label: "API Reference",
    items: [
      { id: "devices-api", text: "Devices API" },
      { id: "telemetry-api", text: "Telemetry API" },
      { id: "commands", text: "Commands API" },
      { id: "ota", text: "OTA API" },
      { id: "shadow-api", text: "Shadow API" },
      { id: "rules", text: "Rules & Alerts API" }
    ]
  },
  {
    label: "Infrastructure",
    items: [
      { id: "mqtt", text: "MQTT Integration" },
      { id: "redpanda", text: "Redpanda" },
      { id: "redis", text: "Redis & Shadows" },
      { id: "minio", text: "MinIO / OTA Store" }
    ]
  }
];

const tocItems = [
  { id: "intro", text: "What is ARGUS?" },
  { id: "architecture-overview", text: "Architecture Layers" },
  { id: "data-flow", text: "Core Data Flow" },
  { id: "config", text: "Configuration" },
  { id: "api-ref", text: "API Reference" },
  { id: "run", text: "Running Locally" }
];

const endpoints = [
  ["GET", "/devices/", "List registered devices"],
  ["POST", "/devices/", "Create a device"],
  ["PUT", "/devices/{deviceID}/", "Update a device"],
  ["DELETE", "/devices/{deviceID}/", "Delete a device"],
  ["POST", "/devices/{deviceID}/heartbeat", "Record heartbeat state"],
  ["POST", "/devices/{deviceID}/telemetry", "Ingest telemetry metrics"],
  ["GET", "/devices/{deviceID}/commands", "List commands for a device"],
  ["POST", "/devices/{deviceID}/commands", "Send a command"],
  ["POST", "/devices/{deviceID}/commands/{commandID}/ack", "ACK a command"],
  ["POST", "/devices/{deviceID}/commands/{commandID}/nack", "NACK a command"],
  ["GET", "/devices/{deviceID}/shadow", "Read device shadow"],
  ["PUT", "/devices/{deviceID}/shadow/desired", "Update desired shadow"],
  ["PUT", "/devices/{deviceID}/shadow/reported", "Update reported shadow"],
  ["GET", "/ota/firmware/", "List firmware artifacts"],
  ["POST", "/ota/firmware/", "Upload firmware"],
  ["POST", "/devices/{deviceID}/ota", "Create OTA deployment manifest"],
  ["GET", "/rules/", "List telemetry rules"],
  ["POST", "/rules/", "Create telemetry rule"],
  ["GET", "/alerts", "List generated alerts"],
  ["GET", "/healthz", "Health check"],
  ["GET", "/metrics", "Prometheus metrics"]
];

export function DocumentationPage() {
  const [searchQuery, setSearchQuery] = useState("");
  const [activeSection, setActiveSection] = useState("intro");

  const filteredSections = sidebarSections.map((section) => ({
    ...section,
    items: section.items.filter((item) =>
      item.text.toLowerCase().includes(searchQuery.toLowerCase())
    )
  })).filter((section) => section.items.length > 0);

  return (
    <section className="section">
      <PageHeader
        eyebrow="Engineering Docs"
        title="ARGUS documentation"
        description="Frontend documentation generated from the implemented backend routes, handlers, DTOs, and runtime configuration."
      />
      <div className="docs-layout">
        <aside className="docs-sidebar">
          <div className="docs-search">
            <input
              placeholder="Search docs..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
          {filteredSections.map((section) => (
            <div className="docs-nav-group" key={section.label}>
              <span className="docs-nav-label">{section.label}</span>
              {section.items.map((item) => (
                <a
                  key={item.id}
                  href={`#${item.id}`}
                  className={`docs-nav-item ${activeSection === item.id ? "active" : ""}`}
                  onClick={() => setActiveSection(item.id)}
                >
                  {item.text}
                  <ChevronRight size={12} />
                </a>
              ))}
            </div>
          ))}
        </aside>
        <article className="docs-content">
          <Panel title="Introduction to ARGUS">
            <section id="intro">
              <p className="muted">
                ARGUS is a production-grade IoT fleet orchestration system built for distributed edge device management.
                It exposes a REST API for device registry, heartbeat tracking, telemetry ingestion, command dispatch,
                Redis-backed shadows, MinIO-backed OTA firmware delivery, threshold-based rules, alert generation,
                and Prometheus metrics.
              </p>
              <h2 id="architecture-overview">What is ARGUS?</h2>
              <p className="muted">
                ARGUS (Adaptive Remote Grid Utilization System) is a modular monolith that exposes a REST API for fleet
                management. The backend persists data in PostgreSQL, stores device shadows in Redis, handles firmware in MinIO, and publishes
                events to Redpanda for downstream processing.
              </p>
            </section>
            <section>
              <h2>Architecture Layers</h2>
              <p className="muted">
                The system is composed of five core layers: Device Runtime (edge), Fleet Gateway (ingestion),
                Domain Services (business logic), Infrastructure Adapters (persistence, messaging, caching, metadata), and Operator Interfaces (REST/UI).
              </p>
            </section>
            <section id="data-flow">
              <h2>Core Data Flow</h2>
              <p className="muted">
                Device → MQTT/HTTP → Ingestion API → Redpanda Decorator → Rules Evaluation → PostgreSQL → Shadow Sync → Alerts → Operator Dashboard.
              </p>
            </section>
            <section id="config">
              <h2 id="run">Configuration</h2>
              <pre className="code-block">{`docker compose -f deployments/compose/docker-compose.yml up -d postgres redis minio
go run ./cmd/api/main.go
cd frontend
npm run dev`}</pre>
            </section>
            <section id="api-ref">
              <h2 id="devices">API Reference</h2>
              <p className="muted">Devices are the central resource. Heartbeats update status and last_seen.</p>
              <pre className="code-block">{`POST /devices/{deviceID}/telemetry
Content-Type: application/json

{ "metrics": { "temp": 38.2, "cpu": 12, "humidity": 62.1 } }`}</pre>
            </section>
            <section id="telemetry">
              <h2>Telemetry API</h2>
              <p className="muted">The backend currently exposes ingestion only. Rule evaluation happens after persistence.</p>
            </section>
            <section id="commands">
              <h2>Commands API</h2>
              <p className="muted">Commands are scoped to a device and move through pending, acked, or nacked states.</p>
            </section>
            <section id="ota">
              <h2>OTA API</h2>
              <p className="muted">Firmware uploads use multipart form data with version and firmware fields. Deployments return a manifest with a presigned download URL.</p>
            </section>
            <section id="rules">
              <h2>Rules API</h2>
              <p className="muted">Rules define metric, operator, threshold, and enabled state. Alerts are produced when telemetry violates enabled rules.</p>
            </section>
            <section id="observability">
              <h2>Observability</h2>
              <p className="muted">The API serves /healthz and /metrics. Grafana and Prometheus are available through the Docker Compose observability stack.</p>
            </section>
          </Panel>
        </article>
        <aside className="toc">
          <strong>On this page</strong>
          {tocItems.map((item) => (
            <a
              key={item.id}
              href={`#${item.id}`}
              className={activeSection === item.id ? "active" : ""}
              onClick={() => setActiveSection(item.id)}
            >
              {item.text}
            </a>
          ))}
        </aside>
      </div>
      <div style={{ marginTop: 22 }}>
        <Panel title="Implemented Endpoints">
          <div className="table-wrap">
            <table>
              <thead><tr><th>Method</th><th>Path</th><th>Purpose</th></tr></thead>
              <tbody>
                {endpoints.map(([method, path, purpose]) => (
                  <tr key={`${method}-${path}`}>
                    <td><span className="status-chip tone-info">{method}</span></td>
                    <td className="mono">{path}</td>
                    <td>{purpose}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>
    </section>
  );
}
