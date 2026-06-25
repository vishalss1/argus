import { Navigate, Route, Routes, Outlet } from "react-router-dom";
import { AppLayout } from "../layouts/AppLayout";
import { ProductLayout } from "../layouts/ProductLayout";
import { AboutPage } from "../pages/AboutPage";
import { DocumentationPage } from "../pages/DocumentationPage";
import { AlertsPage } from "../pages/AlertsPage";
import { CommandsPage } from "../pages/CommandsPage";
import { DevicesPage } from "../pages/DevicesPage";
import { LandingPage } from "../pages/LandingPage";
import { OTAPage } from "../pages/OTAPage";
import { WorkspacesPage } from "../pages/WorkspacesPage";
import { SessionDashboardPage } from "../pages/SessionDashboardPage";
import { SessionReportPage } from "../pages/SessionReportPage";
import { TelemetryPage } from "../pages/TelemetryPage";
import { FleetOverviewPage } from "../pages/FleetOverviewPage";
import { DeviceDetailsPage } from "../pages/DeviceDetailsPage";
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
        background: "#0d1117",
        color: "#8b949e",
        fontSize: "14px",
        fontWeight: 500
      }}>
        Loading...
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
    return <Navigate to="/fleet" replace />;
  }

  return <Outlet />;
}

export function App() {
  return (
    <Routes>
      {/* Marketing / Landing Public Routes */}
      <Route element={<ProductLayout />}>
        <Route index element={<LandingPage />} />
        <Route path="about" element={<AboutPage />} />
        <Route path="docs" element={<DocumentationPage />} />
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
          <Route path="dashboard" element={<Navigate to="/fleet" replace />} />
          <Route path="fleet" element={<FleetOverviewPage />} />
          <Route path="telemetry" element={<TelemetryPage />} />
          <Route path="devices" element={<DevicesPage />} />
          <Route path="devices/:deviceID" element={<DeviceDetailsPage />} />
          <Route path="ota" element={<OTAPage />} />
          <Route path="commands" element={<CommandsPage />} />
          <Route path="automations" element={<AlertsPage />} />
          <Route path="ai" element={<AIPage />} />
        </Route>
      </Route>

      {/* Redirect all unmatched routes */}
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
