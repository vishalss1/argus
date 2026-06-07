import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { request } from "../services/http";
import { Activity, ArrowRight, ShieldCheck, AlertCircle, Mail, KeyRound, User } from "lucide-react";

export function RegisterPage() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const navigate = useNavigate();

  const isPasswordShort = password.length > 0 && password.length < 8;
  const isPasswordTooLong = password.length > 128;
  const passwordsMatch = password === confirmPassword;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !email.trim() || !password.trim()) return;

    if (password.length < 8 || password.length > 128) {
      setError("Password must be between 8 and 128 characters");
      return;
    }

    if (!passwordsMatch) {
      setError("Passwords do not match");
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      await request("/auth/register", {
        method: "POST",
        body: JSON.stringify({
          name: name.trim(),
          email: email.trim(),
          password: password.trim()
        })
      });
      setSuccess(true);
      setTimeout(() => {
        navigate("/login");
      }, 2000);
    } catch (err: any) {
      setError(err.message || "Registration failed. Try again.");
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
          <span className="eyebrow">Create your account</span>
          <h1>Start operating a fleet in minutes.</h1>
          <p>
            Register an operator account to provision workspaces, register devices,
            and ship firmware updates with confidence.
          </p>
          <div className="auth-bullets">
            <span>Workspaces &middot; Sessions &middot; OTA</span>
            <span>AI-assisted anomaly correlation</span>
            <span>Self-hosted, no vendor lock-in</span>
          </div>
        </div>

        <div className="auth-aside-footer">
          <span>ARGUS / v2</span>
          <Link to="/about" className="t-link">About</Link>
        </div>
      </aside>

      <section className="auth-form">
        <div className="auth-form-inner">
          <span className="auth-eyebrow">Register</span>
          <h2>Create an account.</h2>
          <p className="sub">Set up an operator profile to manage your fleet.</p>

          {error && (
            <div className="alert">
              <AlertCircle size={16} aria-hidden />
              <span>{error}</span>
            </div>
          )}

          {success && (
            <div className="alert">
              <ShieldCheck size={16} aria-hidden />
              <span>Registration successful. Redirecting to login…</span>
            </div>
          )}

          <form onSubmit={handleSubmit}>
            <div className="form-group">
              <label>Full Name</label>
              <div className="auth-input-wrap">
                <User size={15} strokeWidth={1.5} className="auth-input-icon" aria-hidden />
                <input
                  type="text"
                  required
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Jane Doe"
                  autoComplete="name"
                />
              </div>
            </div>

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
                  placeholder="Minimum 8 characters"
                  autoComplete="new-password"
                />
              </div>
              {isPasswordShort && (
                <span className="input-error">Password must be at least 8 characters.</span>
              )}
              {isPasswordTooLong && (
                <span className="input-error">Password cannot exceed 128 characters.</span>
              )}
            </div>

            <div className="form-group">
              <label>Confirm Password</label>
              <div className="auth-input-wrap">
                <KeyRound size={15} strokeWidth={1.5} className="auth-input-icon" aria-hidden />
                <input
                  type="password"
                  required
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  placeholder="Confirm password"
                  autoComplete="new-password"
                />
              </div>
              {!passwordsMatch && confirmPassword.length > 0 && (
                <span className="input-error">Passwords do not match.</span>
              )}
            </div>

            <button
              type="submit"
              className="btn-inverse auth-button"
              disabled={isLoading || isPasswordShort || isPasswordTooLong || !passwordsMatch}
            >
              {isLoading ? "Registering..." : (
                <>
                  Register
                  <ArrowRight size={15} strokeWidth={1.5} />
                </>
              )}
            </button>
          </form>

          <p className="footnote">
            Already have an account? <Link to="/login">Sign in here</Link>
          </p>
        </div>
      </section>
    </div>
  );
}
