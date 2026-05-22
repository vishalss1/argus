import { NavLink, Outlet } from "react-router-dom";
import type { LucideIcon } from "lucide-react";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  BookOpen,
  Cpu,
  FileText,
  Gauge,
  Key,
  RadioTower,
  Rocket,
  ScrollText,
  Send,
  Settings,
  Workflow
} from "lucide-react";
import { LiveIndicator, SearchBar } from "../components/ui";

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
  { to: "/observability", label: "Observability", icon: Gauge },
  { to: "/alerts", label: "Alerts", icon: AlertTriangle }
];

const bottomLinks = [
  { to: "/settings", label: "Settings", icon: Settings },
  { to: "/settings", label: "API Keys", icon: Key },
  { to: "/settings", label: "Audit Log", icon: ScrollText },
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
        <div className="operator-card">
          <div>VS</div>
          <span>
            <strong>Vishal Shetagar</strong>
            <small>Admin · Student Plan</small>
          </span>
        </div>
      </aside>
      <div className="app-main">
        <header className="app-topbar">
          <SearchBar placeholder="Search devices..." />
          <LiveIndicator />
        </header>
        <main className="workspace">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
