import { FormEvent, useEffect, useMemo, useState } from "react";
import { Plus, RefreshCw, Trash2, Edit2, Save, X } from "lucide-react";
import { api } from "../services/api";
import { CopyableID, EmptyState, ErrorState, LoadingRows, PageHeader, Panel, StatusChip } from "../components/ui";
import { useCreateDevice, useDevices, useUpdateDevice, useAssignDevice } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { compactID, formatDate, safeJsonParse, stringifyJson } from "../lib/format";
import type { Device } from "../types/api";

export function DevicesPage() {
  const { selectedWorkspaceId, workspaceDevices } = useWorkspaceContext();
  const devices = useDevices();
  const create = useCreateDevice();
  const update = useUpdateDevice();
  const assignDevice = useAssignDevice();
  const [query, setQuery] = useState("");
  const [formError, setFormError] = useState("");
  const [editDevice, setEditDevice] = useState<Device | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);

  // Close drawer on Escape key
  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setIsDrawerOpen(false);
        setEditDevice(null);
        setFormError("");
      }
    }
    if (isDrawerOpen) {
      window.addEventListener("keydown", handleKeyDown);
    }
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isDrawerOpen]);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return workspaceDevices;
    return workspaceDevices.filter((device) =>
      [device.name, device.id, device.type, device.status, device.firmware_version].join(" ").toLowerCase().includes(needle) 
    );
  }, [workspaceDevices, query]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      name: String(form.get("name") || ""),
      type: String(form.get("type") || ""),
      firmware_version: String(form.get("firmware_version") || ""),
      metadata: editDevice ? editDevice.metadata : undefined
    };

    try {
      if (editDevice) {
        await update.mutateAsync({ id: editDevice.id, payload });
        setEditDevice(null);
      } else {
        const newDev = await create.mutateAsync(payload);
        if (selectedWorkspaceId && newDev?.id) {
          await assignDevice.mutateAsync({ workspaceID: selectedWorkspaceId, deviceID: newDev.id });
        }
        formElement.reset();
      }
      setIsDrawerOpen(false);
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

      <Panel
        title="Registered Devices"
        subtitle={`${filtered.length} visible`}
        actions={
          <button
            className="button primary"
            onClick={() => {
              setEditDevice(null);
              setFormError("");
              setIsDrawerOpen(true);
            }}
            aria-label="Add Device"
          >
            <Plus size={15} /> Add Device
          </button>
        }
      >
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
                  <tr><td colSpan={6}><EmptyState title="No devices registered" description="Create the first device using the button above." /></td></tr>
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
                        <button
                          className="button compact secondary"
                          onClick={() => {
                            setEditDevice(device);
                            setIsDrawerOpen(true);
                            setFormError("");
                          }}
                          aria-label={`Edit ${device.name}`}
                        >
                          <Edit2 size={14} />
                        </button>
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

      {/* Drawer Overlay & Panel */}
      <div
        className={`drawer-backdrop ${isDrawerOpen ? "open" : ""}`}
        onClick={() => {
          setIsDrawerOpen(false);
          setEditDevice(null);
          setFormError("");
        }}
      >
        <div className="drawer-panel" onClick={(e) => e.stopPropagation()}>
          <div className="drawer-header">
            <h3>{editDevice ? "Update Device" : "Create Device"}</h3>
            <button
              className="close-btn"
              type="button"
              onClick={() => {
                setIsDrawerOpen(false);
                setEditDevice(null);
                setFormError("");
              }}
              aria-label="Close drawer"
            >
              <X size={18} />
            </button>
          </div>
          <form
            onSubmit={onSubmit}
            key={editDevice?.id || "create"}
            style={{ display: "flex", flexDirection: "column", height: "calc(100% - 69px)" }}
          >
            <div className="drawer-body">
              <p className="muted" style={{ fontSize: 13, marginBottom: 20 }}>
                {editDevice
                  ? "Modify attributes for this registered device."
                  : "Add a new virtual or physical node to the fleet registry."}
              </p>
              <div className="form-grid">
                <label className="field full">
                  <span>Name</span>
                  <input name="name" defaultValue={editDevice?.name || ""} required placeholder="e.g. ESP32 Dev Node" />
                </label>
                <label className="field full">
                  <span>Type</span>
                  <input name="type" defaultValue={editDevice?.type || ""} required placeholder="e.g. esp32" />
                </label>
                <label className="field full">
                  <span>Firmware Version</span>
                  <input name="firmware_version" defaultValue={editDevice?.firmware_version || ""} placeholder="e.g. v1.0.0" />
                </label>
                {editDevice && (
                  <div className="field full status-field">
                    <span>Status</span>
                    <StatusChip value={editDevice.status} />
                  </div>
                )}
                {formError && <p className="form-message error field full">{formError}</p>}
              </div>
            </div>
            <div className="drawer-footer">
              <button
                className="button secondary"
                type="button"
                onClick={() => {
                  setIsDrawerOpen(false);
                  setEditDevice(null);
                  setFormError("");
                }}
              >
                Cancel
              </button>
              <button
                className="button primary"
                type="submit"
                disabled={create.isPending || update.isPending}
              >
                {editDevice ? <Save size={15} /> : <Plus size={15} />}
                {editDevice ? "Save Changes" : "Create Device"}
              </button>
            </div>
          </form>
        </div>
      </div>
    </>
  );
}
