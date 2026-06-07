import { NavLink, Outlet } from "react-router-dom";
import { Activity, ArrowRight } from "lucide-react";

const productLinks = [
  { to: "/", label: "Platform" },
  { to: "/architecture", label: "Architecture" },
  { to: "/documentation", label: "Documentation" },
  { to: "/about", label: "About" }
];

export function ProductLayout() {
  return (
    <div className="site-shell">
      <header className="topbar">
        <NavLink className="brand" to="/">
          <span className="brand-mark">
            <Activity size={20} aria-hidden />
          </span>
          <span>
            <strong>ARGUS</strong>
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
          <NavLink className="btn-inverse" to="/dashboard">
            Dashboard <ArrowRight size={13} aria-hidden />
          </NavLink>
        </div>
      </header>
      <main>
        <Outlet />
      </main>
    </div>
  );
}
