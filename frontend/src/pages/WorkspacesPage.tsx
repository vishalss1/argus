import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { Plus, Briefcase, Play, Search, FolderOpen, Cpu, Trash2 } from "lucide-react";
import { 
  useWorkspaces, 
  useCreateWorkspace, 
  useSessions, 
  useCreateSession, 
  useStartSession,
  useWorkspaceDevices,
  useDevices,
  useAssignDevice,
  useUnassignDevice
} from "../hooks/useArgusData";
import { PageHeader, Panel, EmptyState, Modal } from "../components/ui";

export function WorkspacesPage() {
  const { data: workspaces, isLoading } = useWorkspaces();
  const [selectedWorkspace, setSelectedWorkspace] = useState<string | null>(null);
  
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const createWorkspace = useCreateWorkspace();

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName) return;
    await createWorkspace.mutateAsync({ name: newName, description: newDesc });
    setShowCreateModal(false);
    setNewName("");
    setNewDesc("");
  };

  return (
    <>
      <PageHeader
        title="Workspaces"
        description="Manage logical fleets and active operational sessions."
        actions={
          <button className="button primary" onClick={() => setShowCreateModal(true)}>
            <Plus size={16} /> New Workspace
          </button>
        }
      />

      <div className="grid two">
        <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
          {isLoading && <p>Loading workspaces...</p>}
          {!isLoading && workspaces?.length === 0 && (
            <EmptyState title="No Workspaces" description="Create a workspace to organize your devices and run sessions." />
          )}
          {workspaces?.map((ws) => (
            <div 
              key={ws.id} 
              className={`panel interactive ${selectedWorkspace === ws.id ? "selected" : ""}`}
              onClick={() => setSelectedWorkspace(ws.id)}
              style={{ padding: "16px", cursor: "pointer", border: selectedWorkspace === ws.id ? "1px solid var(--accent)" : undefined }}
            >
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: "12px", marginBottom: "8px" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
                  <Briefcase size={20} style={{ color: "var(--accent)" }} />
                  <h3 style={{ margin: 0 }}>{ws.name}</h3>
                </div>
                <span className="mono" style={{ 
                  fontSize: "11px", 
                  padding: "2px 8px", 
                  background: "rgba(255,255,255,0.05)", 
                  border: "1px solid var(--line)", 
                  borderRadius: "12px", 
                  color: "var(--faint)" 
                }}>
                  {ws.device_count ?? 0} {ws.device_count === 1 ? "device" : "devices"}
                </span>
              </div>
              <p className="muted" style={{ margin: 0, fontSize: "14px" }}>{ws.description || "No description provided."}</p>
              <div className="mono muted" style={{ fontSize: "11px", marginTop: "12px" }}>ID: {ws.id}</div>
            </div>
          ))}
        </div>

        <div>
          {selectedWorkspace ? (
            <WorkspaceDetail workspaceID={selectedWorkspace} />
          ) : (
            <Panel>
              <EmptyState title="Select a Workspace" description="Choose a workspace from the list to view its sessions and details." />
            </Panel>
          )}
        </div>
      </div>

      <Modal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} title="Create Workspace">
        <form onSubmit={handleCreate} style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
          <div className="form-group">
            <label>Name</label>
            <input type="text" value={newName} onChange={e => setNewName(e.target.value)} placeholder="e.g. Factory Floor A" required autoFocus />
          </div>
          <div className="form-group">
            <label>Description</label>
            <textarea value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder="Optional description..." rows={3} />
          </div>
          <div style={{ display: "flex", justifyContent: "flex-end", gap: "12px", marginTop: "8px" }}>
            <button type="button" className="button secondary" onClick={() => setShowCreateModal(false)}>Cancel</button>
            <button type="submit" className="button primary" disabled={createWorkspace.isPending}>
              {createWorkspace.isPending ? "Creating..." : "Create"}
            </button>
          </div>
        </form>
      </Modal>
    </>
  );
}

