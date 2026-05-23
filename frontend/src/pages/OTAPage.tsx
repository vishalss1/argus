import { FormEvent, useEffect, useState } from "react";
import { Upload, FileText, Check, X as XIcon } from "lucide-react";
import { EmptyState, ErrorState, LoadingRows, PageHeader, Panel, SelectField, StatusChip } from "../components/ui";
import { useDeployments, useDevices, useFirmware } from "../hooks/useArgusData";
import { compactID, formatBytes, formatDate, stringifyJson } from "../lib/format";
import { api } from "../services/api";
import type { Manifest } from "../types/api";

export function OTAPage() {
  const devices = useDevices();
  const firmware = useFirmware();
  const [deviceID, setDeviceID] = useState("");
  const deployments = useDeployments(deviceID);
  const [manifest, setManifest] = useState<Manifest | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!deviceID && devices.data?.[0]) setDeviceID(devices.data[0].id);
  }, [deviceID, devices.data]);

  async function upload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    try {
      await api.firmware.upload(form);
      event.currentTarget.reset();
      await firmware.refetch();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function deploy(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setManifest(await api.deployments.create(deviceID, String(form.get("artifact_id"))));
    await deployments.refetch();
  }

  async function viewManifest(deploymentID: string) {
    try {
      const res = await api.deployments.manifest(deviceID, deploymentID);
      setManifest(res);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function record(deploymentID: string, action: "ack" | "nack") {
    if (action === "ack") await api.deployments.ack(deviceID, deploymentID);
    else await api.deployments.nack(deviceID, deploymentID);
    await deployments.refetch();
  }

  return (
    <>
      <PageHeader eyebrow="OTA" title="OTA Updates" description="Manage firmware artifacts and create real deployment manifests for registered devices." />
      <div className="split">
        <Panel title="Firmware Artifacts" subtitle="GET /ota/firmware">
          {firmware.isError ? <ErrorState message={(firmware.error as Error).message} onRetry={() => void firmware.refetch()} /> : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Version</th><th>Filename</th><th>Size</th><th>Created</th></tr></thead>
                <tbody>
                  {firmware.isLoading && <LoadingRows rows={4} />}
                  {!firmware.isLoading && (firmware.data?.length ?? 0) === 0 && <tr><td colSpan={4}><EmptyState title="No firmware artifacts" description="Upload firmware to register artifacts for OTA deployment." /></td></tr>}
                  {firmware.data?.map((artifact) => <tr key={artifact.id}><td>{artifact.version}</td><td>{artifact.filename}<div className="muted mono">{compactID(artifact.id)}</div></td><td>{formatBytes(artifact.size_bytes)}</td><td>{formatDate(artifact.created_at)}</td></tr>)}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <div className="grid">
          <Panel title="Upload Firmware" subtitle="POST /ota/firmware">
            <form className="form-grid" onSubmit={upload}>
              <label className="field full"><span>Version</span><input name="version" required /></label>
              <label className="field full"><span>Firmware File</span><input name="firmware" type="file" required /></label>
              {error && <p className="muted field full">{error}</p>}
              <button className="button primary" type="submit"><Upload size={15} />Upload</button>
            </form>
          </Panel>
          <Panel title="Create Deployment" subtitle="POST /devices/{deviceID}/ota">
            {(devices.data?.length ?? 0) === 0 || (firmware.data?.length ?? 0) === 0 ? <EmptyState title="Deployment unavailable" description="A deployment requires at least one device and one firmware artifact." /> : (
              <form className="form-grid" onSubmit={deploy}>
                <div className="field full"><SelectField label="Device" value={deviceID} onChange={setDeviceID}>{devices.data?.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}</SelectField></div>
                <label className="field full"><span>Firmware Artifact</span><select name="artifact_id">{firmware.data?.map((artifact) => <option key={artifact.id} value={artifact.id}>{artifact.version} · {artifact.filename}</option>)}</select></label>
                <button className="button primary" type="submit">Create Manifest</button>
              </form>
            )}
            {manifest && (
              <div style={{ marginTop: 14 }}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                  <h4 style={{ margin: 0 }}>Manifest Output</h4>
                  <button className="button compact secondary" onClick={() => setManifest(null)}><XIcon size={14} /></button>
                </div>
                <pre className="code-block" style={{ margin: 0 }}>{stringifyJson(manifest)}</pre>
              </div>
            )}
          </Panel>
        </div>
      </div>
      <div style={{ marginTop: 18 }}>
        <Panel title="Device Deployments" subtitle="GET /devices/{deviceID}/ota">
          {!deviceID ? <EmptyState title="No device selected" description="Select a device to inspect OTA deployments." /> : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Deployment</th><th>Artifact</th><th>Status</th><th>Created</th><th>Actions</th></tr></thead>
                <tbody>
                  {deployments.isLoading && <LoadingRows rows={4} />}
                  {!deployments.isLoading && (deployments.data?.length ?? 0) === 0 && <tr><td colSpan={5}><EmptyState title="No deployments" description="No OTA deployments exist for this device." /></td></tr>}
                  {deployments.data?.map((deployment) => (
                    <tr key={deployment.id}>
                      <td className="mono">{compactID(deployment.id)}</td>
                      <td className="mono">{compactID(deployment.artifact_id)}</td>
                      <td><StatusChip value={deployment.status} /></td>
                      <td>{formatDate(deployment.created_at)}</td>
                      <td>
                        <div className="page-actions">
                          <button className="button compact secondary" onClick={() => void viewManifest(deployment.id)} aria-label="View Manifest"><FileText size={14} /></button>
                          <button className="button compact secondary" onClick={() => void record(deployment.id, "ack")} aria-label="ACK"><Check size={14} /></button>
                          <button className="button compact danger" onClick={() => void record(deployment.id, "nack")} aria-label="NACK"><XIcon size={14} /></button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </>
  );
}
