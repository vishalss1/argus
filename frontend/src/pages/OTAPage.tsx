import { FormEvent, useMemo, useState, useRef } from "react";
import { ArrowDownUp, FileText, RefreshCw, Upload, X as XIcon } from "lucide-react";
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
  StatusChip
} from "../components/ui";
import {
  useAllDeployments,
  useDeploymentEvents,
  useDeployments,
  useDevices,
  useFirmware,
  useOTAStats
} from "../hooks/useArgusData";
import { compactID, formatBytes, formatDate, stringifyJson } from "../lib/format";
import { api } from "../services/api";
import type { Deployment, Manifest } from "../types/api";

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

export function OTAPage() {
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
  const uploadTimeoutRef = useRef<number | null>(null);
  const deployTimeoutRef = useRef<number | null>(null);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("All");
  const [sortKey, setSortKey] = useState<"created_at" | "status" | "progress" | "duration">("created_at");

  const deploymentRows = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return [...(allDeployments.data ?? [])]
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
        setDeployError("Select a device before creating an OTA deployment.");
        return;
      }
      const created = await api.deployments.create(deviceID, String(form.get("artifact_id")));
      setManifest(created);
      await Promise.all([deployments.refetch(), allDeployments.refetch(), stats.refetch()]);
      setDeploySuccess("OTA deployment created successfully.");
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

  const totalDeployments = stats.data?.total_deployments ?? 0;
  const successful = stats.data?.successful_deployments ?? 0;
  const failed = stats.data?.failed_deployments ?? 0;
  const successRate = stats.data?.success_rate ?? 0;
  const pendingDevices = stats.data?.devices_pending_update ?? 0;
  const activeCount = (allDeployments.data ?? []).filter((deployment) => ACTIVE_STATUSES.has(deployment.status)).length;

  return (
    <>
      <PageHeader
        eyebrow="OTA Deployment Lifecycle"
        title="OTA Updates"
        description="Track firmware rollout state, progress, history, failures, and deployment timing across the fleet."
        actions={<button className="button secondary" onClick={() => { void allDeployments.refetch(); void stats.refetch(); }}><RefreshCw size={15} />Refresh</button>}
      />

      <div className="stat-grid five">
        <StatCard label="Total Deployments" value={totalDeployments} detail="All time" />
        <StatCard label="Successful" value={successful} detail="ACK received" tone="success" />
        <StatCard label="Failed" value={failed} detail="NACK or timeout" tone={failed > 0 ? "danger" : "neutral"} />
        <StatCard label="Success Rate" value={`${successRate.toFixed(1)}%`} detail="Completed / total" />
        <StatCard label="Pending Update" value={pendingDevices} detail={`${activeCount} active jobs`} tone={activeCount > 0 ? "warning" : "neutral"} />
      </div>

      <div className="split">
        <Panel title="Firmware Artifacts" subtitle="Registered binaries available for rollout">
          {firmware.isError ? <ErrorState message={(firmware.error as Error).message} onRetry={() => void firmware.refetch()} /> : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Version</th><th>Filename</th><th>Size</th><th>Uploaded</th></tr></thead>
                <tbody>
                  {firmware.isLoading && <LoadingRows rows={4} />}
                  {!firmware.isLoading && (firmware.data?.length ?? 0) === 0 && <tr><td colSpan={4}><EmptyState title="No firmware artifacts" description="Upload firmware to create deployment manifests." /></td></tr>}
                  {firmware.data?.map((artifact) => (
                    <tr key={artifact.id}>
                      <td><strong>{artifact.version}</strong><div className="mono muted">{compactID(artifact.id)}</div></td>
                      <td>{artifact.filename}</td>
                      <td>{formatBytes(artifact.size_bytes)}</td>
                      <td>{formatDate(artifact.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <div className="grid">
          <Panel title="Upload Firmware" subtitle="Register a signed production binary">
            <form className="form-grid" onSubmit={upload}>
              <label className="field full"><span>Version</span><input name="version" placeholder="v1.4.0" required /></label>
              <label className="field full"><span>Firmware File</span><input name="firmware" type="file" required /></label>
              {uploadError && <div className="form-message error field full">{uploadError}</div>}
              {uploadSuccess && <div className="form-message success field full">{uploadSuccess}</div>}
              <button className="button primary" type="submit"><Upload size={15} />Upload</button>
            </form>
          </Panel>
          <Panel title="Create Deployment" subtitle="Create a pull-based deployment for one device">
            {(devices.data?.length ?? 0) === 0 || (firmware.data?.length ?? 0) === 0 ? <EmptyState title="Deployment unavailable" description="A deployment requires at least one device and one firmware artifact." /> : (
              <form className="form-grid" onSubmit={deploy}>
                <div className="field full"><SelectField label="Device" value={deviceID} onChange={setDeviceID}><option value="">Select device</option>{devices.data?.map((device) => <option key={device.id} value={device.id}>{device.name} · {compactID(device.id)} · {device.firmware_version || "unset"}</option>)}</SelectField></div>
                <label className="field full"><span>Firmware Artifact</span><select name="artifact_id">{firmware.data?.map((artifact) => <option key={artifact.id} value={artifact.id}>{artifact.version} · {artifact.filename}</option>)}</select></label>
                {deviceID && <div className="field full"><span>Selected Device ID</span><CopyableID id={deviceID} /></div>}
                {deployError && <div className="form-message error field full">{deployError}</div>}
                {deploySuccess && <div className="form-message success field full">{deploySuccess}</div>}
                <button className="button primary" type="submit" disabled={!deviceID}>Create Deployment</button>
              </form>
            )}
          </Panel>
        </div>
      </div>

      <div style={{ marginTop: 18 }}>
        <Panel
          title="Deployment Table"
          subtitle={`${deploymentRows.length} visible`}
          actions={<FilterTabs options={STATUS_FILTERS} active={statusFilter} onChange={setStatusFilter} />}
        >
          <div className="table-toolbar">
            <label className="field"><span>Search</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Device, version, status, reason" /></label>
            <label className="field"><span>Sort</span><select value={sortKey} onChange={(event) => setSortKey(event.target.value as typeof sortKey)}><option value="created_at">Newest</option><option value="status">Status</option><option value="progress">Progress</option><option value="duration">Duration</option></select></label>
          </div>
          <div className="table-wrap">
            <table>
              <thead><tr><th>Device</th><th>Version</th><th>Status</th><th>Progress</th><th>Started</th><th>Completed</th><th>Duration</th><th>Actions</th></tr></thead>
              <tbody>
                {allDeployments.isLoading && <LoadingRows rows={6} />}
                {!allDeployments.isLoading && deploymentRows.length === 0 && <tr><td colSpan={8}><EmptyState title="No deployments" description="Create a deployment to populate the lifecycle table." /></td></tr>}
                {deploymentRows.map((deployment) => (
                  <tr key={deployment.id}>
                    <td><strong>{findDeploymentDeviceName(deployment)}</strong><div><CopyableID id={deployment.device_id} /></div></td>
                    <td>{deployment.version || compactID(deployment.artifact_id)}<div className="muted">{deployment.filename}</div></td>
                    <td><StatusChip value={deployment.status} /></td>
                    <td><div style={{ minWidth: 140 }}><ProgressBar value={statusProgress(deployment)} max={100} color={deployment.status === "acked" ? "var(--success)" : deployment.status === "nacked" || deployment.status === "timeout" ? "var(--danger)" : "var(--warning)"} /><span className="muted" style={{ fontSize: 11 }}>{statusProgress(deployment)}%</span></div></td>
                    <td>{formatDate(deployment.created_at)}</td>
                    <td>{formatDate(terminalTime(deployment))}</td>
                    <td>{durationLabel(deployment)}</td>
                    <td><button className="button compact secondary" onClick={() => void viewManifest(deployment)}><FileText size={14} />Details</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
      </div>

      <div className="split" style={{ marginTop: 18 }}>
        <Panel title="Device OTA History" subtitle={deviceID ? "Latest deployments for the selected device" : "Select a device"}>
          <div className="field" style={{ marginBottom: 14 }}><SelectField label="Device" value={deviceID} onChange={setDeviceID}><option value="">Select device</option>{devices.data?.map((device) => <option key={device.id} value={device.id}>{device.name} · {compactID(device.id)}</option>)}</SelectField></div>
          {latestDeviceDeployment && (
            <div className="settings-row" style={{ marginBottom: 12 }}>
              <span><strong>Latest Firmware Target</strong><p className="muted">{latestDeviceDeployment.version || "Unknown"} · {latestDeviceDeployment.status}</p></span>
              <StatusChip value={latestDeviceDeployment.status} />
            </div>
          )}
          <div className="table-wrap">
            <table>
              <thead><tr><th>Version</th><th>Started</th><th>Duration</th><th>Outcome</th><th>Reason</th></tr></thead>
              <tbody>
                {deployments.isLoading && <LoadingRows rows={4} />}
                {!deployments.isLoading && selectedDeviceDeployments.length === 0 && <tr><td colSpan={5}><EmptyState title="No OTA history" description="This device has not received an OTA deployment." /></td></tr>}
                {selectedDeviceDeployments.map((deployment) => (
                  <tr key={deployment.id}>
                    <td>{deployment.version || compactID(deployment.artifact_id)}</td>
                    <td>{formatDate(deployment.created_at)}</td>
                    <td>{durationLabel(deployment)}</td>
                    <td><StatusChip value={deployment.status} /></td>
                    <td>{deployment.failure_reason || deployment.result_message || "None"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Panel>
        <Panel
          title="Deployment Timeline"
          subtitle={selectedDeployment ? compactID(selectedDeployment.id) : "Select Details from the deployment table"}
          actions={selectedDeployment && <button className="button compact secondary" onClick={() => { setSelectedDeployment(null); setManifest(null); setError(""); }}><XIcon size={14} /></button>}
        >
          {error && <div className="form-message error field full" style={{ marginBottom: 14 }}>{error}</div>}
          {!selectedDeployment ? <EmptyState title="No deployment selected" description="Open a deployment to inspect lifecycle timestamps and diagnostics." /> : (
            <>
              <div className="ota-timeline">
                {(timeline.data ?? []).map((event) => (
                  <div className="ota-timeline-row" key={event.id}>
                    <span className="timeline-dot" />
                    <div>
                      <strong>{event.status}</strong>
                      <p className="muted">{formatDate(event.created_at)}{event.progress !== undefined ? ` · ${event.progress}%` : ""}</p>
                      {event.message && <p>{event.message}</p>}
                    </div>
                  </div>
                ))}
                {!timeline.isLoading && (timeline.data?.length ?? 0) === 0 && <EmptyState title="No timeline events" description="No lifecycle events have been recorded for this deployment." />}
              </div>
              {manifest && (
                <div style={{ marginTop: 14 }}>
                  <div className="settings-row"><strong>Manifest</strong><CopyableID id={manifest.deployment_id} /></div>
                  <pre className="code-block" style={{ margin: 0 }}>{stringifyJson(manifest)}</pre>
                </div>
              )}
            </>
          )}
        </Panel>
      </div>

      <div style={{ marginTop: 18 }}>
        <Panel title="Expected UI Flow" subtitle="Operator deployment lifecycle">
          <div className="ota-flow">
            {["Deployment Created", "Manifest Available", "Downloading", "Flashing", "Rebooting", "ACK Received", "Completed"].map((step, index) => (
              <div className="ota-flow-step" key={step}>
                <span>{index + 1}</span>
                <strong>{step}</strong>
              </div>
            ))}
          </div>
        </Panel>
      </div>
    </>
  );
}
