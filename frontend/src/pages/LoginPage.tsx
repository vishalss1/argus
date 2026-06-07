import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { request } from "../services/http";
import { Activity, ArrowRight, ShieldAlert, Mail, KeyRound } from "lucide-react";

export function LoginPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email.trim() || !password.trim()) return;

    setIsLoading(true);
    setError(null);

    try {
      const data = await request<{ user: any; access_token: string }>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: email.trim(), password: password.trim() }),
      });
      await login(data.access_token);
      navigate("/dashboard");
    } catch (err: any) {
      setError(err.message || "Invalid email or password");
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="auth-shell">
      <aside className="auth-aside">
        <Link className="brand" to="/">
          <span className="brand-mark">
            <Activity size={14} strokeWidth={1.5} aria-hidden />
          </span>
          <strong>ARGUS</strong>
        </Link>

        <div>
          <span className="eyebrow">Fleet Operating System</span>
          <h1>Observe every device. Command any fleet.</h1>
          <p>
            ARGUS is the control plane for distributed IoT — real-time telemetry,
            versioned firmware rollouts, and intelligent automation.
          </p>
          <div className="auth-bullets">
            <span>Postgres &middot; TimescaleDB &middot; MQTT</span>
            <span>Sub-second observability</span>
            <span>Open architecture, self-hosted</span>
          </div>
        </div>

        <div className="auth-aside-footer">
          <span>ARGUS / v2</span>
          <Link to="/about" className="t-link">About</Link>
        </div>
      </aside>

      <section className="auth-form">
        <div className="auth-form-inner">
          <span className="auth-eyebrow">Sign in</span>
          <h2>Access your fleet.</h2>
          <p className="sub">Enter your credentials to continue.</p>

          {error && (
            <div className="alert">
              <ShieldAlert size={16} aria-hidden />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label>Email Address</label>
              <div className="auth-input-wrap">
                <Mail size={15} strokeWidth={1.5} className="auth-input-icon" aria-hidden />
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  autoComplete="email"
                />
              </div>
            </div>

            <div className="form-group">
              <label>Password</label>
              <div className="auth-input-wrap">
                <KeyRound size={15} strokeWidth={1.5} className="auth-input-icon" aria-hidden />
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  autoComplete="current-password"
                />
              </div>
            </div>

            <button
              type="submit"
              className="btn-inverse auth-button"
              disabled={isLoading}
            >
              {isLoading ? "Signing in..." : (
                <>
                  Sign In
                  <ArrowRight size={15} strokeWidth={1.5} />
                </>
              )}
            </button>
          </form>

          <p className="footnote">
            Don't have an account? <Link to="/register">Register here</Link>
          </p>
        </div>
      </section>
    </div>
  );
}
