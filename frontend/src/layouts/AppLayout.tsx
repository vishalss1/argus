import { NavLink, Outlet } from "react-router-dom";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  BookOpen,
  Brain,
  Cpu,
  FileText,
  Gauge,
  RadioTower,
  Rocket,
  Send,
  Settings,
  Workflow
} from "lucide-react";
import { LiveIndicator, SearchBar } from "../components/ui";
import { useHealth } from "../hooks/useArgusData";

const monitorLinks = [
  { to: "/dashboard", label: "Fleet Overview", icon: BarChart3 },
  { to: "/devices", label: "Devices", icon: Cpu },
  { to: "/telemetry", label: "Telemetry", icon: RadioTower }
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

export function AppLayout() {
  const health = useHealth();

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
          <Outlet />
        </main>
      </div>
    </div>
  );
}
