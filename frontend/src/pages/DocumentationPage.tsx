import { useState } from "react";
import { ChevronRight } from "lucide-react";
import { PageHeader, Panel } from "../components/ui";

const sidebarSections = [
  {
    label: "Getting Started",
    items: [
      { id: "intro", text: "Introduction" },
      { id: "devices", text: "Device Registry" },
      { id: "shadow", text: "Digital Twin / Shadow" }
    ]
  },
  {
    label: "Operations",
    items: [
      { id: "telemetry", text: "Telemetry Pipeline" },
      { id: "commands", text: "Commands & Control" },
      { id: "ota", text: "Firmware OTA Updates" },
      { id: "rules", text: "Alerts & Automations" }
    ]
  }
];

const tocItems = [
  { id: "intro", text: "Introduction" },
  { id: "devices", text: "Device Registry" },
  { id: "shadow", text: "Digital Twin" },
  { id: "telemetry", text: "Telemetry" },
  { id: "commands", text: "Commands" },
  { id: "ota", text: "Firmware Updates" },
  { id: "rules", text: "Alerts" }
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
        eyebrow="Help & Support"
        title="ARGUS platform guide"
        description="Learn how to register edge devices, synchronize system state, monitor metrics telemetry, and deploy firmware updates."
      />
      <div className="docs-layout">
        <aside className="docs-sidebar">
          <div className="docs-search">
            <input
              placeholder="Search guide..."
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
          <Panel title="Platform User Manual">
            <section id="intro" style={{ marginBottom: 28 }}>
              <h2>What is ARGUS?</h2>
              <p className="muted">
                ARGUS is an IoT fleet monitoring and control dashboard designed to connect, manage, and automate networks of edge nodes.
                It provides real-time visibility into your device inventory, tracks device heartbeats, streams telemetry data, synchronizes configuration states, and manages firmware updates.
              </p>
            </section>
            
            <section id="devices" style={{ marginBottom: 28 }}>
              <h2>Device Registry</h2>
              <p className="muted">
                The device registry is the inventory of all edge nodes connected to the system. From the registry, operators can:
              </p>
              <ul className="muted" style={{ paddingLeft: 20, lineHeight: 1.6 }}>
                <li>Register new nodes with custom hardware details.</li>
                <li>Monitor live online/offline status updated via device heartbeats.</li>
                <li>Inspect metadata and edit attributes.</li>
                <li>Delete decommissioned nodes from the registry.</li>
              </ul>
            </section>
            
            <section id="shadow" style={{ marginBottom: 28 }}>
              <h2>Digital Twin (Shadow)</h2>
              <p className="muted">
                A digital twin (or shadow state) maintains a synchronized record of your device configuration.
                It tracks two distinct states:
              </p>
              <ul className="muted" style={{ paddingLeft: 20, lineHeight: 1.6 }}>
                <li><strong>Desired State:</strong> The target configuration set by operators (e.g., target reporting interval or setpoints).</li>
                <li><strong>Reported State:</strong> The actual configuration reported back by the device firmware.</li>
              </ul>
              <p className="muted">
                If the Desired State differs from the Reported State, the console highlights a <strong>Drift</strong> condition so operators can investigate.
              </p>
            </section>
            
            <section id="telemetry" style={{ marginBottom: 28 }}>
              <h2>Telemetry Pipeline</h2>
              <p className="muted">
                Edge devices continuously stream telemetry data (such as temperature, CPU utilization, signal strength, and voltage).
                The Telemetry page lets you simulate and ingest new metric payloads, observe active telemetry streams, and inspect parsed payload values in real time.
              </p>
            </section>
            
            <section id="commands" style={{ marginBottom: 28 }}>
              <h2>Commands & Control</h2>
              <p className="muted">
                Operators can send remote commands to devices (e.g., triggering a self-test, reboot, or diagnostic dump).
                Commands are dispatched to the edge queue and their acknowledgement status (Pending, Acknowledged, or Failed) is reported back to the console.
              </p>
            </section>
            
            <section id="ota" style={{ marginBottom: 28 }}>
              <h2>Firmware OTA Updates</h2>
              <p className="muted">
                Deploy firmware updates over-the-air (OTA) to edge nodes securely:
              </p>
              <ul className="muted" style={{ paddingLeft: 20, lineHeight: 1.6 }}>
                <li>Upload new firmware binaries and register update versions.</li>
                <li>Initiate a rollout deployment manifest targeted to a specific device.</li>
                <li>Observe the installation progress and acknowledgment response from the device.</li>
              </ul>
            </section>
            
            <section id="rules" style={{ marginBottom: 28 }}>
              <h2>Alerts & Automations</h2>
              <p className="muted">
                Set up automated rules to continuously monitor telemetry metrics against defined thresholds.
                When a telemetry metric (e.g., temperature) violates a rule condition, the system automatically triggers an <strong>Alert</strong>, logs the violation, and shows notifications in the Alerts panel.
              </p>
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
    </section>
  );
}
