import { NavLink, Outlet } from "react-router-dom";
import { Activity, BarChart3 } from "lucide-react";

const productLinks = [
  { to: "/", label: "Overview" },
  { to: "/architecture", label: "Architecture" },
  { to: "/documentation", label: "Docs" },
  { to: "/about", label: "About" }
];

export function ProductLayout() {
  return (
    <div className="site-shell">
      <header className="topbar">
        <NavLink className="brand" to="/">
          <span className="brand-mark">
            <Activity size={16} aria-hidden />
          </span>
          <span>
            <strong>ARGUS</strong>
            <small>Fleet Intelligence Platform</small>
          </span>
        </NavLink>
        <nav className="topnav" aria-label="Primary navigation">
          {productLinks.map((link) => (
            <NavLink key={link.to} to={link.to} end={link.to === "/"}>
              {link.label}
            </NavLink>
          ))}
        </nav>
        <div className="topbar-right">
          <NavLink className="button secondary compact" to="/documentation">
            View Docs
          </NavLink>
          <NavLink className="button primary compact" to="/dashboard">
            <BarChart3 size={15} aria-hidden />
            Dashboard →
          </NavLink>
        </div>
      </header>
      <main>
        <Outlet />
      </main>
    </div>
  );
}
