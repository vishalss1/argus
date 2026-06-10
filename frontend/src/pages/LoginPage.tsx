import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { request } from "../services/http";
import { Activity, ArrowRight, ShieldAlert } from "lucide-react";

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
          <p>Secure access to the ARGUS control plane.</p>
          <p>Device management, telemetry, OTA delivery and fleet operations.</p>
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
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                autoComplete="email"
              />
            </div>

            <div className="form-group">
              <label>Password</label>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                autoComplete="current-password"
              />
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
