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
      const data = await request<{ user: any; access_token: string; refresh_token: string }>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ email: email.trim(), password: password.trim() }),
      });
      await login(data.access_token, data.refresh_token);
      navigate("/dashboard");
    } catch (err: any) {
      setError(err.message || "Invalid email or password");
    } finally {
      setIsLoading(false);
    }
  };

  const handleGoogleSignIn = async () => {
    setIsLoading(true);
    setError(null);
    try {
      // Simulate Google OAuth flow by sending a token
      // The backend accepts the id_token, queries tokeninfo, and processes email.
      // For verification, we can send a mock ID token or trigger a mock login.
      const mockIDToken = "mock_google_id_token_for_verification";
      const data = await request<{ user: any; access_token: string; refresh_token: string }>("/auth/login/google", {
        method: "POST",
        body: JSON.stringify({ id_token: mockIDToken }),
      });
      await login(data.access_token, data.refresh_token);
      navigate("/dashboard");
    } catch (err: any) {
      setError(err.message || "Google Authentication failed");
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

        <div style={{ margin: "24px 0", display: "flex", alignItems: "center", justifyContent: "center", gap: "10px" }}>
          <div style={{ height: "1px", background: "var(--line)", flex: 1 }}></div>
          <span style={{ fontSize: "12px", color: "var(--faint)", textTransform: "uppercase" }}>or</span>
          <div style={{ height: "1px", background: "var(--line)", flex: 1 }}></div>
        </div>

        {/* Google Sign In */}
        <button
          onClick={handleGoogleSignIn}
          disabled={isLoading}
          style={{
            width: "100%",
            padding: "11px",
            borderRadius: "8px",
            border: "1px solid var(--line)",
            background: "rgba(255,255,255,0.02)",
            color: "#fff",
            fontSize: "14px",
            fontWeight: 500,
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: "10px",
            transition: "background 0.2s"
          }}
          onMouseEnter={(e) => e.currentTarget.style.background = "rgba(255,255,255,0.06)"}
          onMouseLeave={(e) => e.currentTarget.style.background = "rgba(255,255,255,0.02)"}
        >
          {/* Simple flat Google logo mockup */}
          <svg width="18" height="18" viewBox="0 0 18 18">
            <path d="M17.64 9.2c0-.637-.057-1.251-.164-1.84H9v3.481h4.844c-.209 1.125-.843 2.078-1.797 2.717v2.258h2.91c1.702-1.567 2.683-3.874 2.683-6.615z" fill="#4285F4"/>
            <path d="M9 18c2.43 0 4.467-.806 5.956-2.184l-2.91-2.258c-.806.54-1.837.86-3.046.86-2.344 0-4.328-1.584-5.036-3.711H.957v2.332C2.438 15.983 5.482 18 9 18z" fill="#34A853"/>
            <path d="M3.964 10.707c-.18-.54-.282-1.117-.282-1.707s.102-1.167.282-1.707V4.96H.957C.347 6.173 0 7.549 0 9s.347 2.827.957 4.04l3.007-2.333z" fill="#FBBC05"/>
            <path d="M9 3.58c1.32 0 2.507.454 3.44 1.347l2.58-2.58C13.463.887 11.426 0 9 0 5.482 0 2.438 2.017.957 4.96L3.964 7.29C4.672 5.163 6.656 3.58 9 3.58z" fill="#EA4335"/>
          </svg>
          Google OAuth
        </button>

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
