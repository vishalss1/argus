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
    <div className="site-shell site-shell--landing">
      <header className="topbar topbar--landing">
        <NavLink className="brand" to="/">
          <span className="brand-mark brand-mark--landing">
            <Activity size={18} strokeWidth={1.6} aria-hidden />
          </span>
          <span>
            <strong>ARGUS</strong>
          </span>
        </NavLink>
        <nav className="topnav topnav--landing" aria-label="Primary navigation">
          {productLinks.map((link) => (
            <NavLink key={link.to} to={link.to} end={link.to === "/"}>
              {link.label}
            </NavLink>
          ))}
        </nav>
        <div className="topbar-right">
          <NavLink className="topbar-cta" to="/dashboard">
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
