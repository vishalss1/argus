import { PageHeader, Panel, StatusChip } from "../components/ui";
import { API_BASE_URL } from "../services/http";

export function SettingsPage() {
  return (
    <>
      <PageHeader eyebrow="Configuration" title="Settings" description="Runtime frontend configuration and backend integration status. Authentication is not exposed by the current backend." />
      <Panel title="Frontend Runtime">
        <div className="settings-list">
          <div className="settings-row"><span><strong>API Base URL</strong><p className="muted">Set with VITE_API_BASE_URL or served through the Vite /api proxy during development.</p></span><span className="mono">{API_BASE_URL}</span></div>
          <div className="settings-row"><span><strong>Auth Session</strong><p className="muted">No auth or session endpoints exist in the current ARGUS API.</p></span><StatusChip value="not configured" /></div>
          <div className="settings-row"><span><strong>Data Policy</strong><p className="muted">The UI renders backend data only and uses empty states when records are absent.</p></span><StatusChip value="enabled" /></div>
        </div>
      </Panel>
    </>
  );
}
