import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  BookOpen,
  Brain,
  Briefcase,
  Cpu,
  FileText,
  Gauge,
  RadioTower,
  Rocket,
  Send,
  Settings,
  Workflow,
  ArrowRight,
  Plus,
  RefreshCw
} from "lucide-react";
import { LiveIndicator, SearchBar } from "../components/ui";
import { useHealth, useCreateWorkspace } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";

const monitorLinks = [
  { to: "/workspaces", label: "Workspaces", icon: Briefcase },
  { to: "/dashboard", label: "Fleet Overview", icon: BarChart3 },
  { to: "/telemetry", label: "Telemetry", icon: Activity },
  { to: "/devices", label: "Devices", icon: Cpu }
];

const controlLinks = [
  { to: "/commands", label: "Commands", icon: Send },
  { to: "/ota", label: "OTA Updates", icon: Rocket },
  { to: "/alerts", label: "Automations", icon: Workflow }
];

const observeLinks = [
  { to: "/ai", label: "AI Insights", icon: Brain },
  { to: "/alerts", label: "Alerts", icon: AlertTriangle }
];

const bottomLinks = [
  { to: "/settings", label: "Settings", icon: Settings },
  { to: "/documentation", label: "Documentation", icon: BookOpen }
];

function NavGroup({ label, links }: { label: string; links: { to: string; label: string; icon: LucideIcon }[] }) {
  return (
    <div className="nav-group">
      <span>{label}</span>
      {links.map((link) => {
        const Icon = link.icon;
        return (
          <NavLink key={link.to + link.label} to={link.to}>
            <Icon size={16} aria-hidden />
            {link.label}
          </NavLink>
        );
      })}
    </div>
  );
}