function WorkspaceDetail({ workspaceID }: { workspaceID: string }) {
  const [activeTab, setActiveTab] = useState<"sessions" | "devices">("sessions");
  
  // Sessions data
  const { data: sessions, isLoading: sessionsLoading } = useSessions(workspaceID);
  const createSession = useCreateSession();
  const startSession = useStartSession();

  // Devices data
  const { data: workspaceDevices, isLoading: devicesLoading } = useWorkspaceDevices(workspaceID);
  const { data: allDevices } = useDevices();
  const assignDevice = useAssignDevice();
  const unassignDevice = useUnassignDevice();

  const [deviceToAssign, setDeviceToAssign] = useState("");

  const handleNewSession = async () => {
    const s = await createSession.mutateAsync(workspaceID);
    await startSession.mutateAsync(s.id);
  };

  const handleAssignDevice = async () => {
    if (!deviceToAssign) return;
    await assignDevice.mutateAsync({ workspaceID, deviceID: deviceToAssign });
    setDeviceToAssign("");
  };

  const handleUnassignDevice = async (deviceID: string) => {
    await unassignDevice.mutateAsync({ workspaceID, deviceID });
  };

  const unassignedDevices = useMemo(() => {
    if (!allDevices) return [];
    return allDevices.filter(d => !d.workspace_id);
  }, [allDevices]);

  if (sessionsLoading || devicesLoading) return <Panel>Loading detail view...</Panel>;

  const activeSessions = sessions?.filter(s => s.status === "CREATED" || s.status === "RUNNING") || [];
  const history = sessions?.filter(s => s.status === "COMPLETED" || s.status === "FAILED" || s.status === "CANCELLED") || [];

  return (
    <Panel 
      title={
        <div style={{ display: "flex", alignItems: "center", gap: "20px" }}>
          <span 
            onClick={() => setActiveTab("sessions")} 
            style={{ 
              cursor: "pointer", 
              color: activeTab === "sessions" ? "var(--text)" : "var(--faint)",
              borderBottom: activeTab === "sessions" ? "2px solid var(--accent)" : "2px solid transparent",
              paddingBottom: "4px",
              fontWeight: activeTab === "sessions" ? 600 : 400,
              fontSize: "16px",
              transition: "all 0.2s"
            }}
          >
            Sessions
          </span>
          <span 
            onClick={() => setActiveTab("devices")} 
            style={{ 
              cursor: "pointer", 
              color: activeTab === "devices" ? "var(--text)" : "var(--faint)",
              borderBottom: activeTab === "devices" ? "2px solid var(--accent)" : "2px solid transparent",
              paddingBottom: "4px",
              fontWeight: activeTab === "devices" ? 600 : 400,
              fontSize: "16px",
              transition: "all 0.2s"
            }}
          >
            Devices
          </span>
        </div>
      } 
      actions={
        activeTab === "sessions" ? (
          <button className="button primary compact" onClick={handleNewSession} disabled={createSession.isPending || startSession.isPending}>
            <Play size={14} /> Start New Session
          </button>
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
            <select
              value={deviceToAssign}
              onChange={e => setDeviceToAssign(e.target.value)}
              style={{
                padding: "6px 12px",
                background: "var(--surface-2)",
                border: "1px solid var(--line)",
                borderRadius: "4px",
                color: "inherit",
                fontSize: "13px",
                outline: "none"
              }}
            >
              <option value="">Select Device...</option>
              {unassignedDevices.map(d => (
                <option key={d.id} value={d.id}>{d.name} ({d.type})</option>
              ))}
            </select>
            <button 
              className="button primary compact" 
              onClick={handleAssignDevice} 
              disabled={!deviceToAssign || assignDevice.isPending}
            >
              Add Device
            </button>
          </div>
        )
      }
    >
      {activeTab === "sessions" ? (
        <div style={{ display: "flex", flexDirection: "column", gap: "24px" }}>
          <div>
            <h4 style={{ margin: "0 0 12px 0", fontSize: "14px", color: "var(--faint)", textTransform: "uppercase", letterSpacing: "0.5px" }}>Active</h4>
            {activeSessions.length === 0 ? (
              <p className="muted" style={{ fontSize: "14px" }}>No active sessions.</p>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                {activeSessions.map(s => (
                  <div key={s.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px", background: "var(--surface-2)", borderRadius: "6px" }}>
                    <div>
                      <div style={{ fontWeight: 500 }}>Session <span className="mono" style={{ fontSize: "12px" }}>{s.id.split("-")[0]}</span></div>
                      <div className="muted" style={{ fontSize: "12px", marginTop: "4px" }}>Status: <span style={{ color: s.status === "RUNNING" ? "var(--success)" : "var(--accent)" }}>{s.status}</span></div>
                    </div>
                    <Link to={`/sessions/${s.id}`} className="button secondary compact">
                      Open Dashboard
                    </Link>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div>
            <h4 style={{ margin: "0 0 12px 0", fontSize: "14px", color: "var(--faint)", textTransform: "uppercase", letterSpacing: "0.5px" }}>History</h4>
            {history.length === 0 ? (
              <p className="muted" style={{ fontSize: "14px" }}>No historical sessions.</p>
            ) : (
              <div className="table-wrap">
                <table className="ai-table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Status</th>
                      <th>Started</th>
                      <th>Ended</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {history.map(s => (
                      <tr key={s.id}>
                        <td className="mono" style={{ fontSize: "12px" }}>{s.id.split("-")[0]}</td>
                        <td>
                          <span style={{ color: s.status === "FAILED" ? "var(--danger)" : "var(--faint)", fontSize: "12px" }}>
                            {s.status}
                          </span>
                        </td>
                        <td className="muted" style={{ fontSize: "12px" }}>{s.started_at ? new Date(s.started_at).toLocaleDateString() : "-"}</td>
                        <td className="muted" style={{ fontSize: "12px" }}>{s.ended_at ? new Date(s.ended_at).toLocaleDateString() : "-"}</td>
                        <td style={{ textAlign: "right" }}>
                          <Link to={`/sessions/${s.id}/report`} style={{ color: "var(--accent)", fontSize: "12px", textDecoration: "none" }}>View Report</Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>
          <h4 style={{ margin: "0", fontSize: "14px", color: "var(--faint)", textTransform: "uppercase", letterSpacing: "0.5px" }}>
            Assigned Devices ({workspaceDevices?.length ?? 0})
          </h4>
          
          {(!workspaceDevices || workspaceDevices.length === 0) ? (
            <p className="muted" style={{ fontSize: "14px", margin: "10px 0" }}>No devices assigned to this workspace yet.</p>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
              {workspaceDevices.map(d => (
                <div 
                  key={d.id} 
                  style={{ 
                    display: "flex", 
                    alignItems: "center", 
                    justifyContent: "space-between", 
                    padding: "12px 16px", 
                    background: "var(--surface-2)", 
                    borderRadius: "6px",
                    border: "1px solid var(--line)" 
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: "12px" }}>
                    <Cpu size={18} style={{ color: "var(--accent)" }} />
                    <div>
                      <div style={{ fontWeight: 500, fontSize: "14px" }}>{d.name}</div>
                      <div className="muted" style={{ fontSize: "11px", marginTop: "2px" }}>
                        Type: {d.type} | Version: {d.firmware_version}
                      </div>
                    </div>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: "16px" }}>
                    <span style={{ 
                      fontSize: "12px", 
                      color: d.status === "online" ? "var(--success)" : "var(--faint)",
                      display: "flex",
                      alignItems: "center",
                      gap: "4px"
                    }}>
                      <span style={{ 
                        width: "6px", 
                        height: "6px", 
                        borderRadius: "50%", 
                        background: d.status === "online" ? "var(--success)" : "var(--faint)" 
                      }} />
                      {d.status}
                    </span>
                    <button
                      className="button danger compact"
                      onClick={() => handleUnassignDevice(d.id)}
                      disabled={unassignDevice.isPending}
                      style={{ 
                        padding: "6px", 
                        background: "rgba(239, 68, 68, 0.1)", 
                        border: "1px solid rgba(239, 68, 68, 0.2)",
                        color: "var(--danger)",
                        borderRadius: "4px",
                        cursor: "pointer",
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center"
                      }}
                      title="Unassign Device"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </Panel>
  );
}
