import { Navigate, Route, Routes } from "react-router-dom";
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
import { ShadowPage } from "../pages/ShadowPage";
import { TelemetryPage } from "../pages/TelemetryPage";
import AIPage from "../pages/AIPage";

export function App() {
  return (
    <Routes>
      <Route element={<ProductLayout />}>
        <Route index element={<LandingPage />} />
        <Route path="architecture" element={<ArchitecturePage />} />
        <Route path="documentation" element={<DocumentationPage />} />
        <Route path="about" element={<AboutPage />} />
      </Route>
      <Route element={<AppLayout />}>
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="devices" element={<DevicesPage />} />
        <Route path="shadow" element={<ShadowPage />} />
        <Route path="telemetry" element={<TelemetryPage />} />
        <Route path="ota" element={<OTAPage />} />
        <Route path="commands" element={<CommandsPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="observability" element={<ObservabilityPage />} />
        <Route path="ai" element={<AIPage />} />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
