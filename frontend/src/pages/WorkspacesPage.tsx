import { useState, useMemo } from "react";
import { Link } from "react-router-dom";
import { Plus, Briefcase, Play, Cpu, Trash2, Edit2 } from "lucide-react";
import {
  useWorkspaces,
  useCreateWorkspace,
  useDeleteWorkspace,
  useUpdateWorkspace,
  useSessions,
  useCreateSession,
  useStartSession,
  useWorkspaceDevices,
  useDevices,
  useAssignDevice,
  useUnassignDevice
} from "../hooks/useArgusData";
import { useAuth } from "../context/AuthContext";
import { PageHeader, Panel, EmptyState, Modal } from "../components/ui";

export function WorkspacesPage() {
  const { data: workspaces, isLoading } = useWorkspaces();
  const [selectedWorkspace, setSelectedWorkspace] = useState<string | null>(null);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const createWorkspace = useCreateWorkspace();

  const deleteWorkspace = useDeleteWorkspace();
  const { refreshMe } = useAuth();
  const [workspaceToDelete, setWorkspaceToDelete] = useState<{ id: string; name: string } | null>(null);

  const [showEditModal, setShowEditModal] = useState(false);
  const [editId, setEditId] = useState("");
  const [editName, setEditName] = useState("");
  const [editDesc, setEditDesc] = useState("");
  const updateWorkspace = useUpdateWorkspace();

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editId || !editName) return;
    try {
      await updateWorkspace.mutateAsync({ id: editId, name: editName, description: editDesc });
      await refreshMe();
      setShowEditModal(false);
      setEditId("");
      setEditName("");
      setEditDesc("");
    } catch (err: any) {
      alert(err.message || "Failed to update workspace");
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName) return;
    await createWorkspace.mutateAsync({ name: newName, description: newDesc });
    await refreshMe();
    setShowCreateModal(false);
    setNewName("");
    setNewDesc("");
  };

  const handleDeleteWorkspace = async () => {
    if (!workspaceToDelete) return;
    try {
      await deleteWorkspace.mutateAsync(workspaceToDelete.id);
      await refreshMe();
      if (selectedWorkspace === workspaceToDelete.id) {
        setSelectedWorkspace(null);
      }
      setWorkspaceToDelete(null);
    } catch (err: any) {
      alert(err.message || "Failed to delete workspace");
    }
  };

  return (
    <>
      <PageHeader
        eyebrow="Operational Grouping"
        title="Workspaces"
        description="Manage logical fleets and active operational sessions."
        actions={
          <button className="btn-inverse" onClick={() => setShowCreateModal(true)}>
            <Plus size={15} strokeWidth={1.5} /> New Workspace
          </button>
        }
      />

      <div className="grid two">
        <div className="workspace-list">
          {isLoading && <p className="muted">Loading workspaces...</p>}
          {!isLoading && workspaces?.length === 0 && (
            <EmptyState title="No Workspaces" description="Create a workspace to organize your devices and run sessions." />
          )}
          {workspaces?.map((ws) => (
            <div
              key={ws.id}
              className={`panel interactive ${selectedWorkspace === ws.id ? "selected" : ""}`}
              onClick={() => setSelectedWorkspace(ws.id)}
            >
              <div className="workspace-card-header">
                <div className="workspace-card-title">
                  <Briefcase size={18} strokeWidth={1.5} />
                  <h3>{ws.name}</h3>
                </div>
                <div className="workspace-card-actions" onClick={(e) => e.stopPropagation()}>
                  <span className="inline-chip">
                    {ws.device_count ?? 0} {ws.device_count === 1 ? "device" : "devices"}
                  </span>
                  <button
                    className="icon-button"
                    onClick={() => {
                      setEditId(ws.id);
                      setEditName(ws.name);
                      setEditDesc(ws.description || "");
                      setShowEditModal(true);
                    }}
                    title="Edit Workspace"
                    aria-label={`Edit ${ws.name}`}
                  >
                    <Edit2 size={13} strokeWidth={1.5} />
                  </button>
                  <button
                    className="icon-button danger"
                    onClick={() => setWorkspaceToDelete({ id: ws.id, name: ws.name })}
                    title="Delete Workspace"
                    aria-label={`Delete ${ws.name}`}
                  >
                    <Trash2 size={13} strokeWidth={1.5} />
                  </button>
                </div>
              </div>
              <p className="muted workspace-card-desc">{ws.description || "No description provided."}</p>
              <div className="mono muted workspace-card-id">ID: {ws.id}</div>
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
        <form onSubmit={handleCreate} className="modal-form">
          <div className="form-group">
            <label>Name</label>
            <input type="text" value={newName} onChange={e => setNewName(e.target.value)} placeholder="e.g. Factory Floor A" required autoFocus />
          </div>
          <div className="form-group">
            <label>Description</label>
            <textarea value={newDesc} onChange={e => setNewDesc(e.target.value)} placeholder="Optional description..." rows={3} />
          </div>
          <div className="modal-actions">
            <button type="button" className="button secondary" onClick={() => setShowCreateModal(false)}>Cancel</button>
            <button type="submit" className="btn-inverse" disabled={createWorkspace.isPending}>
              {createWorkspace.isPending ? "Creating..." : "Create"}
            </button>
          </div>
        </form>
      </Modal>

      <Modal isOpen={Boolean(workspaceToDelete)} onClose={() => setWorkspaceToDelete(null)} title="Delete Workspace">
        <div className="modal-form">
          <p className="modal-text">
            Are you sure you want to delete workspace <strong>{workspaceToDelete?.name}</strong>?
          </p>
          <p className="muted modal-text-sub">
            This action is irreversible. All associated sessions and historical telemetry logs will be permanently deleted. Assigned devices will be unassigned.
          </p>
          <div className="modal-actions">
            <button type="button" className="button secondary" onClick={() => setWorkspaceToDelete(null)}>Cancel</button>
            <button type="button" className="btn-inverse" onClick={handleDeleteWorkspace} disabled={deleteWorkspace.isPending}>
              {deleteWorkspace.isPending ? "Deleting..." : "Delete Workspace"}
            </button>
          </div>
        </div>
      </Modal>

      <Modal isOpen={showEditModal} onClose={() => setShowEditModal(false)} title="Update Workspace">
        <form onSubmit={handleUpdate} className="modal-form">
          <div className="form-group">
            <label>Name</label>
            <input type="text" value={editName} onChange={e => setEditName(e.target.value)} required autoFocus />
          </div>
          <div className="form-group">
            <label>Description</label>
            <textarea value={editDesc} onChange={e => setEditDesc(e.target.value)} placeholder="Optional description..." rows={3} />
          </div>
          <div className="modal-actions">
            <button type="button" className="button secondary" onClick={() => setShowEditModal(false)}>Cancel</button>
            <button type="submit" className="btn-inverse" disabled={updateWorkspace.isPending}>
              {updateWorkspace.isPending ? "Saving..." : "Save Changes"}
            </button>
          </div>
        </form>
      </Modal>
    </>
  );
}


function WorkspaceDetail({ workspaceID }: { workspaceID: string }) {
  const [activeTab, setActiveTab] = useState<"sessions" | "devices">("sessions");

  const { data: sessions, isLoading: sessionsLoading } = useSessions(workspaceID);
  const createSession = useCreateSession();
  const startSession = useStartSession();

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
        <div className="workspace-tabs">
          <button
            type="button"
            className={`workspace-tab ${activeTab === "sessions" ? "active" : ""}`}
            onClick={() => setActiveTab("sessions")}
          >
            Sessions
          </button>
          <button
            type="button"
            className={`workspace-tab ${activeTab === "devices" ? "active" : ""}`}
            onClick={() => setActiveTab("devices")}
          >
            Devices
          </button>
        </div>
      }
      actions={
        activeTab === "sessions" ? (
          <button className="btn-inverse compact" onClick={handleNewSession} disabled={createSession.isPending || startSession.isPending}>
            <Play size={13} strokeWidth={1.5} /> Start New Session
          </button>
        ) : (
          <div className="workspace-assign">
            <select
              value={deviceToAssign}
              onChange={e => setDeviceToAssign(e.target.value)}
            >
              <option value="">Select device...</option>
              {unassignedDevices.map(d => (
                <option key={d.id} value={d.id}>{d.name} ({d.type})</option>
              ))}
            </select>
            <button
              className="btn-inverse compact"
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
        <div className="detail-stack">
          <div>
            <h4 className="section-heading">Active</h4>
            {activeSessions.length === 0 ? (
              <p className="muted">No active sessions.</p>
            ) : (
              <div className="detail-stack">
                {activeSessions.map(s => (
                  <div key={s.id} className="row-card">
                    <div className="row-card-text">
                      <strong>Session <span className="mono">{s.id.split("-")[0]}</span></strong>
                      <small>Status: {s.status}</small>
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
            <h4 className="section-heading">History</h4>
            {history.length === 0 ? (
              <p className="muted">No historical sessions.</p>
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
                        <td className="mono">{s.id.split("-")[0]}</td>
                        <td>{s.status}</td>
                        <td className="muted">{s.started_at ? new Date(s.started_at).toLocaleDateString() : "-"}</td>
                        <td className="muted">{s.ended_at ? new Date(s.ended_at).toLocaleDateString() : "-"}</td>
                        <td style={{ textAlign: "right" }}>
                          <Link to={`/sessions/${s.id}/report`} className="t-link">View Report</Link>
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
        <div className="detail-stack">
          <h4 className="section-heading">Assigned Devices ({workspaceDevices?.length ?? 0})</h4>

          {(!workspaceDevices || workspaceDevices.length === 0) ? (
            <p className="muted">No devices assigned to this workspace yet.</p>
          ) : (
            <div className="detail-stack">
              {workspaceDevices.map(d => (
                <div key={d.id} className="row-card">
                  <div className="row-card-text">
                    <div className="workspace-card-title">
                      <Cpu size={16} strokeWidth={1.5} />
                      <strong>{d.name}</strong>
                    </div>
                    <small>Type: {d.type} · Version: {d.firmware_version}</small>
                  </div>
                  <div className="row-card-meta">
                    <span className="row-status">
                      <span className="status-dot" />
                      {d.status}
                    </span>
                    <button
                      className="icon-button danger"
                      onClick={() => handleUnassignDevice(d.id)}
                      disabled={unassignDevice.isPending}
                      title="Unassign Device"
                      aria-label={`Unassign ${d.name}`}
                    >
                      <Trash2 size={14} strokeWidth={1.5} />
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
