import { FormEvent, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Activity,
  Clock,
  Cpu,
  Send,
  Battery,
  Thermometer,
  Droplets,
  Wifi,
  Workflow,
  AlertTriangle,
  ArrowRight,
  Database,
  Play,
  X,
  Info,
  Server,
  Code,
  LineChart,
  Terminal,
  Layers
} from "lucide-react";
import { CopyableID, EmptyState, PageHeader, Panel, SelectField, StatusChip } from "../components/ui";
import { useDevices, useAlerts, useCommands, useDeployments } from "../hooks/useArgusData";
import { useRealtime } from "../hooks/useRealtime";
import { api } from "../services/api";
import { safeJsonParse, stringifyJson, compactID } from "../lib/format";
import type { Telemetry, Device, Command, Deployment } from "../types/api";

interface ChartProps {
  data: Telemetry[];
  metricKey: string;
  title: string;
  unit: string;
  color: string;
  timeframe: "15m" | "1h" | "24h" | "7d";
}

function TelemetryChart({ data, metricKey, title, unit, color, timeframe }: ChartProps) {
  const now = Date.now();
  const cutoffs = {
    "15m": 15 * 60 * 1000,
    "1h": 60 * 60 * 1000,
    "24h": 24 * 60 * 60 * 1000,
    "7d": 7 * 24 * 60 * 60 * 1000
  };

  const filteredData = useMemo(() => {
    const cutoff = cutoffs[timeframe];
    return [...data].reverse().filter(t => {
      const recTime = new Date(t.recorded_at).getTime();
      return now - recTime <= cutoff;
    });
  }, [data, timeframe]);

  const values = useMemo(() => {
    return filteredData.map(d => Number((d.metrics as any)?.[metricKey] ?? 0));
  }, [filteredData, metricKey]);

  const { minVal, maxVal, range } = useMemo(() => {
    const min = values.length > 0 ? Math.min(...values) : 0;
    const max = values.length > 0 ? Math.max(...values) : 100;
    const rng = max - min || 1;
    return {
      minVal: min,
      maxVal: max,
      range: rng
    };
  }, [values]);

  if (filteredData.length === 0) {
    return (
      <div className="chart-card-obs" style={{ height: 215, display: "grid", placeItems: "center" }}>
        <div style={{ textAlign: "center", color: "var(--muted)" }}>
          <Info size={20} style={{ marginBottom: 6, opacity: 0.5 }} />
          <p style={{ margin: 0, fontSize: 12 }}>No {title} data in this window</p>
        </div>
      </div>
    );
  }

  const minPadding = minVal - range * 0.1;
  const maxPadding = maxVal + range * 0.1;
  const rangeWithPadding = maxPadding - minPadding || 1;

  const width = 500;
  const height = 180;
  const padding = { left: 45, right: 15, top: 15, bottom: 25 };
  const chartWidth = width - padding.left - padding.right;
  const chartHeight = height - padding.top - padding.bottom;

  const points = filteredData.map((d, index) => {
    const val = Number((d.metrics as any)?.[metricKey] ?? 0);
    const x = padding.left + (index / (filteredData.length - 1 || 1)) * chartWidth;
    const y = padding.top + chartHeight - ((val - minPadding) / rangeWithPadding) * chartHeight;
    return { x, y, value: val, time: d.recorded_at };
  });

  const linePath = points.map((p, i) => `${i === 0 ? "M" : "L"} ${p.x} ${p.y}`).join(" ");
  const areaPath = `${linePath} L ${points[points.length - 1].x} ${padding.top + chartHeight} L ${points[0].x} ${padding.top + chartHeight} Z`;

  const firstTime = new Date(filteredData[0].recorded_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  const lastTime = new Date(filteredData[filteredData.length - 1].recorded_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });

  const currentVal = values[values.length - 1];

  return (
    <div className="chart-card-obs">
      <div className="chart-card-obs-header">
        <div>
          <h3 style={{ fontSize: 13, color: "var(--muted)", fontWeight: 600 }}>{title}</h3>
          <div style={{ display: "flex", alignItems: "baseline", gap: 4, marginTop: 4 }}>
            <span style={{ fontSize: 20, fontWeight: 700 }}>{currentVal.toFixed(1)}</span>
            <span style={{ fontSize: 11, color: "var(--muted)" }}>{unit}</span>
          </div>
        </div>
      </div>
      
      <svg viewBox={`0 0 ${width} ${height}`} style={{ width: "100%", height: "auto", overflow: "visible" }}>
        <defs>
          <linearGradient id={`gradient-${metricKey}`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={color} stopOpacity="0.2" />
            <stop offset="100%" stopColor={color} stopOpacity="0.0" />
          </linearGradient>
        </defs>

        {/* Gridlines */}
        {[0, 0.5, 1].map((ratio) => {
          const y = padding.top + ratio * chartHeight;
          const gridVal = maxPadding - ratio * rangeWithPadding;
          return (
            <g key={ratio} opacity="0.12">
              <line x1={padding.left} y1={y} x2={width - padding.right} y2={y} stroke="var(--text)" strokeDasharray="3 3" />
              <text x={padding.left - 8} y={y + 4} textAnchor="end" fontSize="10" fill="var(--text)">
                {gridVal.toFixed(0)}
              </text>
            </g>
          );
        })}

        {/* Time bounds */}
        <text x={padding.left} y={height - 6} fontSize="10" fill="var(--muted)" opacity="0.8">
          {firstTime}
        </text>
        <text x={width - padding.right} y={height - 6} textAnchor="end" fontSize="10" fill="var(--muted)" opacity="0.8">
          {lastTime}
        </text>

        {/* Shaded Area */}
        <path d={areaPath} fill={`url(#gradient-${metricKey})`} />

        {/* Line Path */}
        <path d={linePath} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />

        {/* Interactive Dots */}
        {points.map((p, idx) => (
          <circle
            key={idx}
            cx={p.x}
            cy={p.y}
            r="3.5"
            fill={color}
            stroke="var(--surface)"
            strokeWidth="1"
            style={{ cursor: "pointer" }}
          >
            <title>{`${p.value.toFixed(1)} ${unit} at ${new Date(p.time).toLocaleTimeString()}`}</title>
          </circle>
        ))}
      </svg>
    </div>
  );
}

function formatUptime(seconds: number): string {
  if (!seconds || !Number.isFinite(seconds)) return "Unknown";
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  const remainingMinutes = minutes % 60;
  if (days > 0) {
    return `${days}d ${remainingHours}h`;
  }
  return `${hours}h ${remainingMinutes}m`;
}

function BatteryIcon({ level }: { level: number }) {
  if (level > 80) return <Battery size={15} style={{ color: "var(--success)" }} />;
  if (level > 40) return <Battery size={15} style={{ color: "var(--warning)" }} />;
  return <Battery size={15} style={{ color: "var(--danger)" }} />;
}

function RSSISignal({ rssi }: { rssi: number }) {
  let bars = 0;
  if (rssi > -60) bars = 4;
  else if (rssi > -70) bars = 3;
  else if (rssi > -80) bars = 2;
  else if (rssi > -90) bars = 1;
  return (
    <div style={{ display: "inline-flex", gap: 2, alignItems: "flex-end", height: 12 }}>
      {[1, 2, 3, 4].map(b => (
        <span
          key={b}
          style={{
            display: "block",
            width: 3,
            height: b * 3,
            borderRadius: 1,
            backgroundColor: b <= bars ? "var(--success)" : "var(--line)"
          }}
        />
      ))}
    </div>
  );
}

export function TelemetryPage() {
  const devices = useDevices();
  const alerts = useAlerts();
  const realtime = useRealtime();

  const [deviceID, setDeviceID] = useState("");
  const [timeframe, setTimeframe] = useState<"15m" | "1h" | "24h" | "7d">("1h");
  const [showSimulation, setShowSimulation] = useState(false);
  const [showRawPayload, setShowRawPayload] = useState(false);
  const [simError, setSimError] = useState("");
  const [simSuccess, setSimSuccess] = useState("");

  const deployments = useDeployments(deviceID);
  const commands = useCommands(deviceID);

  const semanticEvents = useQuery({
    queryKey: ["device-semantic-events", deviceID],
    queryFn: () => api.ai.listDeviceEvents(deviceID),
    enabled: Boolean(deviceID)
  });

  const deviceHistory = useQuery({
    queryKey: ["device-history", deviceID],
    queryFn: () => api.ai.getDeviceHistory(deviceID),
    enabled: Boolean(deviceID)
  });

  // Fleet stats computation
  const totalFleetCount = devices.data?.length ?? 0;
  const onlineCount = devices.data?.filter(d => d.status === "online").length ?? 0;
  const offlineCount = devices.data?.filter(d => d.status === "offline").length ?? 0;
  
  const activeAlertsCount = alerts.data?.length ?? 0;

  const totalMessageCount = useMemo(() => {
    return Object.values(realtime.telemetryByDevice).reduce((acc, curr) => acc + curr.length, 0);
  }, [realtime.telemetryByDevice]);

  const avgRSSI = useMemo(() => {
    let sum = 0;
    let count = 0;
    Object.values(realtime.telemetryByDevice).forEach(packets => {
      if (packets.length > 0) {
        const last = packets[0];
        const val = Number((last.metrics as any)?.rssi);
        if (Number.isFinite(val)) {
          sum += val;
          count++;
        }
      }
    });
    return count > 0 ? Math.round(sum / count) : -65;
  }, [realtime.telemetryByDevice]);

  const firmwareDistribution = useMemo(() => {
    const counts: Record<string, number> = {};
    devices.data?.forEach(d => {
      const v = d.firmware_version || "Unknown";
      counts[v] = (counts[v] || 0) + 1;
    });
    const sorted = Object.entries(counts).sort((a, b) => b[1] - a[1]);
    if (sorted.length === 0) return "N/A";
    return `${sorted[0][0]} (${sorted[0][1]} dev)`;
  }, [devices.data]);

  // Selected device scoping
  const liveTelemetry = deviceID ? realtime.telemetryByDevice[deviceID] || [] : [];
  const selectedDevice = devices.data?.find(d => d.id === deviceID);

  const latestTelemetry = liveTelemetry[0];
  const latestMetrics = (latestTelemetry?.metrics as any) || {};

  // Extract metric values
  const currentTemp = latestMetrics.temp_c ?? latestMetrics.temperature ?? null;
  const currentHumidity = latestMetrics.humidity_pct ?? latestMetrics.humidity ?? null;
  const currentRSSI = latestMetrics.rssi ?? null;
  const currentBattery = latestMetrics.battery_level ?? latestMetrics.battery ?? null;
  const currentCPU = latestMetrics.cpu_load ?? null;
  const currentHeap = latestMetrics.free_heap ?? null;
  const currentBlock = latestMetrics.largest_free_block ?? null;
  const currentUptime = latestMetrics.uptime_s ?? null;
  const currentStatus = latestMetrics.status ?? selectedDevice?.status ?? "offline";

  // Synthesize events timeline
  const timelineEvents = useMemo(() => {
    if (!deviceID) return [];
    
    interface TimelineItem {
      id: string;
      time: string;
      type: string;
      title: string;
      message: string;
      tone: "neutral" | "success" | "warning" | "danger" | "info";
    }

    const list: TimelineItem[] = [];

    // Alerts
    (alerts.data ?? [])
      .filter(a => a.device_id === deviceID)
      .forEach(a => {
        list.push({
          id: `alert-${a.id}`,
          time: a.created_at,
          type: "Alert Triggered",
          title: `Alert Triggered: ${a.metric}`,
          message: `${a.message} (value: ${a.observed_value})`,
          tone: "danger"
        });
      });

    // OTA Deployments
    (deployments.data ?? []).forEach(d => {
      if (d.created_at) {
        list.push({
          id: `ota-start-${d.id}`,
          time: d.created_at,
          type: "OTA Started",
          title: `OTA Update Started: ${d.version || "Unknown"}`,
          message: `File: ${d.filename || "n/a"}`,
          tone: "warning"
        });
      }
      if (d.status === "acked" && d.completed_at) {
        list.push({
          id: `ota-complete-${d.id}`,
          time: d.completed_at,
          type: "OTA Completed",
          title: `OTA Update Succeeded`,
          message: `Device running version ${d.version}`,
          tone: "success"
        });
      } else if ((d.status === "nacked" || d.status === "timeout") && (d.failed_at || d.updated_at)) {
        list.push({
          id: `ota-failed-${d.id}`,
          time: d.failed_at || d.updated_at,
          type: "OTA Failed",
          title: `OTA Update Failed: ${d.status}`,
          message: d.failure_reason || d.result_message || "OTA process aborted",
          tone: "danger"
        });
      }
    });

    // Commands
    (commands.data ?? []).forEach(c => {
      if (c.created_at) {
        list.push({
          id: `cmd-sent-${c.id}`,
          time: c.created_at,
          type: "Command Sent",
          title: `Command Sent: ${c.type}`,
          message: c.payload ? JSON.stringify(c.payload) : "No arguments",
          tone: "info"
        });
      }
      if (c.status === "acked" && c.acknowledged_at) {
        list.push({
          id: `cmd-exec-${c.id}`,
          time: c.acknowledged_at,
          type: "Command Executed",
          title: `Command Executed Successfully`,
          message: `Command type: ${c.type} response: ${c.result_message || "Success"}`,
          tone: "success"
        });
      } else if (c.status === "nacked" && c.acknowledged_at) {
        list.push({
          id: `cmd-failed-${c.id}`,
          time: c.acknowledged_at,
          type: "Command Failed",
          title: `Command Rejected`,
          message: `Command: ${c.type} was rejected: ${c.result_message || "Unknown error"}`,
          tone: "danger"
        });
      }
    });

    // Semantic events from AI anomaly/correlation engine
    (semanticEvents.data ?? []).forEach(e => {
      list.push({
        id: `semantic-${e.id}`,
        time: e.created_at,
        type: e.type.replace(/_/g, " ").toUpperCase(),
        title: e.title,
        message: e.summary,
        tone: e.severity === "critical" || e.severity === "danger" ? "danger" : e.severity === "warning" ? "warning" : "info"
      });
    });

    // Device history memories
    (deviceHistory.data ?? []).forEach(h => {
      list.push({
        id: `history-${h.id}`,
        time: h.timestamp || h.created_at,
        type: h.type.replace(/_/g, " ").toUpperCase(),
        title: h.summary,
        message: "Recorded operational log item",
        tone: "neutral"
      });
    });

    return list.sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime());
  }, [alerts.data, deployments.data, commands.data, semanticEvents.data, deviceHistory.data, deviceID]);

  async function submitSimulation(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSimError("");
    setSimSuccess("");
    const formElement = event.currentTarget;
    const form = new FormData(formElement);
    try {
      await api.telemetry.ingest(deviceID, {
        metrics: safeJsonParse(String(form.get("metrics") || "{}"))
      });
      formElement.reset();
      setSimSuccess("Simulation telemetry packet ingested successfully!");
      setTimeout(() => setSimSuccess(""), 4000);
      await devices.refetch();
    } catch (err) {
      setSimError((err as Error).message);
    }
  }

  return (
    <>
      <PageHeader
        eyebrow="Fleet Analytics"
        title="Fleet Observability"
        description="Real-time telemetry aggregation, device vitals, system metrics graphing, and anomaly analysis."
        actions={
          <div className={`live-badge ${realtime.status === "connected" ? "online" : "offline"}`}>
            <span className="live-dot" />
            {realtime.status === "connected" ? "LIVE STREAM ACTIVE" : `WS: ${realtime.status.toUpperCase()}`}
          </div>
        }
      />

      {/* Fleet health summary statistics */}
      <div className="stat-grid five" style={{ marginBottom: 18 }}>
        <div className="stat-card tone-success">
          <span>Online Devices</span>
          <strong>{onlineCount}</strong>
          <small>{totalFleetCount} total nodes registered</small>
        </div>
        <div className="stat-card tone-neutral">
          <span>Offline Devices</span>
          <strong>{offlineCount}</strong>
          <small>{((offlineCount / (totalFleetCount || 1)) * 100).toFixed(0)}% of fleet inactive</small>
        </div>
        <div className="stat-card tone-info">
          <span>Live Session Messages</span>
          <strong>{totalMessageCount}</strong>
          <small>Accumulated live stream</small>
        </div>
        <div className="stat-card tone-warning">
          <span>Active Alerts</span>
          <strong>{activeAlertsCount}</strong>
          <small>Unresolved triggers</small>
        </div>
        <div className="stat-card tone-info">
          <span>Top Firmware Version</span>
          <strong>{firmwareDistribution.split(" ")[0]}</strong>
          <small>{firmwareDistribution.substring(firmwareDistribution.indexOf("(") + 1, firmwareDistribution.length - 1) || "no devices"}</small>
        </div>
      </div>

      {/* Selector sticky bar */}
      <div className="sticky-selector-bar">
        <label>
          <Cpu size={16} className="text-accent" />
          <span>Monitor Device</span>
          <select value={deviceID} onChange={(event) => setDeviceID(event.target.value)}>
            <option value="">Select monitored device</option>
            {devices.data?.map((device) => (
              <option key={device.id} value={device.id}>
                {device.name} · {compactID(device.id)} ({device.status})
              </option>
            ))}
          </select>
        </label>
        <div style={{ display: "flex", gap: 10 }}>
          {deviceID && (
            <button className="button compact secondary" onClick={() => setShowRawPayload(true)}>
              <Code size={14} /> View Raw Payload
            </button>
          )}
          {deviceID && (
            <button
              className={`button compact ${showSimulation ? "secondary" : "primary"}`}
              onClick={() => setShowSimulation(!showSimulation)}
            >
              <Terminal size={14} /> Simulator
            </button>
          )}
        </div>
      </div>

      {!deviceID ? (
        <EmptyState title="Select a device to start monitoring" description="Choose a registered device from the Selector dropdown above to load time-series charts, health analytics, and timeline events." />
      ) : (
        <div className="telemetry-dashboard">
          {/* Main observability view */}
          <div className="telemetry-main-column">
            
            {/* Live metric cards */}
            <h3 style={{ fontSize: 14, fontWeight: 700, margin: "0 0 10px" }}>Latest Vitals</h3>
            <div className="metrics-cards-grid">
              <div className={`metric-card-obs ${currentTemp !== null && currentTemp > 50 ? "danger" : "neutral"}`}>
                <div className="metric-card-obs-header">
                  <span>Temperature</span>
                  <Thermometer size={14} />
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentTemp !== null ? currentTemp.toFixed(1) : "--"}</span>
                  <span className="metric-card-obs-unit">°C</span>
                </div>
                <div style={{ fontSize: 11, color: "var(--muted)" }}>Vitals sensor</div>
              </div>

              <div className="metric-card-obs neutral">
                <div className="metric-card-obs-header">
                  <span>Humidity</span>
                  <Droplets size={14} />
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentHumidity !== null ? currentHumidity.toFixed(1) : "--"}</span>
                  <span className="metric-card-obs-unit">%</span>
                </div>
                <div style={{ fontSize: 11, color: "var(--muted)" }}>Rel. humidity</div>
              </div>

              <div className={`metric-card-obs ${currentRSSI !== null && currentRSSI < -80 ? "warning" : "success"}`}>
                <div className="metric-card-obs-header">
                  <span>RSSI</span>
                  <Wifi size={14} />
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentRSSI !== null ? currentRSSI : "--"}</span>
                  <span className="metric-card-obs-unit">dBm</span>
                </div>
                <div className="metric-card-obs-trend">
                  <RSSISignal rssi={currentRSSI ?? -100} /> Signal Status
                </div>
              </div>

              <div className={`metric-card-obs ${currentBattery !== null && currentBattery < 20 ? "danger" : "neutral"}`}>
                <div className="metric-card-obs-header">
                  <span>Battery</span>
                  {currentBattery !== null ? <BatteryIcon level={currentBattery} /> : <Battery size={14} />}
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentBattery !== null ? currentBattery.toFixed(0) : "--"}</span>
                  <span className="metric-card-obs-unit">%</span>
                </div>
                <div style={{ fontSize: 11, color: "var(--muted)" }}>Cell capacity</div>
              </div>

              <div className={`metric-card-obs ${currentCPU !== null && currentCPU > 85 ? "danger" : "neutral"}`}>
                <div className="metric-card-obs-header">
                  <span>CPU Load</span>
                  <Activity size={14} />
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentCPU !== null ? currentCPU.toFixed(0) : "--"}</span>
                  <span className="metric-card-obs-unit">%</span>
                </div>
                <div style={{ fontSize: 11, color: "var(--muted)" }}>Processor usage</div>
              </div>

              <div className={`metric-card-obs ${currentHeap !== null && currentHeap < 40000 ? "warning" : "neutral"}`}>
                <div className="metric-card-obs-header">
                  <span>Free Heap</span>
                  <Server size={14} />
                </div>
                <div className="metric-card-obs-body">
                  <span className="metric-card-obs-value">{currentHeap !== null ? (currentHeap / 1024).toFixed(0) : "--"}</span>
                  <span className="metric-card-obs-unit">KB</span>
                </div>
                <div style={{ fontSize: 11, color: "var(--muted)" }}>System memory</div>
              </div>
            </div>

            {/* Time-series charts */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", margin: "18px 0 10px" }}>
              <h3 style={{ fontSize: 14, fontWeight: 700, margin: 0 }}>Metrics Visualization</h3>
              <div className="chart-time-filters">
                {(["15m", "1h", "24h", "7d"] as const).map(tf => (
                  <button
                    key={tf}
                    className={timeframe === tf ? "active" : ""}
                    onClick={() => setTimeframe(tf)}
                  >
                    {tf.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            
            <div className="charts-grid-obs">
              <TelemetryChart data={liveTelemetry} metricKey="temp_c" title="Temperature Chart" unit="°C" color="#f0525f" timeframe={timeframe} />
              <TelemetryChart data={liveTelemetry} metricKey="rssi" title="RSSI Wireless Signal Strength" unit="dBm" color="#4f8cff" timeframe={timeframe} />
              <TelemetryChart data={liveTelemetry} metricKey="free_heap" title="Free Heap Memory" unit="bytes" color="#22c878" timeframe={timeframe} />
              <TelemetryChart data={liveTelemetry} metricKey="cpu_load" title="CPU Core Load" unit="%" color="#e3a62f" timeframe={timeframe} />
            </div>

            {/* Event Timeline */}
            <Panel title="Event Timeline" subtitle="Unified operations log from telemetry alarms, OTA deployments, command executions, and presence transitions.">
              {timelineEvents.length === 0 ? (
                <EmptyState title="No timeline events recorded" description="No actions or anomalies have occurred on this device yet." />
              ) : (
                <div className="timeline-list">
                  {timelineEvents.slice(0, 15).map(event => (
                    <div className="timeline-event-card" key={event.id}>
                      <div className="timeline-event-side">
                        <div className={`timeline-event-icon ${event.tone}`}>
                          {event.tone === "danger" && <AlertTriangle size={13} />}
                          {event.tone === "warning" && <Workflow size={13} />}
                          {event.tone === "success" && <Activity size={13} />}
                          {event.tone === "info" && <Send size={13} />}
                          {event.tone === "neutral" && <Clock size={13} />}
                        </div>
                      </div>
                      <div className="timeline-event-content">
                        <div className="timeline-event-meta">
                          <span style={{ fontWeight: 600, textTransform: "uppercase", fontSize: 9, letterSpacing: 0.5 }}>{event.type}</span>
                          <time>{new Date(event.time).toLocaleString()}</time>
                        </div>
                        <span className="timeline-event-title">{event.title}</span>
                        {event.message && <p className="timeline-event-desc">{event.message}</p>}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </Panel>
          </div>

          {/* Right sidebar: device specifics */}
          <div className="telemetry-sidebar-column">
            <Panel title="Device Health Vitals" subtitle={`Vitals details for node`}>
              <div className="detail-card-list">
                <div className="detail-card-row">
                  <strong>Device Name</strong>
                  <span>{selectedDevice?.name || "--"}</span>
                </div>
                <div className="detail-card-row">
                  <strong>ID</strong>
                  <span className="mono" style={{ fontSize: 11 }}><CopyableID id={deviceID} length={8} /></span>
                </div>
                <div className="detail-card-row">
                  <strong>Status</strong>
                  <StatusChip value={currentStatus} />
                </div>
                <div className="detail-card-row">
                  <strong>Firmware Version</strong>
                  <span>{selectedDevice?.firmware_version || "Unknown"}</span>
                </div>
                <div className="detail-card-row">
                  <strong>Uptime</strong>
                  <span>{currentUptime !== null ? formatUptime(currentUptime) : "--"}</span>
                </div>
                <div className="detail-card-row">
                  <strong>MQTT status</strong>
                  <StatusChip value={realtime.status === "connected" ? "connected" : "disconnected"} />
                </div>
                <div className="detail-card-row">
                  <strong>Network RSSI</strong>
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    <span className="mono">{currentRSSI !== null ? `${currentRSSI} dBm` : "--"}</span>
                    {currentRSSI !== null && <RSSISignal rssi={currentRSSI} />}
                  </div>
                </div>
                <div className="detail-card-row" style={{ flexDirection: "column", alignItems: "flex-start", gap: 6 }}>
                  <strong style={{ display: "flex", justifyContent: "space-between", width: "100%" }}>
                    <span>Battery Level</span>
                    <span>{currentBattery !== null ? `${currentBattery.toFixed(0)}%` : "--"}</span>
                  </strong>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, width: "100%" }}>
                    {currentBattery !== null && <BatteryIcon level={currentBattery} />}
                    <div style={{ flex: 1, height: 6, background: "var(--line-soft)", borderRadius: 3, overflow: "hidden" }}>
                      <div
                        style={{
                          height: "100%",
                          width: currentBattery !== null ? `${currentBattery}%` : "0%",
                          background: currentBattery !== null && currentBattery < 20 ? "var(--danger)" : currentBattery < 50 ? "var(--warning)" : "var(--success)"
                        }}
                      />
                    </div>
                  </div>
                </div>
                <div className="detail-card-row" style={{ flexDirection: "column", alignItems: "flex-start", gap: 6 }}>
                  <strong style={{ display: "flex", justifyContent: "space-between", width: "100%" }}>
                    <span>CPU Core Load</span>
                    <span>{currentCPU !== null ? `${currentCPU.toFixed(0)}%` : "--"}</span>
                  </strong>
                  <div style={{ height: 6, background: "var(--line-soft)", borderRadius: 3, overflow: "hidden", width: "100%" }}>
                    <div
                      style={{
                        height: "100%",
                        width: currentCPU !== null ? `${currentCPU}%` : "0%",
                        background: currentCPU !== null && currentCPU > 85 ? "var(--danger)" : currentCPU > 60 ? "var(--warning)" : "var(--success)"
                      }}
                    />
                  </div>
                </div>
                <div className="detail-card-row" style={{ flexDirection: "column", alignItems: "flex-start", gap: 6 }}>
                  <strong style={{ display: "flex", justifyContent: "space-between", width: "100%" }}>
                    <span>Heap Free Memory</span>
                    <span>{currentHeap !== null ? `${(currentHeap / 1024).toFixed(1)} KB` : "--"}</span>
                  </strong>
                  {/* Assume standard ESP32 max usable heap ~280KB for progress display */}
                  <div style={{ height: 6, background: "var(--line-soft)", borderRadius: 3, overflow: "hidden", width: "100%" }}>
                    <div
                      style={{
                        height: "100%",
                        width: currentHeap !== null ? `${Math.min(100, (currentHeap / 280000) * 100)}%` : "0%",
                        background: currentHeap !== null && currentHeap < 40000 ? "var(--danger)" : "var(--accent)"
                      }}
                    />
                  </div>
                </div>
                <div className="detail-card-row">
                  <strong>Largest Free Block</strong>
                  <span>{currentBlock !== null ? `${(currentBlock / 1024).toFixed(1)} KB` : "--"}</span>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      )}

      {/* Simulator form (collapsible panel) */}
      {showSimulation && deviceID && (
        <div style={{ marginTop: 18 }}>
          <Panel title="Developer Tools: Telemetry Payload Simulator" subtitle="Publish simulated JSON telemetry payloads to the device's real-time MQTT feed.">
            <form className="form-grid" onSubmit={submitSimulation}>
              <label className="field full">
                <span>Metrics JSON</span>
                <textarea
                  name="metrics"
                  defaultValue={JSON.stringify(
                    {
                      temp_c: currentTemp ?? 24.5,
                      humidity_pct: currentHumidity ?? 45,
                      rssi: currentRSSI ?? -62,
                      battery_level: currentBattery ?? 85,
                      cpu_load: currentCPU ?? 15,
                      free_heap: currentHeap ?? 242000,
                      largest_free_block: currentBlock ?? 115000,
                      uptime_s: currentUptime ? currentUptime + 30 : 9630,
                      status: "online"
                    },
                    null,
                    2
                  )}
                  rows={10}
                  style={{ fontFamily: "var(--font-mono)", fontSize: "12px", background: "#0d1015" }}
                />
              </label>
              {simError && <div className="form-message error field full">{simError}</div>}
              {simSuccess && <div className="form-message success field full">{simSuccess}</div>}
              <button className="button primary" type="submit">
                <Send size={15} /> Publish Ingestion Event
              </button>
            </form>
          </Panel>
        </div>
      )}

      {/* Raw Payload sliding drawer modal */}
      {showRawPayload && (
        <div className="payload-drawer-overlay" onClick={() => setShowRawPayload(false)}>
          <div className="payload-drawer" onClick={(event) => event.stopPropagation()}>
            <div className="payload-drawer-header">
              <h3>Latest Raw Payload</h3>
              <button className="button compact secondary" onClick={() => setShowRawPayload(false)}>
                <X size={16} />
              </button>
            </div>
            <div className="payload-drawer-body">
              <pre className="code-block" style={{ margin: 0, fontSize: "11px", whiteSpace: "pre-wrap", overflowX: "auto" }}>
                {latestTelemetry ? stringifyJson(latestTelemetry) : "{}"}
              </pre>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
