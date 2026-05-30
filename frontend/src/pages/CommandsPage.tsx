import { FormEvent, useEffect, useState } from "react";
import { Send } from "lucide-react";
import { EmptyState, ErrorState, LoadingRows, PageHeader, Panel, SelectField, StatusChip } from "../components/ui";
import { useCommands, useDevices } from "../hooks/useArgusData";
import { compactID, formatDate, safeJsonParse, stringifyJson } from "../lib/format";
import { api } from "../services/api";

export function CommandsPage() {
  const devices = useDevices();
  const [deviceID, setDeviceID] = useState("");
  const commands = useCommands(deviceID);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!deviceID && devices.data?.[0]) setDeviceID(devices.data[0].id);
  }, [deviceID, devices.data]);

  async function sendCommand(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    setError("");
    const form = new FormData(formElement);
    try {
      await api.commands.send(deviceID, {
        type: String(form.get("type") || ""),
        payload: safeJsonParse(String(form.get("payload") || "{}"))
      });
      formElement.reset();
      await commands.refetch();
    } catch (err) {
      setError((err as Error).message);
    }
  }

  return (
    <>
      <PageHeader eyebrow="Command Control" title="Commands" description="Send device commands and track actual command acknowledgement state." />
      <div className="split">
        <Panel title="Device Commands" subtitle="Historical logs of sent commands">
          <div className="field" style={{ marginBottom: 14 }}>
            <SelectField label="Device" value={deviceID} onChange={setDeviceID}>
              <option value="">Select a device</option>
              {devices.data?.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}
            </SelectField>
          </div>
          {!deviceID ? (
            <EmptyState title="No device selected" description="Commands are scoped to a registered device." />
          ) : commands.isError ? (
            <ErrorState message={(commands.error as Error).message} onRetry={() => void commands.refetch()} />
          ) : (
            <div className="table-wrap">
              <table>
                <thead><tr><th>Command</th><th>Type</th><th>Status</th><th>Created</th></tr></thead>
                <tbody>
                  {commands.isLoading && <LoadingRows rows={5} />}
                  {!commands.isLoading && (commands.data?.length ?? 0) === 0 && <tr><td colSpan={4}><EmptyState title="No commands" description="No commands have been sent to this device." /></td></tr>}
                  {commands.data?.map((command) => (
                    <tr key={command.id}>
                      <td><span className="mono">{compactID(command.id)}</span><pre className="code-block">{stringifyJson(command.payload)}</pre></td>
                      <td>{command.type}</td>
                      <td><StatusChip value={command.status} /></td>
                      <td>{formatDate(command.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
        <Panel title="Send Command" subtitle="Transmit a control command to the device">
          <form className="form-grid" onSubmit={sendCommand}>
            <label className="field full"><span>Command Type</span><input name="type" required /></label>
            <label className="field full"><span>Payload JSON</span><textarea name="payload" defaultValue="{}" /></label>
            {error && <p className="muted field full">{error}</p>}
            <button className="button primary" type="submit" disabled={!deviceID}><Send size={15} />Send Command</button>
          </form>
        </Panel>
      </div>
    </>
  );
}
