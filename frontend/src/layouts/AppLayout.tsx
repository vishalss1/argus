import { NavLink, Outlet } from "react-router-dom";
import {
  Activity,
  AlertTriangle,
  BarChart3,
  Cpu,
  FileText,
  Gauge,
  Home,
  RadioTower,
  Rocket,
  Send,
  Settings,
  Workflow
} from "lucide-react";

const appLinks = [
  { to: "/dashboard", label: "Dashboard", icon: BarChart3 },
  { to: "/devices", label: "Devices", icon: Cpu },
  { to: "/telemetry", label: "Telemetry", icon: RadioTower },
  { to: "/ota", label: "OTA Updates", icon: Rocket },
  { to: "/commands", label: "Commands", icon: Send },
  { to: "/alerts", label: "Alerts", icon: AlertTriangle },
  { to: "/observability", label: "Observability", icon: Gauge },
  { to: "/settings", label: "Settings", icon: Settings }
];

const resourceLinks = [
  { to: "/", label: "Overview", icon: Home },
  { to: "/architecture", label: "Architecture", icon: Workflow },
  { to: "/documentation", label: "Documentation", icon: FileText }
];

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
        <div className="nav-group">
          <span>Operate</span>
          {appLinks.map((link) => {
            const Icon = link.icon;
            return (
              <NavLink key={link.to} to={link.to}>
                <Icon size={16} aria-hidden />
                {link.label}
              </NavLink>
            );
          })}
        </div>
        <div className="nav-group">
          <span>Resources</span>
          {resourceLinks.map((link) => {
            const Icon = link.icon;
            return (
              <NavLink key={link.to} to={link.to}>
                <Icon size={16} aria-hidden />
                {link.label}
              </NavLink>
            );
          })}
        </div>
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
          <span>Distributed IoT Fleet Intelligence Platform</span>
          <NavLink className="button secondary compact" to="/documentation">
            API Docs
          </NavLink>
        </header>
        <main className="workspace">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
