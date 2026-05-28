import { FormEvent, useState } from "react";
import { Send, Clock, Cpu, Activity } from "lucide-react";
import { CopyableID, EmptyState, PageHeader, Panel, SelectField } from "../components/ui";
import { useDevices } from "../hooks/useArgusData";
import { useRealtime } from "../hooks/useRealtime";
import { api } from "../services/api";
import { safeJsonParse, stringifyJson } from "../lib/format";
import type { Telemetry } from "../types/api";

export function TelemetryPage() {
  const devices = useDevices();
  const realtime = useRealtime();
  const [deviceID, setDeviceID] = useState("");
  const [result, setResult] = useState<Telemetry | null>(null);
  const [error, setError] = useState("");
  
  // Scoped to selected device
  const liveTelemetry = deviceID ? realtime.telemetryByDevice[deviceID] || [] : [];
  const selectedDevice = devices.data?.find(d => d.id === deviceID);

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
        title="Ingest Telemetry"
        description="Ingest, monitor, and stream telemetry payload metrics from registered devices in real time."
        actions={
          <div className={`live-badge ${realtime.status === "connected" ? 'online' : 'offline'}`}>
            <span className="live-dot" />
            {realtime.status === "connected" ? "LIVE STREAM ACTIVE" : `WS: ${realtime.status.toUpperCase()}`}
          </div>
        }
      />
      <div className="split">
        <Panel title="Telemetry Payload" subtitle="Transmit a metrics payload simulation">
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
                <textarea 
                  name="metrics" 
                  defaultValue={JSON.stringify({
                    temp_c: 24.5,
                    humidity_pct: 45,
                    status: "operational"
                  }, null, 2)} 
                  rows={6}
                  style={{ fontFamily: 'var(--font-mono)', fontSize: '12px' }}
                />
              </label>
              {error && <p className="muted field full" style={{ color: 'var(--danger)' }}>{error}</p>}
              <button className="button primary" type="submit" disabled={!deviceID}>
                <Send size={15} /> Submit Telemetry
              </button>
            </form>
          )}
        </Panel>

        <Panel 
          title="Live Telemetry Feed" 
          subtitle={deviceID ? `Streaming history for ${selectedDevice?.name || 'device'}` : "Select a device to monitor stream"}
          actions={
            deviceID && <span className="table-count">{liveTelemetry.length} messages</span>
          }
        >
          {!deviceID ? (
            <EmptyState title="No device selected" description="Select a device above to inspect its live telemetry stream." />
          ) : liveTelemetry.length > 0 ? (
            <div className="event-stream">
              {liveTelemetry.map((t, i) => (
                <div key={t.id || i} className="event-entry" style={{ padding: '12px', borderBottom: '1px solid var(--line-soft)' }}>
                  <div className="flex items-center justify-between mb-2">
                    <div className="flex items-center gap-2">
                      <Activity size={14} className="text-accent" />
                      <strong className="text-sm">Telemetry Packet</strong>
                    </div>
                    <time className="text-[10px] text-faint mono flex items-center gap-1">
                      <Clock size={10} />
                      {new Date(t.recorded_at).toLocaleString()}
                    </time>
                  </div>
                  <pre className="code-block" style={{ margin: 0, fontSize: '11px', maxHeight: '150px', overflow: 'auto' }}>
                    {stringifyJson(t.metrics)}
                  </pre>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Waiting for data..." description={`Live telemetry for ${selectedDevice?.name} will appear here when published.`} />
          )}
        </Panel>
      </div>
    </>
  );
}
