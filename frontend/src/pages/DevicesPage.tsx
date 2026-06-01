import { FormEvent, useMemo, useState } from "react";
import { Plus, RefreshCw, Trash2, Edit2, Save, X } from "lucide-react";
import { api } from "../services/api";
import { CopyableID, EmptyState, ErrorState, LoadingRows, PageHeader, Panel, StatusChip } from "../components/ui";
import { useCreateDevice, useDevices, useUpdateDevice } from "../hooks/useArgusData";
import { compactID, formatDate, safeJsonParse, stringifyJson } from "../lib/format";
import type { Device } from "../types/api";

export function DevicesPage() {
  const devices = useDevices();
  const create = useCreateDevice();
  const update = useUpdateDevice();
  const [query, setQuery] = useState("");
  const [formError, setFormError] = useState("");
  const [editDevice, setEditDevice] = useState<Device | null>(null);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return devices.data ?? [];
    return (devices.data ?? []).filter((device) =>
      [device.name, device.id, device.type, device.status, device.firmware_version].join(" ").toLowerCase().includes(needle) 
    );
  }, [devices.data, query]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      id: editDevice ? undefined : String(form.get("id") || "").trim() || undefined,
      name: String(form.get("name") || ""),
      type: String(form.get("type") || ""),
      firmware_version: String(form.get("firmware_version") || ""),
      metadata: safeJsonParse(String(form.get("metadata") || "{}"))
    };

    try {
      if (editDevice) {
        await update.mutateAsync({ id: editDevice.id, payload });
        setEditDevice(null);
      } else {
        await create.mutateAsync(payload);
        formElement.reset();
      }
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
        description="Create, inspect, update, and delete real device records from the ARGUS backend. Device status is reported by backend heartbeat state."
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
                      <td><strong>{device.name}</strong><div><CopyableID id={device.id} /></div></td>
                      <td>{device.type}</td>
                      <td><StatusChip value={device.status} /></td>
                      <td>{device.firmware_version || "Unset"}</td>
                      <td>{formatDate(device.last_seen)}</td>
                      <td>
                        <div className="page-actions">
                          <button className="button compact secondary" onClick={() => setEditDevice(device)} aria-label={`Edit ${device.name}`}><Edit2 size={14} /></button>
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
        <Panel title={editDevice ? "Update Device" : "Create Device"} subtitle={editDevice ? "Modify device attributes and configuration metadata" : "Add a new physical or virtual node to the registry"}>
          <form className="form-grid" onSubmit={onSubmit} key={editDevice?.id || "create"}>
            {!editDevice && <label className="field full"><span>Device UUID</span><input name="id" placeholder="372fa1f0-2d4f-44ff-8e6c-ced30602e7d5" /></label>}
            <label className="field"><span>Name</span><input name="name" defaultValue={editDevice?.name || ""} required /></label>
            <label className="field"><span>Type</span><input name="type" defaultValue={editDevice?.type || ""} required /></label>
            <label className="field"><span>Firmware Version</span><input name="firmware_version" defaultValue={editDevice?.firmware_version || ""} /></label>
            {editDevice && <div className="field status-field"><span>Status</span><StatusChip value={editDevice.status} /></div>}
            <label className="field full"><span>Metadata JSON</span><textarea name="metadata" defaultValue={editDevice ? stringifyJson(editDevice.metadata) : "{}"} /></label>   
            {formError && <p className="muted field full">{formError}</p>}
            <div className="field full page-actions" style={{ marginTop: "1rem" }}>
              <button className="button primary" type="submit" disabled={create.isPending || update.isPending}>
                {editDevice ? <Save size={15} /> : <Plus size={15} />}
                {editDevice ? "Save Changes" : "Create Device"}
              </button>
              {editDevice && (
                <button className="button secondary" type="button" onClick={() => setEditDevice(null)}>
                  <X size={15} /> Cancel
                </button>
              )}
            </div>
          </form>
        </Panel>
      </div>
    </>
  );
}
