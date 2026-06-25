import { FormEvent, useEffect, useMemo, useState } from "react";
import { Plus, RefreshCw, Trash2, Edit2, Save, X, Copy, Check, Wifi, Download } from "lucide-react";
import { api } from "../services/api";
import { CopyableID, EmptyState, ErrorState, LoadingRows, PageHeader, Modal } from "../components/ui";
import { useCreateDevice, useDevices, useUpdateDevice, useAlerts, useAllDeployments, useWorkspaces } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { Link } from "react-router-dom";
import { formatDate } from "../lib/format";
import type { Device } from "../types/api";

function CredentialCopyButton({ value }: { value: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      setCopied(false);
    }
  }

  return (
    <span className="copy-id-wrap">
      <button
        className="copy-id"
        type="button"
        onClick={handleCopy}
        aria-label="Copy to clipboard"
        style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", padding: "4px" }}
      >
        {copied ? <Check size={12} /> : <Copy size={12} />}
      </button>
      <span
        className="copy-id-tooltip"
        role="tooltip"
        style={copied ? { display: "inline-flex" } : undefined}
      >
        {copied ? (
          <span style={{ display: "inline-flex", alignItems: "center", gap: "4px" }}>
            <Check size={12} aria-hidden /> Copied
          </span>
        ) : (
          <span style={{ display: "inline-flex", alignItems: "center", gap: "4px" }}>
            <Copy size={12} aria-hidden /> Copy
          </span>
        )}
      </span>
    </span>
  );
}

