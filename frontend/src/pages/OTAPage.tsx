import { FormEvent, useMemo, useState, useRef } from "react";
import { ArrowDownUp, FileText, RefreshCw, Upload, X as XIcon, Trash2, ChevronDown, ChevronRight, Play } from "lucide-react";
import {
  CopyableID,
  EmptyState,
  ErrorState,
  FilterTabs,
  LoadingRows,
  PageHeader,
  Panel,
  ProgressBar,
  SelectField,
  StatCard,
  StatusChip,
  Modal
} from "../components/ui";
import {
  useAllDeployments,
  useDeploymentEvents,
  useDeployments,
  useDevices,
  useFirmware,
  useFleets,
  useFleetDeploy,
  useOTAStats
} from "../hooks/useArgusData";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { compactID, formatBytes, formatDate, stringifyJson } from "../lib/format";
import { api } from "../services/api";
import type { Deployment, Manifest, FirmwareArtifact } from "../types/api";

const STATUS_FILTERS = ["All", "pending", "available", "downloading", "flashing", "rebooting", "acked", "nacked", "timeout"];
const ACTIVE_STATUSES = new Set(["pending", "available", "downloading", "flashing", "rebooting"]);

function terminalTime(deployment: Deployment) {
  return deployment.completed_at ?? deployment.failed_at ?? deployment.timed_out_at ?? deployment.acknowledged_at;
}

