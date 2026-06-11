import { useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Brain,
  Briefcase,
  Cpu,
  Rocket,
  Send,
  Settings,
  Workflow,
  ArrowRight,
  Plus,
  RefreshCw,
  LogOut
} from "lucide-react";
import { LiveIndicator, SearchBar } from "../components/ui";
import { useHealth, useCreateWorkspace } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { useAuth } from "../context/AuthContext";

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
  { to: "/settings", label: "Settings", icon: Settings }
];

function NavGroup({ label, links }: { label: string; links: { to: string; label: string; icon: LucideIcon }[] }) {
  return (
    <div className="nav-group">
      {label && <span>{label}</span>}
      {links.map((link) => {
        const Icon = link.icon;
        return (
          <NavLink key={link.to + link.label} to={link.to}>
            <Icon size={15} aria-hidden />
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
  const { refreshMe } = useAuth();

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) return;
    await createWorkspace.mutateAsync({ name: name.trim(), description: description.trim() });
    await refreshMe();
    setName("");
    setDescription("");
  };

  return (
    <div className="onboarding-shell">
      <div className="onboarding-inner">
        <div className="onboarding-mark">
          <Briefcase size={32} strokeWidth={1.5} />
        </div>

        <span className="eyebrow">Get started</span>
        <h1 className="onboarding-title">Welcome to ARGUS</h1>
        <p className="onboarding-sub">
          Create your first workspace to organize devices, run sessions, and monitor your fleet in real time.
        </p>

        <form onSubmit={handleCreate} className="onboarding-form">
          <div className="form-group">
            <label>Workspace Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Factory Floor Alpha"
              required
              autoFocus
            />
          </div>

          <div className="form-group">
            <label>Description</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Optional — describe what this workspace monitors"
              rows={3}
            />
          </div>

          <button
            type="submit"
            className="btn-inverse auth-button"
            disabled={createWorkspace.isPending || !name.trim()}
          >
            {createWorkspace.isPending ? (
              "Creating..."
            ) : (
              <>
                Get Started
                <ArrowRight size={15} />
              </>
            )}
          </button>
        </form>

        <div className="onboarding-bullets">
          {[
            { icon: <Briefcase size={16} strokeWidth={1.5} />, label: "Organize Devices", desc: "Group by project" },
            { icon: <Rocket size={16} strokeWidth={1.5} />, label: "Run Sessions", desc: "Monitor in real time" },
            { icon: <RefreshCw size={16} strokeWidth={1.5} />, label: "OTA Updates", desc: "Deploy firmware" }
          ].map((item) => (
            <div key={item.label} className="onboarding-bullet">
              <div className="onboarding-bullet-mark">{item.icon}</div>
              <div className="onboarding-bullet-label">{item.label}</div>
              <div className="muted onboarding-bullet-desc">{item.desc}</div>
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
  const { logout, user } = useAuth();

  const hasWorkspaces = workspaces.length > 0;

  const initials = user?.name
    ? user.name
        .split(" ")
        .map((n) => n[0])
        .join("")
        .toUpperCase()
        .slice(0, 2)
    : "?";

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <NavLink className="sidebar-brand" to="/">
          <span className="brand-mark">
            <Activity size={14} strokeWidth={1.5} aria-hidden />
          </span>
          <span>
            <strong>ARGUS</strong>
            <small>Fleet Control</small>
          </span>
        </NavLink>

        {/* Scrollable Navigation links */}
        <div className="sidebar-nav">
          {hasWorkspaces && (
            <div className="workspace-selector-nav">
              <label>Active Workspace</label>
              <div className="workspace-selector-wrap">
                <select
                  value={selectedWorkspaceId}
                  onChange={(e) => setSelectedWorkspaceId(e.target.value)}
                >
                  {workspaces.map(ws => (
                    <option key={ws.id} value={ws.id}>{ws.name}</option>
                  ))}
                </select>
                <Briefcase size={13} className="workspace-selector-icon" aria-hidden />
              </div>
            </div>
          )}

          <NavGroup label="Monitor" links={monitorLinks} />
          <NavGroup label="Control" links={controlLinks} />
          <NavGroup label="Observe" links={observeLinks} />
          <NavGroup label="" links={bottomLinks} />
        </div>

        {/* Fixed Footer with Operator Card & Logout */}
        <div className="sidebar-footer">
          <div className="operator-card">
            <div className="operator-avatar">{initials}</div>
            <div className="operator-meta">
              <strong>{user?.name || "Operator"}</strong>
              <small>{user?.email || "No email"}</small>
            </div>
          </div>
          <button
            onClick={logout}
            className="btn-inverse sidebar-logout"
          >
            <LogOut size={14} strokeWidth={1.5} />
            Logout
          </button>
        </div>
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
