import { Navigate, Route, Routes, Outlet } from "react-router-dom";
import { AppLayout } from "../layouts/AppLayout";
import { ProductLayout } from "../layouts/ProductLayout";
import { AboutPage } from "../pages/AboutPage";
import { AlertsPage } from "../pages/AlertsPage";
import { ArchitecturePage } from "../pages/ArchitecturePage";
import { CommandsPage } from "../pages/CommandsPage";
import { DashboardPage } from "../pages/DashboardPage";
import { DevicesPage } from "../pages/DevicesPage";
import { DocumentationPage } from "../pages/DocumentationPage";
import { LandingPage } from "../pages/LandingPage";
import { ObservabilityPage } from "../pages/ObservabilityPage";
import { OTAPage } from "../pages/OTAPage";
import { SettingsPage } from "../pages/SettingsPage";
import { WorkspacesPage } from "../pages/WorkspacesPage";
import { SessionDashboardPage } from "../pages/SessionDashboardPage";
import { SessionReportPage } from "../pages/SessionReportPage";
import { TelemetryPage } from "../pages/TelemetryPage";
import AIPage from "../pages/AIPage";
import { LoginPage } from "../pages/LoginPage";
import { RegisterPage } from "../pages/RegisterPage";
import { useAuth } from "../context/AuthContext";

// Protects routes from unauthenticated users
function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        minHeight: "100vh",
        background: "radial-gradient(circle at top, #1e1b4b, #09090b)",
        color: "var(--accent)",
        fontSize: "16px",
        fontWeight: 600
      }}>
        Loading ARGUS...
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  return <Outlet />;
}

// Restricts authenticated users from entering login/register again
function AuthRoute() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) return null;

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  return <Outlet />;
}

export function App() {
  return (
    <Routes>
      {/* Marketing / Landing Public Routes */}
      <Route element={<ProductLayout />}>
        <Route index element={<LandingPage />} />
        <Route path="architecture" element={<ArchitecturePage />} />
        <Route path="documentation" element={<DocumentationPage />} />
        <Route path="about" element={<AboutPage />} />
      </Route>

      {/* Guest Authentication Routes */}
      <Route element={<AuthRoute />}>
        <Route path="login" element={<LoginPage />} />
        <Route path="register" element={<RegisterPage />} />
      </Route>

      {/* Protected Fleet Control Console Routes */}
      <Route element={<ProtectedRoute />}>
        <Route element={<AppLayout />}>
          <Route path="workspaces" element={<WorkspacesPage />} />
          <Route path="sessions/:sessionID" element={<SessionDashboardPage />} />
          <Route path="sessions/:sessionID/report" element={<SessionReportPage />} />
          <Route path="dashboard" element={<DashboardPage />} />
          <Route path="telemetry" element={<TelemetryPage />} />
          <Route path="devices" element={<DevicesPage />} />
          <Route path="ota" element={<OTAPage />} />
          <Route path="commands" element={<CommandsPage />} />
          <Route path="alerts" element={<AlertsPage />} />
          <Route path="observability" element={<ObservabilityPage />} />
          <Route path="ai" element={<AIPage />} />
          <Route path="settings" element={<SettingsPage />} />
        </Route>
      </Route>

      {/* Redirect all unmatched routes */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
