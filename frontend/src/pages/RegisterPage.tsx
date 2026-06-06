import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { request } from "../services/http";
import { Mail, KeyRound, User, ArrowRight, ShieldCheck, AlertCircle } from "lucide-react";

export function RegisterPage() {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const navigate = useNavigate();

  // Password requirements warnings
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
            <ShieldCheck size={24} style={{ color: "#fff" }} />
          </div>
          <h2 style={{ fontSize: "24px", fontWeight: 700, margin: "0 0 6px", letterSpacing: "-0.5px" }}>
            Create an Account
          </h2>
          <p className="muted" style={{ fontSize: "14px", margin: 0 }}>
            Register to start managing your fleet of IoT devices
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
            <AlertCircle size={16} />
            <span>{error}</span>
          </div>
        )}

        {success && (
          <div style={{
            background: "rgba(16, 185, 129, 0.1)",
            border: "1px solid rgba(16, 185, 129, 0.2)",
            borderRadius: "8px",
            padding: "12px 16px",
            color: "#34d399",
            fontSize: "13px",
            marginBottom: "24px",
            display: "flex",
            alignItems: "center",
            gap: "10px"
          }}>
            <ShieldCheck size={16} />
            <span>Registration successful! Redirecting to login...</span>
          </div>
        )}

        <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
          {/* Full Name */}
          <div className="form-group">
            <label style={{ display: "block", fontSize: "12px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "6px" }}>
              Full Name
            </label>
            <div style={{ position: "relative" }}>
              <input
                type="text"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Jane Doe"
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
              <User size={16} style={{ position: "absolute", left: 14, top: "50%", transform: "translateY(-50%)", color: "var(--faint)" }} />
            </div>
          </div>

          {/* Email address */}
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

          {/* Password */}
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
                placeholder="Minimum 8 characters"
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
            {isPasswordShort && (
              <span style={{ display: "block", fontSize: "11px", color: "#f87171", marginTop: "4px" }}>
                Password must be at least 8 characters long.
              </span>
            )}
            {isPasswordTooLong && (
              <span style={{ display: "block", fontSize: "11px", color: "#f87171", marginTop: "4px" }}>
                Password cannot exceed 128 characters.
              </span>
            )}
          </div>

          {/* Confirm Password */}
          <div className="form-group">
            <label style={{ display: "block", fontSize: "12px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "6px" }}>
              Confirm Password
            </label>
            <div style={{ position: "relative" }}>
              <input
                type="password"
                required
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm password"
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
            {!passwordsMatch && confirmPassword.length > 0 && (
              <span style={{ display: "block", fontSize: "11px", color: "#f87171", marginTop: "4px" }}>
                Passwords do not match.
              </span>
            )}
          </div>

          <button
            type="submit"
            className="button primary"
            disabled={isLoading || isPasswordShort || isPasswordTooLong || !passwordsMatch}
            style={{
              padding: "12px",
              fontSize: "14px",
              fontWeight: 600,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              gap: "8px",
              marginTop: "8px"
            }}
          >
            {isLoading ? "Registering..." : <>Register <ArrowRight size={16} /></>}
          </button>
        </form>

        <p className="muted" style={{ textAlign: "center", fontSize: "14px", marginTop: "32px", marginBottom: 0 }}>
          Already have an account?{" "}
          <Link to="/login" style={{ color: "var(--accent)", fontWeight: 500, textDecoration: "none" }}>
            Sign In here
          </Link>
        </p>
      </div>
    </div>
  );
}