function WorkspaceOnboarding() {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const createWorkspace = useCreateWorkspace();

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    await createWorkspace.mutateAsync({ name: name.trim(), description: description.trim() });
    setName("");
    setDescription("");
  };

  return (
    <div style={{ display: "flex", alignItems: "center", justifyContent: "center", minHeight: "70vh", padding: "40px 20px" }}>
      <div style={{ maxWidth: 560, width: "100%", textAlign: "center" }}>
        <div style={{
          width: 80, height: 80, borderRadius: "20px", margin: "0 auto 28px",
          background: "linear-gradient(135deg, rgba(99,102,241,0.2), rgba(139,92,246,0.15))",
          border: "1px solid rgba(99,102,241,0.3)",
          display: "flex", alignItems: "center", justifyContent: "center"
        }}>
          <Briefcase size={36} style={{ color: "var(--accent)" }} />
        </div>

        <h1 style={{ fontSize: "28px", fontWeight: 700, margin: "0 0 12px", letterSpacing: "-0.5px" }}>
          Welcome to ARGUS
        </h1>
        <p className="muted" style={{ fontSize: "16px", lineHeight: 1.6, margin: "0 0 36px", maxWidth: 420, marginLeft: "auto", marginRight: "auto" }}>
          Create your first workspace to organize devices, run sessions, and monitor your fleet in real time.
        </p>

        <form onSubmit={handleCreate} style={{
          background: "var(--surface)", border: "1px solid var(--line)", borderRadius: "12px",
          padding: "28px", textAlign: "left"
        }}>
          <h3 style={{ margin: "0 0 20px", fontSize: "16px", fontWeight: 600, display: "flex", alignItems: "center", gap: "8px" }}>
            <Plus size={18} style={{ color: "var(--accent)" }} />
            Create Workspace
          </h3>

          <div className="form-group" style={{ marginBottom: "16px" }}>
            <label style={{ display: "block", fontSize: "13px", fontWeight: 500, marginBottom: "6px", color: "var(--faint)" }}>
              Workspace Name
            </label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Factory Floor Alpha"
              required
              autoFocus
              style={{
                width: "100%", padding: "10px 14px", fontSize: "14px",
                background: "var(--surface-2)", border: "1px solid var(--line)",
                borderRadius: "8px", color: "inherit", outline: "none",
                boxSizing: "border-box"
              }}
            />
          </div>

          <div className="form-group" style={{ marginBottom: "24px" }}>
            <label style={{ display: "block", fontSize: "13px", fontWeight: 500, marginBottom: "6px", color: "var(--faint)" }}>
              Description
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional — describe what this workspace monitors"
              rows={3}
              style={{
                width: "100%", padding: "10px 14px", fontSize: "14px",
                background: "var(--surface-2)", border: "1px solid var(--line)",
                borderRadius: "8px", color: "inherit", outline: "none", resize: "vertical",
                boxSizing: "border-box"
              }}
            />
          </div>

          <button
            type="submit"
            className="button primary"
            disabled={createWorkspace.isPending || !name.trim()}
            style={{ width: "100%", padding: "12px", fontSize: "15px", fontWeight: 600, display: "flex", alignItems: "center", justifyContent: "center", gap: "8px" }}
          >
            {createWorkspace.isPending ? (
              "Creating..."
            ) : (
              <>
                Get Started
                <ArrowRight size={16} />
              </>
            )}
          </button>
        </form>

        <div style={{
          display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "16px", marginTop: "32px"
        }}>
          {[
            { icon: <Briefcase size={18} />, label: "Organize Devices", desc: "Group by project" },
            { icon: <Rocket size={18} />, label: "Run Sessions", desc: "Monitor in real time" },
            { icon: <RefreshCw size={18} />, label: "OTA Updates", desc: "Deploy firmware" }
          ].map((item) => (
            <div key={item.label} style={{
              padding: "16px 12px", borderRadius: "10px",
              background: "rgba(255,255,255,0.02)", border: "1px solid var(--line)",
              textAlign: "center"
            }}>
              <div style={{ color: "var(--accent)", marginBottom: "8px" }}>{item.icon}</div>
              <div style={{ fontSize: "13px", fontWeight: 500, marginBottom: "4px" }}>{item.label}</div>
              <div className="muted" style={{ fontSize: "11px" }}>{item.desc}</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export function AppLayout() {
  const health = useHealth();
  const { selectedWorkspaceId, setSelectedWorkspaceId, workspaces } = useWorkspaceContext();

  const hasWorkspaces = workspaces.length > 0;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <NavLink className="brand sidebar-brand" to="/">
          <span className="brand-mark">
            <Activity size={16} aria-hidden />
          </span>
          <span>
            <strong>ARGUS</strong>
            <small>Fleet Control</small>
          </span>
        </NavLink>

        {hasWorkspaces && (
          <div className="workspace-selector-nav" style={{ padding: "0 16px 16px", borderBottom: "1px solid var(--line)", marginBottom: "16px" }}>
            <label style={{ display: "block", fontSize: "10px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "6px", letterSpacing: "0.5px" }}>
              Active Workspace
            </label>
            <div style={{ position: "relative" }}>
              <select
                value={selectedWorkspaceId}
                onChange={(e) => setSelectedWorkspaceId(e.target.value)}
                style={{
                  width: "100%",
                  padding: "8px 28px 8px 12px",
                  background: "var(--surface-2)",
                  border: "1px solid var(--line)",
                  borderRadius: "6px",
                  color: "var(--text)",
                  fontSize: "13px",
                  outline: "none",
                  appearance: "none",
                  cursor: "pointer",
                  fontWeight: 500
                }}
              >
                {workspaces.map(ws => (
                  <option key={ws.id} value={ws.id}>{ws.name}</option>
                ))}
              </select>
              <Briefcase size={14} style={{ position: "absolute", right: "12px", top: "50%", transform: "translateY(-50%)", color: "var(--faint)", pointerEvents: "none" }} />
            </div>
          </div>
        )}

        <NavGroup label="Monitor" links={monitorLinks} />
        <NavGroup label="Control" links={controlLinks} />
        <NavGroup label="Observe" links={observeLinks} />
        <NavGroup label="" links={bottomLinks} />
      </aside>
      <div className="app-main">
        <header className="app-topbar">
          <SearchBar placeholder="Search devices..." />
          <LiveIndicator isOnline={health.data?.ok === true && !health.isError} isChecking={health.isLoading || health.isFetching} />
        </header>
        <main className="workspace">
          {hasWorkspaces ? <Outlet /> : <WorkspaceOnboarding />}
        </main>
      </div>
    </div>
  );
}
