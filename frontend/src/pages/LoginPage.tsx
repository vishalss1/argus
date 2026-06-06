import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useAuth } from "../context/AuthContext";
import { request } from "../services/http";
import { Briefcase, KeyRound, Mail, ArrowRight, ShieldAlert } from "lucide-react";

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
    <div style={{
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      minHeight: "100vh",
      background: "radial-gradient(circle at top, #1e1b4b, #09090b)",
      padding: "20px"
    }}>
      <div style={{
        maxWidth: 420,
        width: "100%",
        background: "rgba(15, 23, 42, 0.4)",
        backdropFilter: "blur(12px)",
        border: "1px solid rgba(255, 255, 255, 0.05)",
        borderRadius: "16px",
        padding: "40px 32px",
        boxShadow: "0 20px 25px -5px rgba(0,0,0,0.5)"
      }}>
        {/* Brand Header */}
        <div style={{ textAlign: "center", marginBottom: "32px" }}>
          <div style={{
            width: 48,
            height: 48,
            borderRadius: "12px",
            margin: "0 auto 16px",
            background: "linear-gradient(135deg, #6366f1, #8b5cf6)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            boxShadow: "0 0 20px rgba(99, 102, 241, 0.4)"
          }}>
            <ShieldAlert size={24} style={{ color: "#fff" }} />
          </div>
          <h2 style={{ fontSize: "24px", fontWeight: 700, margin: "0 0 6px", letterSpacing: "-0.5px" }}>
            Sign in to ARGUS
          </h2>
          <p className="muted" style={{ fontSize: "14px", margin: 0 }}>
            Enter your credentials or use SSO to access your fleet
          </p>
        </div>

        {error && (
          <div style={{
            background: "rgba(239, 68, 68, 0.1)",
            border: "1px solid rgba(239, 68, 68, 0.2)",
            borderRadius: "8px",
            padding: "12px 16px",
            color: "#f87171",
            fontSize: "13px",
            marginBottom: "24px",
            display: "flex",
            alignItems: "center",
            gap: "10px"
          }}>
            <ShieldAlert size={16} />
            <span>{error}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: "20px" }}>
          {/* Email input */}
          <div className="form-group">
            <label style={{ display: "block", fontSize: "12px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "6px" }}>
              Email Address
            </label>
            <div style={{ position: "relative" }}>
              <input
                type="email"
                required
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                style={{
                  width: "100%",
                  padding: "10px 14px 10px 40px",
                  fontSize: "14px",
                  background: "var(--surface-2)",
                  border: "1px solid var(--line)",
                  borderRadius: "8px",
                  color: "inherit",
                  outline: "none",
                  boxSizing: "border-box"
                }}
              />
              <Mail size={16} style={{ position: "absolute", left: 14, top: "50%", transform: "translateY(-50%)", color: "var(--faint)" }} />
            </div>
          </div>

          {/* Password input */}
          <div className="form-group">
            <label style={{ display: "block", fontSize: "12px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "6px" }}>
              Password
            </label>
            <div style={{ position: "relative" }}>
              <input
                type="password"
                required
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                style={{
                  width: "100%",
                  padding: "10px 14px 10px 40px",
                  fontSize: "14px",
                  background: "var(--surface-2)",
                  border: "1px solid var(--line)",
                  borderRadius: "8px",
                  color: "inherit",
                  outline: "none",
                  boxSizing: "border-box"
                }}
              />
              <KeyRound size={16} style={{ position: "absolute", left: 14, top: "50%", transform: "translateY(-50%)", color: "var(--faint)" }} />
            </div>
          </div>

          <button
            type="submit"
            className="button primary"
            disabled={isLoading}
            style={{
              padding: "12px",
              fontSize: "14px",
              fontWeight: 600,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "8px"
            }}
          >
            {isLoading ? "Signing in..." : <>Sign In <ArrowRight size={16} /></>}
          </button>
        </form>



        <p className="muted" style={{ textAlign: "center", fontSize: "14px", marginTop: "32px", marginBottom: 0 }}>
          Don't have an account?{" "}
          <Link to="/register" style={{ color: "var(--accent)", fontWeight: 500, textDecoration: "none" }}>
            Register here
          </Link>
        </p>
      </div>
    </div>
  );
}