function durationLabel(deployment: Deployment) {
  const end = terminalTime(deployment);
  if (!end) return "In progress";
  const ms = new Date(end).getTime() - new Date(deployment.created_at).getTime();
  if (!Number.isFinite(ms) || ms < 0) return "Unknown";
  const seconds = Math.round(ms / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}m ${rest}s`;
}

function statusProgress(deployment: Deployment) {
  if (deployment.status === "acked") return 100;
  if (deployment.status === "nacked" || deployment.status === "timeout") return deployment.progress ?? 0;
  return deployment.progress ?? 0;
}

function findDeploymentDeviceName(deployment: Deployment) {
  return deployment.device_name || compactID(deployment.device_id);
}

function DeploymentRow({ deployment }: { deployment: Deployment }) {
  const [expanded, setExpanded] = useState(false);
  const timeline = useDeploymentEvents(expanded ? deployment.id : undefined);

  return (
    <div style={{ borderBottom: "1px solid var(--border)", display: "flex", flexDirection: "column" }}>
      <div 
        onClick={() => setExpanded(!expanded)} 
        style={{ display: "flex", alignItems: "center", padding: "8px 16px", cursor: "pointer", gap: 12, background: expanded ? "var(--surface)" : "transparent" }}
      >
        <div style={{ color: "var(--text-muted)", display: "flex", alignItems: "center" }}>
          {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        </div>
        
        <div style={{ display: "flex", alignItems: "center", gap: 16, flex: 1, fontSize: 13 }}>
          <div style={{ width: 120, fontWeight: 500 }}>
            {deployment.version || "Custom Build"}
          </div>
          <div style={{ width: 120, fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>
            {compactID(deployment.artifact_id)}
          </div>
          <div style={{ flex: 1, color: "var(--text-primary)" }}>
            {findDeploymentDeviceName(deployment)}
          </div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 24, fontSize: 12 }}>
          <div style={{ width: 120, textAlign: "right", fontFamily: "var(--font-mono)", color: "var(--text-muted)" }}>{formatDate(deployment.created_at)}</div>
          <div style={{ width: 80, textAlign: "right", color: "var(--text-muted)" }}>{durationLabel(deployment)}</div>
          <div style={{ width: 100, display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 6 }}>
            <span style={{ color: ["completed", "acked", "success"].includes(deployment.status) ? "var(--success)" : ["failed", "cancelled", "timeout", "nacked"].includes(deployment.status) ? "var(--danger)" : "var(--vercel-cyan)", textTransform: "capitalize", fontWeight: 500 }}>
              {deployment.status}
            </span>
          </div>
        </div>
      </div>

      {expanded && (
        <div style={{ padding: "0", borderTop: "1px solid var(--border)", background: "var(--background)", fontFamily: "var(--font-mono)" }}>
          <div style={{ display: "flex", flexDirection: "column" }}>
            {(timeline.data ?? []).map((event) => (
              <div key={event.id} style={{ display: "flex", alignItems: "flex-start", gap: 16, padding: "6px 16px 6px 44px", borderBottom: "1px solid var(--border)", fontSize: 12 }}>
                <div style={{ width: 100, color: "var(--text-muted)" }}>
                  {formatDate(event.created_at).split(",")[1]?.trim()}
                </div>
                <div style={{ width: 100, color: "var(--text-primary)" }}>
                  {event.status}
                </div>
                <div style={{ flex: 1 }}>
                  {event.progress !== undefined && <span style={{ color: "var(--vercel-cyan)", marginRight: 8 }}>[{event.progress}%]</span>}
                  {event.message && <span style={{ color: "var(--text-primary)" }}>{event.message}</span>}
                </div>
              </div>
            ))}
            {timeline.isLoading && <div style={{ padding: "8px 16px 8px 44px", color: "var(--text-muted)", fontSize: 12 }}>Loading timeline...</div>}
            {!timeline.isLoading && timeline.data?.length === 0 && <div style={{ padding: "8px 16px 8px 44px", color: "var(--text-muted)", fontSize: 12 }}>No recorded events</div>}
          </div>
        </div>
      )}
    </div>
  );
}

export function OTAPage() {
  const { workspaceDevices } = useWorkspaceContext();
  const devices = useDevices();
  const firmware = useFirmware();
  const allDeployments = useAllDeployments();
  const stats = useOTAStats();
  const [deviceID, setDeviceID] = useState("");
  const deployments = useDeployments(deviceID);
  const [manifest, setManifest] = useState<Manifest | null>(null);
  const [selectedDeployment, setSelectedDeployment] = useState<Deployment | null>(null);
  const timeline = useDeploymentEvents(selectedDeployment?.id);
  const [error, setError] = useState("");
  const [uploadError, setUploadError] = useState("");
  const [uploadSuccess, setUploadSuccess] = useState("");
  const [deployError, setDeployError] = useState("");
  const [deploySuccess, setDeploySuccess] = useState("");
  const [artifactToDelete, setArtifactToDelete] = useState<FirmwareArtifact | null>(null);
  const [deleteError, setDeleteError] = useState("");
  const [isDeleting, setIsDeleting] = useState(false);
  const uploadTimeoutRef = useRef<number | null>(null);
  const deployTimeoutRef = useRef<number | null>(null);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("All");
  const [sortKey, setSortKey] = useState<"created_at" | "status" | "progress" | "duration">("created_at");

  const fleets = useFleets();
  const fleetDeploy = useFleetDeploy();
  const [fleetID, setFleetID] = useState("");
  const [fleetArtifactID, setFleetArtifactID] = useState("");
  const [fleetDeployError, setFleetDeployError] = useState("");
  const [fleetDeploySuccess, setFleetDeploySuccess] = useState("");
  const fleetDeployTimeoutRef = useRef<number | null>(null);

  const activeDeviceIds = useMemo(() => new Set(workspaceDevices.map(d => d.id)), [workspaceDevices]);

  const deploymentRows = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return [...(allDeployments.data ?? [])]
      .filter((deployment) => activeDeviceIds.has(deployment.device_id))
      .filter((deployment) => statusFilter === "All" || deployment.status === statusFilter)
      .filter((deployment) => {
        if (!needle) return true;
        return [
          deployment.id,
          deployment.device_id,
          deployment.device_name,
          deployment.version,
          deployment.filename,
          deployment.status,
          deployment.result_message,
          deployment.failure_reason
        ].join(" ").toLowerCase().includes(needle);
      })
      .sort((a, b) => {
        if (sortKey === "status") return a.status.localeCompare(b.status);
        if (sortKey === "progress") return statusProgress(b) - statusProgress(a);
        if (sortKey === "duration") {
          const ad = terminalTime(a) ? new Date(terminalTime(a)!).getTime() - new Date(a.created_at).getTime() : Number.MAX_SAFE_INTEGER;
          const bd = terminalTime(b) ? new Date(terminalTime(b)!).getTime() - new Date(b.created_at).getTime() : Number.MAX_SAFE_INTEGER;
          return ad - bd;
        }
        return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
      });
  }, [allDeployments.data, query, sortKey, statusFilter]);

  const selectedDeviceDeployments = deviceID ? deployments.data ?? [] : [];
  const latestDeviceDeployment = selectedDeviceDeployments[0];

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setUploadError("");
    setUploadSuccess("");
    if (uploadTimeoutRef.current) {
      window.clearTimeout(uploadTimeoutRef.current);
    }
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      await api.firmware.upload(form);
      formElement.reset();
      await firmware.refetch();
      setUploadSuccess("Firmware uploaded and registered successfully.");
      uploadTimeoutRef.current = window.setTimeout(() => {
        setUploadSuccess("");
      }, 4500);
    } catch (err) {
      setUploadError((err as Error).message);
    }
  }

  async function handleDeleteFirmware() {
    if (!artifactToDelete) return;
    setDeleteError("");
    setIsDeleting(true);
    try {
      await api.firmware.remove(artifactToDelete.id);
      await firmware.refetch();
      setArtifactToDelete(null);
    } catch (err) {
      setDeleteError((err as Error).message);
    } finally {
      setIsDeleting(false);
    }
  }

  async function deploy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setDeployError("");
    setDeploySuccess("");
    if (deployTimeoutRef.current) {
      window.clearTimeout(deployTimeoutRef.current);
    }
    try {
      if (!deviceID) {
        setDeployError("Select a device before creating a deployment.");
        return;
      }
      const created = await api.deployments.create(deviceID, String(form.get("artifact_id")));
      setManifest(created);
      await Promise.all([deployments.refetch(), allDeployments.refetch(), stats.refetch()]);
      setDeploySuccess("Deployment created successfully.");
      deployTimeoutRef.current = window.setTimeout(() => {
        setDeploySuccess("");
      }, 4500);
    } catch (err) {
      setDeployError((err as Error).message);
    }
  }

  async function viewManifest(deployment: Deployment) {
    setError("");
    try {
      setSelectedDeployment(deployment);
      const res = await api.deployments.manifest(deployment.device_id, deployment.id);
      setManifest(res);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function fleetDeployHandler(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setFleetDeployError("");
    setFleetDeploySuccess("");
    if (fleetDeployTimeoutRef.current) {
      window.clearTimeout(fleetDeployTimeoutRef.current);
    }
    if (!fleetID || !fleetArtifactID) {
      setFleetDeployError("Select both a fleet and an artifact.");
      return;
    }
    try {
      const result = await fleetDeploy.mutateAsync({ fleetID, artifactID: fleetArtifactID });
      await Promise.all([allDeployments.refetch(), stats.refetch()]);
      setFleetDeploySuccess(`${result.deployed_count} of ${result.total_count} devices queued for deployment.`);
      fleetDeployTimeoutRef.current = window.setTimeout(() => {
        setFleetDeploySuccess("");
      }, 5000);
    } catch (err) {
      setFleetDeployError((err as Error).message);
    }
  }

  const totalDeployments = stats.data?.total_deployments ?? 0;
  const successful = stats.data?.successful_deployments ?? 0;
  const failed = stats.data?.failed_deployments ?? 0;
  const successRate = stats.data?.success_rate ?? 0;
  const pendingDevices = stats.data?.devices_pending_update ?? 0;
  const activeCount = (allDeployments.data ?? []).filter((deployment) => ACTIVE_STATUSES.has(deployment.status)).length;

  const [isReleaseModalOpen, setIsReleaseModalOpen] = useState(false);
  const [releaseAction, setReleaseAction] = useState<"deploy" | "upload">("deploy");

  return (
    <>
      <PageHeader
        title="Deployments"
        actions={
          <div style={{ display: "flex", gap: 12 }}>
            <button className="button secondary" onClick={() => { void allDeployments.refetch(); void stats.refetch(); void firmware.refetch(); }}><RefreshCw size={15} />Refresh</button>
            <button className="button secondary" onClick={() => { setReleaseAction("upload"); setIsReleaseModalOpen(true); }}><Upload size={15} /> Upload Artifact</button>
            <button className="button primary" onClick={() => { setReleaseAction("deploy"); setIsReleaseModalOpen(true); }}><Play size={15} /> Deploy Release</button>
          </div>
        }
      />

      <div style={{ maxWidth: 900 }}>
        {/* Artifacts Section */}
        <h3 style={{ marginTop: 0, marginBottom: 16 }}>Available Artifacts</h3>
        <div style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", background: "var(--surface)", overflow: "hidden", marginBottom: 32 }}>
          {firmware.isLoading && <LoadingRows rows={3} />}
          {!firmware.isLoading && (firmware.data?.length === 0 || !firmware.data) && (
            <EmptyState title="No artifacts" description="Upload a firmware artifact to get started." />
          )}
          {firmware.data?.map(artifact => (
            <div key={artifact.id} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "12px 16px", borderBottom: "1px solid var(--border)" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
                <FileText size={16} color="var(--text-muted)" />
                <div style={{ fontWeight: 500 }}>{artifact.version}</div>
                <div style={{ color: "var(--text-muted)", fontSize: 13 }}><CopyableID id={artifact.id} /></div>
                <div style={{ color: "var(--text-muted)", fontSize: 13, fontFamily: "var(--font-mono)" }}>{formatBytes(artifact.size_bytes)}</div>
              </div>
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <div style={{ color: "var(--text-muted)", fontSize: 12 }}>{formatDate(artifact.created_at)}</div>
                <button
                  className="button icon-only"
                  style={{ color: "var(--danger)" }}
                  title="Delete Artifact"
                  onClick={() => setArtifactToDelete(artifact)}
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))}
        </div>

        {/* Fleet OTA Deploy Section */}
        <div style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", background: "var(--surface)", padding: "16px 20px", marginBottom: 32 }}>
          <h3 style={{ marginTop: 0, marginBottom: 16, fontSize: 14, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--text-secondary)" }}>Fleet OTA Deploy</h3>
          <form style={{ display: "flex", gap: 12, alignItems: "flex-end", flexWrap: "wrap" }} onSubmit={fleetDeployHandler}>
            <SelectField label="Fleet" value={fleetID} onChange={setFleetID}>
              <option value="">Select fleet</option>
              {(fleets.data ?? []).map((fleet) => (
                <option key={fleet.id} value={fleet.id}>{fleet.name} ({fleet.total_nodes} nodes)</option>
              ))}
            </SelectField>
            <SelectField label="Artifact" value={fleetArtifactID} onChange={setFleetArtifactID}>
              <option value="">Select artifact</option>
              {(firmware.data ?? []).map((artifact) => (
                <option key={artifact.id} value={artifact.id}>{artifact.version} ({formatBytes(artifact.size_bytes)})</option>
              ))}
            </SelectField>
            <button
              className="button primary"
              type="submit"
              disabled={!fleetID || !fleetArtifactID || fleetDeploy.isPending}
              style={{ height: 36 }}
            >
              <Play size={14} style={{ marginRight: 6 }} />
              {fleetDeploy.isPending ? "Deploying..." : "Deploy to Fleet"}
            </button>
          </form>
          {fleetDeployError && (
            <div style={{ marginTop: 12, padding: "8px 12px", background: "rgba(239, 68, 68, 0.1)", border: "1px solid rgba(239, 68, 68, 0.2)", borderRadius: 6, fontSize: 13, color: "var(--danger)" }}>
              {fleetDeployError}
            </div>
          )}
          {fleetDeploySuccess && (
            <div style={{ marginTop: 12, padding: "8px 12px", background: "rgba(16, 185, 129, 0.1)", border: "1px solid rgba(16, 185, 129, 0.2)", borderRadius: 6, fontSize: 13, color: "var(--success)" }}>
              {fleetDeploySuccess}
            </div>
          )}
        </div>

        {/* Deployments Section */}
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
          <h3 style={{ margin: 0 }}>Deployment Timeline</h3>
        </div>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 24 }}>
          <label className="field" style={{ flex: 1, marginRight: 32 }}>
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Search deployments..." style={{ background: "var(--surface)", border: "1px solid var(--border)", borderRadius: "var(--radius-sm)", width: "100%", maxWidth: "100%" }} />
          </label>
          <div style={{ display: "flex", alignItems: "center" }}>
            <FilterTabs options={STATUS_FILTERS} active={statusFilter} onChange={setStatusFilter} />
          </div>
        </div>

        <div style={{ border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", background: "var(--surface)", overflow: "hidden" }}>
          {allDeployments.isLoading && <LoadingRows rows={6} />}
          {!allDeployments.isLoading && deploymentRows.length === 0 && <EmptyState title="No deployments" description="Create a deployment to populate the release timeline." />}
          {deploymentRows.map(deployment => (
            <DeploymentRow key={deployment.id} deployment={deployment} />
          ))}
        </div>
      </div>

      <Modal isOpen={isReleaseModalOpen} onClose={() => setIsReleaseModalOpen(false)} title={releaseAction === "deploy" ? "Deploy Release" : "Upload Firmware Artifact"}>
        {releaseAction === "deploy" ? (
          <form className="form-grid" onSubmit={(e) => { deploy(e).then(() => { if (!deployError) setIsReleaseModalOpen(false); }) }}>
            <div className="field full"><SelectField label="Device" value={deviceID} onChange={setDeviceID}><option value="">Select device</option>{workspaceDevices.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}</SelectField></div>
            <label className="field full"><span>Firmware Artifact</span><select name="artifact_id">{firmware.data?.map((artifact) => <option key={artifact.id} value={artifact.id}>{artifact.version} ({formatBytes(artifact.size_bytes)})</option>)}</select></label>
            {deployError && <div className="form-message error field full">{deployError}</div>}
            <div className="modal-actions">
              <button type="button" className="button secondary" onClick={() => setIsReleaseModalOpen(false)}>Cancel</button>
              <button className="button primary" type="submit" disabled={!deviceID || !firmware.data?.length}>Deploy Release</button>
            </div>
          </form>
        ) : (
          <form className="form-grid" onSubmit={(e) => { upload(e).then(() => { if (!uploadError) setIsReleaseModalOpen(false); }) }}>
            <label className="field full"><span>Version</span><input name="version" placeholder="v1.4.0" required /></label>
            <label className="field full"><span>Binary File</span><input name="firmware" type="file" required /></label>
            {uploadError && <div className="form-message error field full">{uploadError}</div>}
            <div className="modal-actions">
              <button type="button" className="button secondary" onClick={() => setIsReleaseModalOpen(false)}>Cancel</button>
              <button className="button secondary" type="submit"><Upload size={14} style={{ marginRight: 6 }} /> Upload</button>
            </div>
          </form>
        )}
      </Modal>

      <Modal isOpen={!!artifactToDelete} onClose={() => setArtifactToDelete(null)} title="Delete Artifact">
        <div style={{ marginBottom: 24, fontSize: 14 }}>
          Are you sure you want to delete the firmware artifact <strong>{artifactToDelete?.version}</strong>? This cannot be undone, but existing deployments using this artifact will not be affected on devices that have already downloaded it.
        </div>
        {deleteError && <div className="form-message error field full">{deleteError}</div>}
        <div className="modal-actions">
          <button type="button" className="button secondary" onClick={() => setArtifactToDelete(null)} disabled={isDeleting}>Cancel</button>
          <button className="button primary" style={{ background: "var(--danger)", borderColor: "var(--danger)" }} onClick={handleDeleteFirmware} disabled={isDeleting}>
            {isDeleting ? "Deleting..." : "Delete Artifact"}
          </button>
        </div>
      </Modal>

    </>
  );
}
