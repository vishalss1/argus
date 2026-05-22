import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Trash2 } from "lucide-react";
import { api } from "../services/api";
import { EmptyState, ErrorState, LoadingRows, PageHeader, Panel, StatusChip } from "../components/ui";
import { useCreateDevice, useDevices, useHeartbeat } from "../hooks/useArgusData";
import { compactID, formatDate, safeJsonParse } from "../lib/format";

export function DevicesPage() {
  const devices = useDevices();
  const create = useCreateDevice();
  const heartbeat = useHeartbeat();
  const [query, setQuery] = useState("");
  const [formError, setFormError] = useState("");

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return devices.data ?? [];
    return (devices.data ?? []).filter((device) =>
      [device.name, device.id, device.type, device.status, device.firmware_version].join(" ").toLowerCase().includes(needle)
    );
  }, [devices.data, query]);

  async function onCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const form = new FormData(event.currentTarget);
    try {
      await create.mutateAsync({
        name: String(form.get("name") || ""),
        type: String(form.get("type") || ""),
        firmware_version: String(form.get("firmware_version") || ""),
        status: String(form.get("status") || ""),
        metadata: safeJsonParse(String(form.get("metadata") || "{}"))
      });
      event.currentTarget.reset();
    } catch (error) {
      setFormError((error as Error).message);
    }
  }

  async function removeDevice(id: string) {
    await api.devices.remove(id);
    await devices.refetch();
  }

  return (
    <>
      <PageHeader
        eyebrow="Device Registry"
        title="Devices"
        description="Create, inspect, heartbeat, and delete real device records from the ARGUS backend."
        actions={<button className="button secondary" onClick={() => void devices.refetch()}><RefreshCw size={15} />Refresh</button>}
      />
      <div className="split">
        <Panel title="Registered Devices" subtitle={`${filtered.length} visible`}>
          <label className="field" style={{ marginBottom: 14 }}>
            <span>Search</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, ID, type, firmware, status" />
          </label>
          {devices.isError ? (
            <ErrorState message={(devices.error as Error).message} onRetry={() => void devices.refetch()} />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Status</th>
                    <th>Firmware</th>
                    <th>Last Seen</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {devices.isLoading && <LoadingRows rows={6} />}
                  {!devices.isLoading && filtered.length === 0 && (
                    <tr><td colSpan={6}><EmptyState title="No devices registered" description="Create the first device using the form on this page." /></td></tr>
                  )}
                  {filtered.map((device) => (
                    <tr key={device.id}>
                      <td><strong>{device.name}</strong><div className="muted mono">{compactID(device.id)}</div></td>
                      <td>{device.type}</td>
                      <td><StatusChip value={device.status} /></td>
                      <td>{device.firmware_version || "Unset"}</td>
                      <td>{formatDate(device.last_seen)}</td>
                      <td>
                        <div className="page-actions">
                          <button className="button compact secondary" onClick={() => heartbeat.mutate({ id: device.id, status: "online" })}>Heartbeat</button>
                          <button className="button compact danger" onClick={() => void removeDevice(device.id)} aria-label={`Delete ${device.name}`}><Trash2 size={14} /></button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <Panel title="Create Device" subtitle="POST /devices">
          <form className="form-grid" onSubmit={onCreate}>
            <label className="field"><span>Name</span><input name="name" required /></label>
            <label className="field"><span>Type</span><input name="type" required /></label>
            <label className="field"><span>Firmware Version</span><input name="firmware_version" /></label>
            <label className="field"><span>Status</span><select name="status" defaultValue="online"><option>online</option><option>offline</option><option>warning</option><option>critical</option></select></label>
            <label className="field full"><span>Metadata JSON</span><textarea name="metadata" defaultValue="{}" /></label>
            {formError && <p className="muted field full">{formError}</p>}
            <button className="button primary" type="submit" disabled={create.isPending}><Plus size={15} />Create Device</button>
          </form>
        </Panel>
      </div>
    </>
  );
}
