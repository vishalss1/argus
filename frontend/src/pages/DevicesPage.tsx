import { FormEvent, useMemo, useState, Fragment } from "react";
import { Plus, RefreshCw, Trash2, ChevronDown, ChevronRight, Server, Download } from "lucide-react";
import { api } from "../services/api";
import { EmptyState, ErrorState, LoadingRows, PageHeader, Panel, StatusChip, Modal } from "../components/ui";
import { useFleets, useCreateFleet } from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { compactID, formatDate, safeJsonParse } from "../lib/format";

export function DevicesPage() {
  const { workspaceDevices } = useWorkspaceContext();
  const fleets = useFleets();
  const create = useCreateFleet();
  const [query, setQuery] = useState("");
  const [formError, setFormError] = useState("");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [expandedFleets, setExpandedFleets] = useState<Set<string>>(new Set());
  const [createdFleetData, setCreatedFleetData] = useState<{
    blobUrl: string;
    filename: string;
    name: string;
    node_count: number;
    hardware_type: string;
    node_prefix: string;
  } | null>(null);
  const [hasDownloaded, setHasDownloaded] = useState(false);

  const [isAddDevicesModalOpen, setIsAddDevicesModalOpen] = useState(false);
  const [fleetToAddDevices, setFleetToAddDevices] = useState<any | null>(null);
  const [addDevicesFormError, setAddDevicesFormError] = useState("");
  const [isAddingDevices, setIsAddingDevices] = useState(false);
  const [addedDevicesData, setAddedDevicesData] = useState<{
    blobUrl: string;
    filename: string;
    fleet_name: string;
    node_count: number;
    node_prefix: string;
  } | null>(null);

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return fleets.data ?? [];
    return (fleets.data ?? []).filter((fleet) =>
      [fleet.name, fleet.id, fleet.node_role, fleet.hardware_type, fleet.firmware_version].join(" ").toLowerCase().includes(needle)
    );
  }, [fleets.data, query]);

  async function onCreate(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFormError("");
    const form = new FormData(event.currentTarget);
    const name = String(form.get("name") || "");
    const node_count = Number(form.get("node_count") || 1);
    const hardware_type = String(form.get("hardware_type") || "");
    const node_prefix = String(form.get("node_prefix") || "");

    try {
      const blob = await create.mutateAsync({
        name,
        node_role: String(form.get("node_role") || ""),
        hardware_type,
        node_prefix,
        node_count,
        firmware_version: String(form.get("firmware_version") || ""),
        firmware_template: "",
        wifi_ssid: String(form.get("wifi_ssid") || ""),
        wifi_password: String(form.get("wifi_password") || "")
      });
      
      const url = window.URL.createObjectURL(blob);
      setCreatedFleetData({
        blobUrl: url,
        filename: `fleet_${name.replace(/\s+/g, "_")}.zip`,
        name,
        node_count,
        hardware_type,
        node_prefix
      });
      setHasDownloaded(false);
    } catch (error) {
      setFormError((error as Error).message);
    }
  }

  async function onAddDevices(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAddDevicesFormError("");
    if (!fleetToAddDevices) return;
    
    setIsAddingDevices(true);
    const form = new FormData(event.currentTarget);
    const node_count = Number(form.get("node_count") || 1);
    const node_prefix = String(form.get("node_prefix") || fleetToAddDevices.node_prefix || "Node");
    
    try {
      const blob = await api.fleets.addDevices(fleetToAddDevices.id, {
        node_count,
        node_prefix,
        wifi_ssid: String(form.get("wifi_ssid") || ""),
        wifi_password: String(form.get("wifi_password") || "")
      });
      
      const url = window.URL.createObjectURL(blob);
      setAddedDevicesData({
        blobUrl: url,
        filename: `fleet_${fleetToAddDevices.name.replace(/\s+/g, "_")}_new_devices.zip`,
        fleet_name: fleetToAddDevices.name,
        node_count,
        node_prefix
      });
      setHasDownloaded(false);
    } catch (error) {
      setAddDevicesFormError((error as Error).message);
    } finally {
      setIsAddingDevices(false);
    }
  }

  async function removeDevice(id: string) {
    if (window.confirm("Are you sure you want to delete this device?")) {
      try {
        await api.devices.remove(id);
        // Force reload to update context
        window.location.reload();
      } catch (err) {
        console.error("Failed to delete device", err);
        alert("Failed to delete device: " + (err as Error).message);
      }
    }
  }

  async function removeFleet(id: string) {
    await api.fleets.remove(id);
    await fleets.refetch();
  }

  const toggleExpand = (id: string) => {
    setExpandedFleets(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  return (
    <>
      <PageHeader
        eyebrow="Fleet Registry"
        title="Fleets"
        description="Create, inspect, update, and delete fleet records from the ARGUS backend."
        actions={
          <>
            <button className="button secondary" onClick={() => void fleets.refetch()}><RefreshCw size={15} />Refresh</button>
            <button className="button primary" onClick={() => setIsModalOpen(true)}><Plus size={15} />Create Fleet</button>
          </>
        }
      />
      <div style={{ padding: "0 20px" }}>
        <Panel title="Registered Fleets" subtitle={`${filtered.length} visible`}>
          <label className="field" style={{ marginBottom: 14 }}>
            <span>Search</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, role, hardware, firmware" />
          </label>
          {fleets.isError ? (
            <ErrorState message={(fleets.error as Error).message} onRetry={() => void fleets.refetch()} />
          ) : (
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th style={{ width: 30 }}></th>
                    <th>Name</th>
                    <th>Role</th>
                    <th>Nodes</th>
                    <th>Firmware</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {fleets.isLoading && <LoadingRows rows={6} />}
                  {!fleets.isLoading && filtered.length === 0 && (
                    <tr><td colSpan={6}><EmptyState title="No fleets registered" description="Create the first fleet using the form on this page." /></td></tr>
                  )}
                  {filtered.map((fleet) => (
                    <Fragment key={fleet.id}>
                      <tr onClick={() => toggleExpand(fleet.id)} style={{ cursor: "pointer" }}>
                        <td style={{ color: "var(--text-muted)" }}>
                          {expandedFleets.has(fleet.id) ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                        </td>
                        <td>
                          <strong>{fleet.name}</strong>
                          <div className="muted mono">{compactID(fleet.id)}</div>
                        </td>
                        <td>{fleet.node_role}<br/><span className="muted mono" style={{fontSize: "0.85em"}}>{fleet.hardware_type}</span></td>
                        <td>{fleet.online_nodes} / {fleet.total_nodes} online</td>
                        <td>{fleet.firmware_version || "Unset"}</td>
                        <td onClick={e => e.stopPropagation()}>
                          <div className="page-actions">
                            <button className="button compact secondary" onClick={(e) => { e.stopPropagation(); setFleetToAddDevices(fleet); setIsAddDevicesModalOpen(true); }} aria-label={`Add Devices to ${fleet.name}`}><Plus size={14} /></button>
                            <button className="button compact secondary" onClick={() => void api.fleets.firmware(fleet.id)} aria-label={`Download Firmware for ${fleet.name}`}><Download size={14} /></button>
                            <button className="button compact danger" onClick={() => void removeFleet(fleet.id)} aria-label={`Delete ${fleet.name}`}><Trash2 size={14} /></button>
                          </div>
                        </td>
                      </tr>
                      {expandedFleets.has(fleet.id) && (
                        <tr style={{ backgroundColor: "rgba(255,255,255,0.02)" }}>
                          <td></td>
                          <td colSpan={5} style={{ padding: "16px 16px 16px 0" }}>
                            <div className="table-wrap" style={{ margin: 0, border: "1px solid var(--border-color)", backgroundColor: "var(--bg-panel)" }}>
                              <table>
                                <thead>
                                  <tr>
                                    <th>Status</th>
                                    <th>Device</th>
                                    <th>Firmware</th>
                                    <th>Last Seen</th>
                                    <th>RSSI</th>
                                    <th style={{ textAlign: "right" }}>Actions</th>
                                  </tr>
                                </thead>
                                <tbody>
                                  {(() => {
                                    const fleetDevices = workspaceDevices.filter(d => d.fleet_id === fleet.id);
                                    if (!fleetDevices || fleetDevices.length === 0) {
                                      return <tr><td colSpan={5}><EmptyState title="No devices" description="This fleet currently has no devices." /></td></tr>;
                                    }
                                    return fleetDevices.map(device => (
                                      <tr key={device.id}>
                                        <td><StatusChip value={device.status} /></td>
                                        <td><strong>{device.name}</strong><div className="muted mono">{compactID(device.id)}</div></td>
                                        <td>{fleet.firmware_version || "Unset"}</td>
                                        <td>{formatDate(device.last_seen)}</td>
                                        <td>{device.status === 'online' ? "-65 dBm" : "--"}</td>
                                        <td style={{ textAlign: "right" }}>
                                          <button className="button compact danger icon-only" onClick={(e) => { e.stopPropagation(); removeDevice(device.id); }} aria-label={`Delete ${device.name}`}>
                                            <Trash2 size={14} />
                                          </button>
                                        </td>
                                      </tr>
                                    ));
                                  })()}
                                </tbody>
                              </table>
                            </div>
                          </td>
                        </tr>
                      )}
                    </Fragment>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>

      <Modal 
        isOpen={isModalOpen} 
        onClose={() => {
          if (createdFleetData && !hasDownloaded) return;
          setIsModalOpen(false);
          if (createdFleetData) {
            window.URL.revokeObjectURL(createdFleetData.blobUrl);
            setCreatedFleetData(null);
            fleets.refetch();
          }
        }} 
        title={createdFleetData ? "Fleet Provisioned" : "Create Fleet"} 
        style={{ maxWidth: 580, width: "90vw" }}
      >
        {!createdFleetData ? (
          <form className="form-grid" onSubmit={onCreate} style={{ padding: "16px 0 0 0", display: "flex", flexDirection: "column", gap: "16px" }}>
            
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
              <label className="field full" style={{ margin: 0 }}><span>Fleet Name</span><input name="name" required placeholder="Factory Sensors" /></label>
              <div className="field full" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", margin: 0 }}>
                <label className="field" style={{ margin: 0 }}><span>Node Role</span><input name="node_role" required placeholder="Sensor Node" /></label>
                <label className="field" style={{ margin: 0 }}><span>Hardware</span>
                  <select name="hardware_type" defaultValue="ESP32">
                    <option>ESP32</option>
                    <option>ESP32-S3</option>
                    <option>ESP8266</option>
                  </select>
                </label>
              </div>
              <div className="field full" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", margin: 0 }}>
                <label className="field" style={{ margin: 0 }}><span>Node Prefix</span><input name="node_prefix" required defaultValue="Node" /></label>
                <label className="field" style={{ margin: 0 }}><span>Node Count</span><input name="node_count" type="number" min="1" required defaultValue="1" /></label>
              </div>
              <label className="field full" style={{ margin: 0 }}><span>Firmware Version</span><input name="firmware_version" required defaultValue="1.0.0" /></label>
              <div className="field full" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", margin: 0, paddingTop: "12px", borderTop: "1px solid var(--border-color)" }}>
                <label className="field" style={{ margin: 0 }}><span>Target WiFi SSID</span><input name="wifi_ssid" placeholder="IoT_Network" /></label>
                <label className="field" style={{ margin: 0 }}><span>WiFi Password</span><input name="wifi_password" type="password" placeholder="••••••••" /></label>
              </div>
            </div>

            {formError && <p className="muted field full" style={{ margin: 0 }}>{formError}</p>}
            <div className="page-actions field full" style={{ gridColumn: "1 / -1", marginTop: "16px", justifyContent: "flex-end", display: "flex", gap: "12px", paddingTop: "20px", borderTop: "1px solid var(--border-color)" }}>
              <button className="button secondary" type="button" onClick={() => setIsModalOpen(false)}>Cancel</button>
              <button className="button primary" type="submit" disabled={create.isPending}><Plus size={15} />Create Fleet</button>
            </div>
          </form>
        ) : (
          <div style={{ padding: "24px 0 0 0", display: "flex", flexDirection: "column", gap: "24px" }}>
            <div style={{ padding: "16px", backgroundColor: "rgba(0, 255, 128, 0.05)", border: "1px solid rgba(0, 255, 128, 0.2)", borderRadius: "8px" }}>
              <h3 style={{ color: "var(--accent-green)", margin: "0 0 8px 0" }}>Successfully Provisioned {createdFleetData.name}</h3>
              <p style={{ margin: 0, color: "var(--text-secondary)" }}>
                Generated {createdFleetData.node_count} node(s) with prefix "{createdFleetData.node_prefix}" for {createdFleetData.hardware_type}.
              </p>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))", gap: "12px", maxHeight: "240px", overflowY: "auto", paddingRight: "8px" }}>
              {Array.from({ length: createdFleetData.node_count }).map((_, i) => (
                <div key={i} style={{ padding: "8px 12px", background: "var(--surface-2)", borderRadius: "4px", border: "1px solid var(--border)", fontSize: "13px", fontFamily: "monospace", color: "var(--text-primary)", display: "flex", alignItems: "center", gap: "8px" }}>
                  <div style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--text-secondary)" }}></div>
                  {createdFleetData.node_prefix}-{String(i+1).padStart(3, '0')}
                </div>
              ))}
            </div>
            
            <div className="page-actions" style={{ marginTop: "8px", justifyContent: "flex-end", display: "flex", gap: "12px", paddingTop: "20px", borderTop: "1px solid var(--border-color)" }}>
              <button className="button secondary" onClick={() => {
                  const a = document.createElement("a");
                  a.href = createdFleetData.blobUrl;
                  a.download = createdFleetData.filename;
                  document.body.appendChild(a);
                  a.click();
                  document.body.removeChild(a);
                  setHasDownloaded(true);
              }}>
                Download ZIP Bundle
              </button>
              <button className="button primary" disabled={!hasDownloaded} onClick={() => {
                  setIsModalOpen(false);
                  window.URL.revokeObjectURL(createdFleetData.blobUrl);
                  setCreatedFleetData(null);
                  fleets.refetch();
              }}>
                Done
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal 
        isOpen={isAddDevicesModalOpen} 
        onClose={() => {
          if (addedDevicesData && !hasDownloaded) return;
          setIsAddDevicesModalOpen(false);
          if (addedDevicesData) {
            window.URL.revokeObjectURL(addedDevicesData.blobUrl);
            setAddedDevicesData(null);
            fleets.refetch();
          }
        }} 
        title={addedDevicesData ? "Devices Provisioned" : `Add Devices to ${fleetToAddDevices?.name}`} 
        style={{ maxWidth: 500, width: "90vw" }}
      >
        {!addedDevicesData ? (
          <form className="form-grid" onSubmit={onAddDevices} style={{ padding: "16px 0 0 0", display: "flex", flexDirection: "column", gap: "16px" }}>
            
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
              <div className="field full" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", margin: 0 }}>
                <label className="field" style={{ margin: 0 }}><span>Node Prefix</span><input name="node_prefix" required defaultValue={fleetToAddDevices?.node_prefix || "Node"} /></label>
                <label className="field" style={{ margin: 0 }}><span>New Node Count</span><input name="node_count" type="number" min="1" required defaultValue="1" /></label>
              </div>
              <div className="field full" style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "12px", margin: 0, paddingTop: "12px", borderTop: "1px solid var(--border-color)" }}>
                <label className="field" style={{ margin: 0 }}><span>Target WiFi SSID</span><input name="wifi_ssid" placeholder="IoT_Network" /></label>
                <label className="field" style={{ margin: 0 }}><span>WiFi Password</span><input name="wifi_password" type="password" placeholder="••••••••" /></label>
              </div>
            </div>

            {addDevicesFormError && <p className="muted field full" style={{ margin: 0 }}>{addDevicesFormError}</p>}
            <div className="page-actions field full" style={{ gridColumn: "1 / -1", marginTop: "16px", justifyContent: "flex-end", display: "flex", gap: "12px", paddingTop: "20px", borderTop: "1px solid var(--border-color)" }}>
              <button className="button secondary" type="button" onClick={() => setIsAddDevicesModalOpen(false)}>Cancel</button>
              <button className="button primary" type="submit" disabled={isAddingDevices}><Plus size={15} />Add Devices</button>
            </div>
          </form>
        ) : (
          <div style={{ padding: "24px 0 0 0", display: "flex", flexDirection: "column", gap: "24px" }}>
            <div style={{ padding: "16px", backgroundColor: "rgba(0, 255, 128, 0.05)", border: "1px solid rgba(0, 255, 128, 0.2)", borderRadius: "8px" }}>
              <h3 style={{ color: "var(--accent-green)", margin: "0 0 8px 0" }}>Successfully Added to {addedDevicesData.fleet_name}</h3>
              <p style={{ margin: 0, color: "var(--text-secondary)" }}>
                Generated {addedDevicesData.node_count} node(s) with prefix "{addedDevicesData.node_prefix}".
              </p>
            </div>
            
            <div style={{ display: "flex", gap: "12px", justifyContent: "flex-end", paddingTop: "20px", borderTop: "1px solid var(--border-color)" }}>
              <button className="button primary" onClick={() => {
                const a = document.createElement("a");
                a.href = addedDevicesData.blobUrl;
                a.download = addedDevicesData.filename;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                setHasDownloaded(true);
              }}>
                <Download size={15} /> Download ZIP
              </button>
              {hasDownloaded && <button className="button secondary" onClick={() => setIsAddDevicesModalOpen(false)}>Close</button>}
            </div>
          </div>
        )}
      </Modal>
    </>
  );
}