export function DevicesPage() {
  const { selectedWorkspaceId, workspaceDevices } = useWorkspaceContext();
  const devices = useDevices();
  const create = useCreateDevice();
  const update = useUpdateDevice();
  const [query, setQuery] = useState("");
  const [formError, setFormError] = useState("");
  const [editDevice, setEditDevice] = useState<Device | null>(null);
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [createdCredentials, setCreatedCredentials] = useState<{ id: string; apiKey: string } | null>(null);
  const [hasDownloaded, setHasDownloaded] = useState(false);
  const [firmwareFileContent, setFirmwareFileContent] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"overview" | "telemetry" | "settings">("overview");

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

  const alerts = useAlerts();
  const deployments = useAllDeployments();
  const workspaces = useWorkspaces();

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    let result = workspaceDevices;
    if (needle) {
      result = workspaceDevices.filter((device) =>
        [device.name, device.id, device.type, device.status, device.firmware_version].join(" ").toLowerCase().includes(needle)
      );
    }
    // Offline devices sort to top — operators care about broken devices first.
    return result.sort((a, b) => {
      const aOffline = a.status === "offline" || a.status === "critical" ? 1 : 0;
      const bOffline = b.status === "offline" || b.status === "critical" ? 1 : 0;
      if (aOffline !== bOffline) return bOffline - aOffline;
      return a.name.localeCompare(b.name);
    });
  }, [workspaceDevices, query]);

  async function onCreateSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    const payload = {
      name: String(form.get("name") || ""),
      type: String(form.get("type") || ""),
      firmware_version: String(form.get("firmware_version") || ""),
    };
    try {
      const fileContent = (await create.mutateAsync(payload)) as unknown as string;
      const deviceIdMatch = fileContent.match(/ARGUS_DEVICE_ID(?:\[\])?\s*(?:=|)\s*"([^"]+)"/);
      const apiKeyMatch = fileContent.match(/ARGUS_API_KEY(?:\[\])?\s*(?:=|)\s*"([^"]+)"/);
      const deviceId = deviceIdMatch ? deviceIdMatch[1] : "";
      const apiKey = apiKeyMatch ? apiKeyMatch[1] : "";
      formElement.reset();
      setIsCreateModalOpen(false);
      if (deviceId && apiKey) {
        setFirmwareFileContent(fileContent);
        setCreatedCredentials({ id: deviceId, apiKey: apiKey });
      }
    } catch (error) {
      setFormError((error as Error).message);
    }
  }

  async function onEditSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const form = new FormData(event.currentTarget);
    const payload = {
      name: String(form.get("name") || ""),
      type: String(form.get("type") || ""),
      firmware_version: String(form.get("firmware_version") || ""),
      metadata: editDevice ? editDevice.metadata : undefined,
    };
    try {
      if (editDevice) {
        await update.mutateAsync({ id: editDevice.id, payload });
        setEditDevice(null);
        setIsDrawerOpen(false);
      }
    } catch (error) {
      setFormError((error as Error).message);
    }
  }

  function triggerFirmwareDownload() {
    if (!firmwareFileContent || !createdCredentials) return;
    const blob = new Blob([firmwareFileContent], { type: "text/plain" });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `firmware_${createdCredentials.id}.ino`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
    setHasDownloaded(true);
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

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "24px" }}>
        <label className="field" style={{ flex: 1, maxWidth: "400px" }}>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search by name, ID, type, or firmware..." />
        </label>
        <button
          className="button primary"
          onClick={() => { setFormError(""); setIsCreateModalOpen(true); }}
          aria-label="Add Device"
        >
          <Plus size={15} /> Add Device
        </button>
      </div>

      {devices.isError ? (
        <ErrorState message={(devices.error as Error).message} onRetry={() => void devices.refetch()} />
      ) : (
        <div className="table-wrapper" style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", overflow: "hidden" }}>
          <table className="data-table" style={{ width: "100%", borderCollapse: "collapse", fontSize: 13 }}>
            <thead>
              <tr style={{ background: "var(--surface)", borderBottom: "1px solid var(--border)", textAlign: "left", color: "var(--text-muted)" }}>
                <th style={{ padding: "12px 16px", width: 40 }}>Status</th>
                <th style={{ padding: "12px 16px" }}>Device</th>
                <th style={{ padding: "12px 16px" }}>Firmware</th>
                <th style={{ padding: "12px 16px" }}>Last Seen</th>
                <th style={{ padding: "12px 16px" }}>RSSI</th>
                <th style={{ padding: "12px 16px", textAlign: "center" }}>Alerts</th>
                <th style={{ padding: "12px 16px" }}>Deployment</th>
                <th style={{ padding: "12px 16px" }}>Workspace</th>
                <th style={{ padding: "12px 16px", textAlign: "right" }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {devices.isLoading && (
                <tr>
                  <td colSpan={9} style={{ padding: "24px" }}><LoadingRows rows={6} /></td>
                </tr>
              )}
              {!devices.isLoading && filtered.length === 0 && (
                <tr>
                  <td colSpan={9} style={{ padding: "24px", textAlign: "center" }}>
                    <EmptyState title="No devices registered" description="Create the first device using the button above." />
                  </td>
                </tr>
              )}
              {filtered.map((device) => {
                const deviceAlerts = (alerts.data ?? []).filter(a => a.device_id === device.id);
                const deviceDeployments = (deployments.data ?? []).filter(d => d.device_id === device.id).sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
                const lastDeploy = deviceDeployments[0];
                const workspaceName = workspaces.data?.find(w => w.id === selectedWorkspaceId)?.name ?? "Unknown";

                return (
                  <tr key={device.id} style={{ borderBottom: "1px solid var(--border)" }}>
                    <td style={{ padding: "12px 16px", textAlign: "center" }}>
                      <div style={{ width: 10, height: 10, borderRadius: "50%", background: device.status === "online" ? "var(--success)" : device.status === "warning" ? "var(--warning)" : device.status === "critical" ? "var(--danger)" : "var(--text-muted)", margin: "0 auto" }} />
                    </td>
                    <td style={{ padding: "12px 16px" }}>
                      <Link to={`/devices/${device.id}`} style={{ fontWeight: 500, color: "var(--text-primary)", textDecoration: "none" }}>{device.name}</Link>
                      <div style={{ fontSize: 11, color: "var(--text-muted)", fontFamily: "var(--font-mono)", marginTop: 4 }}>{device.id.slice(0, 8)}</div>
                    </td>
                    <td style={{ padding: "12px 16px", fontFamily: "var(--font-mono)" }}>
                      v{device.firmware_version || "unset"}
                    </td>
                    <td style={{ padding: "12px 16px", color: "var(--text-muted)" }}>
                      {formatDate(device.last_seen)}
                    </td>
                    <td style={{ padding: "12px 16px" }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 6, color: "var(--text-muted)" }}>
                        <Wifi size={12} /> -65 dBm
                      </div>
                    </td>
                    <td style={{ padding: "12px 16px", textAlign: "center" }}>
                      {deviceAlerts.length > 0 ? (
                        <span style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", minWidth: 20, height: 20, borderRadius: 10, background: "var(--danger)", color: "#fff", fontSize: 11, fontWeight: 600 }}>{deviceAlerts.length}</span>
                      ) : (
                        <span style={{ color: "var(--text-muted)" }}>0</span>
                      )}
                    </td>
                    <td style={{ padding: "12px 16px" }}>
                      {lastDeploy ? (
                        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                          {lastDeploy.status === "completed" || lastDeploy.status === "success" ? <Check size={12} color="var(--success)" /> : <RefreshCw size={12} color="var(--vercel-cyan)" />}
                          <span style={{ color: lastDeploy.status === "completed" || lastDeploy.status === "success" ? "var(--success)" : "var(--vercel-cyan)" }}>{lastDeploy.status}</span>
                        </div>
                      ) : (
                        <span style={{ color: "var(--text-muted)" }}>None</span>
                      )}
                    </td>
                    <td style={{ padding: "12px 16px", color: "var(--text-muted)" }}>
                      {workspaceName}
                    </td>
                    <td style={{ padding: "12px 16px", textAlign: "right" }}>
                      <div className="page-actions" style={{ justifyContent: "flex-end" }}>
                        <button
                          className="button compact secondary"
                          onClick={() => { setEditDevice(device); setIsDrawerOpen(true); setFormError(""); }}
                          aria-label={`Edit ${device.name}`}
                        >
                          <Edit2 size={14} />
                        </button>
                        <button className="button compact danger" onClick={() => void removeDevice(device.id)} aria-label={`Delete ${device.name}`}><Trash2 size={14} /></button>
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* ── Drawer — edit device only ── */}
      <div
        className={`drawer-backdrop ${isDrawerOpen ? "open" : ""}`}
        onClick={() => { setIsDrawerOpen(false); setEditDevice(null); setFormError(""); }}
      >
        <div className="drawer-panel" onClick={(e) => e.stopPropagation()}>
          <div className="drawer-header" style={{ display: "flex", flexDirection: "column", alignItems: "flex-start", gap: 16, paddingBottom: 0 }}>
            <div style={{ display: "flex", justifyContent: "space-between", width: "100%", alignItems: "flex-start" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                {editDevice && <div style={{ width: 12, height: 12, borderRadius: "50%", background: editDevice.status === "online" ? "var(--success)" : editDevice.status === "warning" ? "var(--warning)" : editDevice.status === "critical" ? "var(--danger)" : "var(--text-muted)" }} />}
                <div>
                  <h3 style={{ fontSize: 28, margin: "0 0 4px", fontWeight: 600 }}>{editDevice?.name ?? ""}</h3>
                  {editDevice && (
                    <div style={{ display: "flex", alignItems: "center", gap: 12, fontSize: 13, color: "var(--text-muted)", fontFamily: "var(--font-mono)" }}>
                      <CopyableID id={editDevice.id} length={12} />
                      <span style={{ padding: "2px 6px", background: "var(--surface-2)", borderRadius: "var(--radius-sm)", border: "1px solid var(--border)", fontSize: 11, letterSpacing: "0.05em", textTransform: "uppercase" }}>{editDevice.type}</span>
                    </div>
                  )}
                </div>
              </div>
              <button
                className="close-btn"
                type="button"
                onClick={() => { setIsDrawerOpen(false); setEditDevice(null); setFormError(""); }}
                aria-label="Close drawer"
              >
                <X size={20} />
              </button>
            </div>

            {editDevice && (
              <div className="drawer-tabs" style={{ display: "flex", gap: 32, marginTop: 8 }}>
                {(["overview", "telemetry", "settings"] as const).map(tab => (
                  <button
                    key={tab}
                    className={`drawer-tab ${activeTab === tab ? "active" : ""}`}
                    onClick={() => setActiveTab(tab)}
                    style={{
                      background: "transparent", border: "none", cursor: "pointer",
                      padding: "0 0 12px 0", fontSize: 14, fontWeight: 500, textTransform: "capitalize",
                      color: activeTab === tab ? "var(--text-primary)" : "var(--text-muted)",
                      borderBottom: `2px solid ${activeTab === tab ? "var(--text-primary)" : "transparent"}`,
                      transition: "all 150ms ease",
                    }}
                  >
                    {tab}
                  </button>
                ))}
              </div>
            )}
          </div>

          {editDevice && activeTab !== "settings" ? (
            <div className="drawer-body-wrap" style={{ display: "flex", flexDirection: "column", height: "calc(100% - 120px)" }}>
              <div className="drawer-body" style={{ background: "var(--background)" }}>
                {activeTab === "overview" && (
                  <div style={{ display: "flex", flexDirection: "column", gap: 32 }}>
                    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 24, paddingBottom: 24, borderBottom: "1px solid var(--border)" }}>
                      <div>
                        <h4 style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--text-muted)", margin: "0 0 8px", fontFamily: "var(--font-mono)" }}>Last Seen</h4>
                        <div style={{ fontSize: 16, color: "var(--text-primary)" }}>{formatDate(editDevice.last_seen)}</div>
                      </div>
                      <div>
                        <h4 style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.06em", color: "var(--text-muted)", margin: "0 0 8px", fontFamily: "var(--font-mono)" }}>Firmware</h4>
                        <div style={{ fontSize: 16, color: "var(--text-primary)", fontFamily: "var(--font-mono)" }}>{editDevice.firmware_version || "Unset"}</div>
                      </div>
                    </div>
                    <div>
                      <h3 style={{ fontSize: 18, fontWeight: 500, margin: "0 0 16px" }}>Recent Activity</h3>
                      <EmptyState title="No recent activity" description="This device hasn't reported any notable events." />
                    </div>
                  </div>
                )}
                {activeTab === "telemetry" && (
                  <EmptyState title="Telemetry Unavailable" description="This view will stream live metrics when connected." />
                )}
              </div>
            </div>
          ) : (
            <form
              onSubmit={onEditSubmit}
              key={editDevice?.id ?? "edit"}
              style={{ display: "flex", flexDirection: "column", height: "calc(100% - 120px)" }}
            >
              <div className="drawer-body" style={{ maxWidth: 600 }}>
                <h3 style={{ fontSize: 18, fontWeight: 500, margin: "0 0 8px" }}>General Settings</h3>
                <p className="muted" style={{ fontSize: 14, marginBottom: 32 }}>
                  Modify identifying attributes for this registered device.
                </p>
                <div className="form-grid">
                  <label className="field full">
                    <span style={{ fontWeight: 500 }}>Device Name</span>
                    <input name="name" defaultValue={editDevice?.name ?? ""} required placeholder="e.g. ESP32 Dev Node" style={{ padding: 12 }} />
                  </label>
                  <label className="field full">
                    <span style={{ fontWeight: 500 }}>Device Type</span>
                    <input name="type" defaultValue={editDevice?.type ?? ""} required placeholder="e.g. esp32" style={{ padding: 12 }} />
                  </label>
                  <label className="field full">
                    <span style={{ fontWeight: 500 }}>Expected Firmware Version</span>
                    <input name="firmware_version" defaultValue={editDevice?.firmware_version ?? ""} placeholder="e.g. v1.0.0" style={{ padding: 12 }} />
                  </label>
                  {formError && <p className="form-message error field full" style={{ padding: 16 }}>{formError}</p>}
                </div>
              </div>
              <div className="drawer-footer">
                <button className="button secondary" type="button" onClick={() => { setIsDrawerOpen(false); setEditDevice(null); setFormError(""); }}>Cancel</button>
                <button className="button primary" type="submit" disabled={update.isPending}>
                  <Save size={15} /> Save Settings
                </button>
              </div>
            </form>
          )}
        </div>
      </div>

      {/* ── Modal — Create Device ── */}
      <Modal isOpen={isCreateModalOpen} onClose={() => { setIsCreateModalOpen(false); setFormError(""); }} title="Create Device">
        <form className="form-grid" onSubmit={onCreateSubmit}>
          <label className="field full">
            <span>Device Name</span>
            <input name="name" required placeholder="e.g. ESP32 Dev Node" />
          </label>
          <label className="field full">
            <span>Device Type</span>
            <input name="type" required placeholder="e.g. esp32" />
          </label>
          <label className="field full">
            <span>Expected Firmware Version</span>
            <input name="firmware_version" placeholder="e.g. v1.0.0" />
          </label>
          {formError && <div className="form-message error field full">{formError}</div>}
          <div className="modal-actions">
            <button type="button" className="button secondary" onClick={() => { setIsCreateModalOpen(false); setFormError(""); }}>Cancel</button>
            <button className="button primary" type="submit" disabled={create.isPending}>
              <Plus size={14} />
              {create.isPending ? "Creating..." : "Create Device"}
            </button>
          </div>
        </form>
      </Modal>

      {/* ── Modal — Credentials after creation ── */}
      <Modal isOpen={!!createdCredentials} onClose={() => {}} title="Device Created">
        <div className="form-grid">
          <p className="muted field full" style={{ fontSize: 13, margin: 0 }}>
            Your device has been created. Copy the credentials below, then download the pre-filled firmware sketch.
          </p>

          <div className="field full">
            <span>Device ID</span>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 4 }}>
              <span style={{ fontFamily: "var(--font-mono)", fontSize: 13, color: "var(--text-primary)", wordBreak: "break-all" }}>{createdCredentials?.id}</span>
              <CredentialCopyButton value={createdCredentials?.id ?? ""} />
            </div>
          </div>

          <div className="field full">
            <span>API Key</span>
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 4 }}>
              <span style={{ fontFamily: "var(--font-mono)", fontSize: 13, color: "var(--text-primary)", wordBreak: "break-all" }}>{createdCredentials?.apiKey}</span>
              <CredentialCopyButton value={createdCredentials?.apiKey ?? ""} />
            </div>
          </div>

          <div className="form-message error field full" style={{ borderLeft: "3px solid #eab308", background: "rgba(234,179,8,0.05)", color: "#eab308" }}>
            This is the only time your API key will be shown. Copy it now — it cannot be recovered.
          </div>

          <div className="field full">
            <button
              type="button"
              className={`button ${hasDownloaded ? "secondary" : "primary"}`}
              style={{ width: "100%", justifyContent: "center" }}
              onClick={triggerFirmwareDownload}
            >
              {hasDownloaded ? <Check size={14} /> : <Download size={14} />}
              {hasDownloaded ? "Firmware Template Downloaded" : "Download Firmware Template"}
            </button>
            {!hasDownloaded && (
              <p style={{ fontSize: 12, color: "var(--text-muted)", marginTop: 6, textAlign: "center" }}>
                Required — download the pre-filled sketch before continuing.
              </p>
            )}
          </div>

          <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 8, gridColumn: "1 / -1" }}>
            <button
              type="button"
              className="button primary"
              disabled={!hasDownloaded}
              onClick={() => {
                setCreatedCredentials(null);
                setHasDownloaded(false);
                setFirmwareFileContent(null);
                void devices.refetch();
              }}
            >
              Done
            </button>
          </div>
        </div>
      </Modal>
    </>
  );
}
