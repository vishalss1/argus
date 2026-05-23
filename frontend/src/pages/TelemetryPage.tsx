import { FormEvent, useState } from "react";
import { Send } from "lucide-react";
import { CopyableID, EmptyState, PageHeader, Panel, SelectField } from "../components/ui";
import { useDevices } from "../hooks/useArgusData";
import { api } from "../services/api";
import { safeJsonParse, stringifyJson } from "../lib/format";
import type { Telemetry } from "../types/api";

export function TelemetryPage() {
  const devices = useDevices();
  const [deviceID, setDeviceID] = useState("");
  const [result, setResult] = useState<Telemetry | null>(null);
  const [error, setError] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setResult(null);
    const form = new FormData(event.currentTarget);
    try {
      const created = await api.telemetry.ingest(deviceID, {
        metrics: safeJsonParse(String(form.get("metrics") || "{}"))
      }) as Telemetry;
      setResult(created);
      await devices.refetch();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Telemetry"
        title="Ingest telemetry"
        description="The backend exposes telemetry ingestion, not a historical telemetry listing endpoint. This page submits real telemetry and shows the created record returned by the API."
      />
      <div className="split">
        <Panel title="Telemetry Payload" subtitle="POST /devices/{deviceID}/telemetry">
          {(devices.data?.length ?? 0) === 0 ? (
            <EmptyState title="No devices available" description="Telemetry requires a registered device. Create a device before ingesting metrics." />
          ) : (
            <form className="form-grid" onSubmit={submit}>
              <div className="field full">
                <SelectField label="Device" value={deviceID} onChange={setDeviceID}>
                  <option value="">Select a device</option>
                  {devices.data?.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}
                </SelectField>
              </div>
              <label className="field full">
                <span>Metrics JSON</span>
                <textarea name="metrics" defaultValue="{}" />
              </label>
              {error && <p className="muted field full">{error}</p>}
              <button className="button primary" type="submit" disabled={!deviceID}>
                <Send size={15} /> Submit Telemetry
              </button>
            </form>
          )}
        </Panel>
        <Panel title="Created Record" subtitle="API response">
          {result ? (
            <>
              <div className="settings-row" style={{ marginBottom: 12 }}>
                <strong>Device ID</strong>
                <CopyableID id={result.device_id} />
              </div>
              <pre className="code-block">{stringifyJson(result)}</pre>
            </>
          ) : (
            <EmptyState title="No telemetry submitted" description="Submit telemetry to see the backend response. No synthetic telemetry history is rendered." />
          )}
        </Panel>
      </div>
    </>
  );
}
