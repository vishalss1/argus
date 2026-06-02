import { FormEvent, useEffect, useState } from "react";
import { Save } from "lucide-react";
import { EmptyState, ErrorState, PageHeader, Panel, SelectField, StatusChip } from "../components/ui";
import { useDevices, useShadow } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { formatDate, safeJsonParse, stringifyJson } from "../lib/format";
import { api } from "../services/api";

export function ShadowPage() {
  const { workspaceDevices } = useWorkspaceContext();
  const [deviceID, setDeviceID] = useState("");
  const shadow = useShadow(deviceID);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!deviceID && workspaceDevices[0]) setDeviceID(workspaceDevices[0].id);
  }, [deviceID, workspaceDevices]);

  async function updateDesired(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await api.shadows.updateDesired(deviceID, safeJsonParse(String(form.get("state") || "{}")));
      await shadow.refetch();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function updateReported(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await api.shadows.updateReported(deviceID, safeJsonParse(String(form.get("state") || "{}")));
      await shadow.refetch();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <>
      <PageHeader eyebrow="Device State" title="Device Shadow" description="Manage desired and reported device state (Device Twins)." />
      <div className="split">
        <Panel title="Shadow State" subtitle="Synchronized Twin state version logs">
          <div className="field" style={{ marginBottom: 14 }}>
            <SelectField label="Device" value={deviceID} onChange={setDeviceID}>
              <option value="">Select a device</option>
              {workspaceDevices.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}
            </SelectField>
          </div>
          {!deviceID ? (
            <EmptyState title="No device selected" description="Shadow state is scoped to a registered device." />
          ) : shadow.isError ? (
            <ErrorState message={(shadow.error as Error).message} onRetry={() => void shadow.refetch()} />
          ) : shadow.isLoading ? (
            <p>Loading shadow state...</p>
          ) : shadow.data ? (
            <div>
              <div style={{ marginBottom: 14, display: "flex", gap: 12 }}>
                <div><strong>Version:</strong> {shadow.data.version}</div>
                <div><strong>Drift:</strong> <StatusChip value={shadow.data.drift ? "warning" : "online"} /> {shadow.data.drift ? "Yes" : "No"}</div>
                <div><strong>Updated:</strong> {formatDate(shadow.data.updated_at)}</div>
              </div>
              <div className="grid">
                <div>
                  <h4>Desired State</h4>
                  <pre className="code-block">{stringifyJson(shadow.data.desired)}</pre>
                </div>
                <div>
                  <h4>Reported State</h4>
                  <pre className="code-block">{stringifyJson(shadow.data.reported)}</pre>
                </div>
              </div>
            </div>
          ) : (
            <EmptyState title="No shadow found" description="Initialize the shadow by updating the desired or reported state." />
          )}
        </Panel>
        <div className="grid">
          <Panel title="Update Desired State" subtitle="Set target configurations for the device to pull">
            <form className="form-grid" onSubmit={updateDesired}>
              <label className="field full"><span>Desired State JSON</span><textarea name="state" defaultValue={shadow.data ? stringifyJson(shadow.data.desired) : "{}"} required /></label>
              <button className="button primary" type="submit" disabled={!deviceID}><Save size={15} /> Update Desired</button>
            </form>
          </Panel>
          <Panel title="Update Reported State" subtitle="Simulate state feedback reported from the device firmware">
            <form className="form-grid" onSubmit={updateReported}>
              <label className="field full"><span>Reported State JSON</span><textarea name="state" defaultValue={shadow.data ? stringifyJson(shadow.data.reported) : "{}"} required /></label>
              <button className="button secondary" type="submit" disabled={!deviceID}><Save size={15} /> Update Reported</button>
            </form>
            {error && <p className="muted field full" style={{ marginTop: 14 }}>{error}</p>}
          </Panel>
        </div>
      </div>
    </>
  );
}