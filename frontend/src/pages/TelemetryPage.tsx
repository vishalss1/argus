import { useState, useMemo, useEffect } from "react";
import { 
  useWorkspaceDevices, 
  useLatestTelemetry 
} from "../hooks/useArgusData";
import { useRealtime } from "../hooks/useRealtime";
import { useWorkspaceContext } from "../context/WorkspaceContext";
import { PageHeader, Panel } from "../components/ui";
import { 
  Activity, 
  Briefcase, 
  Cpu, 
  Radio, 
  RefreshCw, 
  Thermometer, 
  HardDrive, 
  Battery, 
  ShieldAlert,
  Signal
} from "lucide-react";

// Helper to format uptime cleanly
function formatUptime(seconds: number): string {
  if (seconds < 60) {
    return `${Math.floor(seconds)}s`;
  }
  const minutes = seconds / 60;
  if (minutes < 60) {
    return `${Math.floor(minutes)}m ${Math.floor(seconds % 60)}s`;
  }
  const hours = minutes / 60;
  return `${Math.floor(hours)}h ${Math.floor(minutes % 60)}m ${Math.floor(seconds % 60)}s`;
}

// Check if value is numeric
function isNumeric(val: any): boolean {
  return typeof val === "number" && !isNaN(val);
}

export function TelemetryPage() {
  const { selectedWorkspaceId } = useWorkspaceContext();

  // Reset device selection when workspace changes
  useEffect(() => {
    setSelectedDeviceId("");
  }, [selectedWorkspaceId]);

  // Get devices for selected workspace
  const { data: devices, isLoading: devicesLoading } = useWorkspaceDevices(selectedWorkspaceId);

  // Persistence for Device ID
  const [selectedDeviceId, setSelectedDeviceId] = useState<string>(() => {
    return localStorage.getItem("argus_telemetry_device_id") || "";
  });

  // Set default device for workspace
  useEffect(() => {
    if (!selectedDeviceId && devices && devices.length > 0) {
      const firstDevId = devices[0].id;
      setSelectedDeviceId(firstDevId);
      localStorage.setItem("argus_telemetry_device_id", firstDevId);
    }
  }, [devices, selectedDeviceId]);

  const handleDeviceChange = (id: string) => {
    setSelectedDeviceId(id);
    localStorage.setItem("argus_telemetry_device_id", id);
  };

  // WebSocket / Realtime connection status and telemetry stream
  const { status: wsStatus, telemetryByDevice } = useRealtime();

  // Initial / latest telemetry fallback
  const latestTelemetryQuery = useLatestTelemetry(selectedDeviceId || undefined);

  // Combine real-time WebSocket telemetry list and initial fallback
  const telemetryHistory = useMemo(() => {
    if (!selectedDeviceId) return [];
    
    // Check WebSocket history (newest first, up to 50 items)
    const wsHistory = telemetryByDevice[selectedDeviceId] || [];
    
    if (wsHistory.length > 0) {
      return wsHistory;
    }

    // Fallback to latest telemetry query
    if (latestTelemetryQuery.data) {
      return [latestTelemetryQuery.data];
    }

    return [];
  }, [selectedDeviceId, telemetryByDevice, latestTelemetryQuery.data]);

  // Extract latest metrics payload
  const currentMetrics = useMemo(() => {
    if (telemetryHistory.length === 0) return null;
    const rawMetrics = telemetryHistory[0].metrics;
    if (rawMetrics && typeof rawMetrics === "object" && !Array.isArray(rawMetrics)) {
      return rawMetrics as Record<string, any>;
    }
    return null;
  }, [telemetryHistory]);

  // Track selected metric for charting
  const [selectedMetric, setSelectedMetric] = useState<string>("cpu");

  // Dynamically extract all numeric metrics keys for charts/cards selection
  const numericMetricKeys = useMemo(() => {
    if (!currentMetrics) return ["cpu", "memory", "temperature"];
    return Object.keys(currentMetrics).filter(key => isNumeric(currentMetrics[key]));
  }, [currentMetrics]);

  // Automatically select an available numeric metric if the current selection is invalid
  useEffect(() => {
    if (numericMetricKeys.length > 0 && !numericMetricKeys.includes(selectedMetric)) {
      setSelectedMetric(numericMetricKeys[0]);
    }
  }, [numericMetricKeys, selectedMetric]);

  // Active Device information helper
  const activeDevice = useMemo(() => {
    if (!devices || !selectedDeviceId) return null;
    return devices.find(d => d.id === selectedDeviceId) || null;
  }, [devices, selectedDeviceId]);

  // Chart series (sorted chronologically - oldest to newest)
  const chartData = useMemo(() => {
    if (telemetryHistory.length === 0) return [];
    
    // Slice last 30 data points and reverse to have oldest first
    const points = telemetryHistory
      .slice(0, 30)
      .map(item => {
        const val = item.metrics && typeof item.metrics === "object" && !Array.isArray(item.metrics)
          ? (item.metrics as Record<string, any>)[selectedMetric]
          : null;
        
        return {
          time: new Date(item.recorded_at || item.created_at).toLocaleTimeString([], { 
            hour: "2-digit", 
            minute: "2-digit", 
            second: "2-digit" 
          }),
          timestamp: new Date(item.recorded_at || item.created_at).getTime(),
          value: isNumeric(val) ? Number(val) : 0
        };
      })
      .filter(p => isNumeric(p.value));

    // Sort chronologically by timestamp
    return points.sort((a, b) => a.timestamp - b.timestamp);
  }, [telemetryHistory, selectedMetric]);

  // Auto-refresh control state (enabled by default)
  const [autoRefresh, setAutoRefresh] = useState(true);

  // SVG Chart sizing & calculations
  const chartWidth = 600;
  const chartHeight = 220;
  const paddingX = 40;
  const paddingY = 30;

  const svgPaths = useMemo(() => {
    if (chartData.length < 2) return { line: "", area: "", yMin: 0, yMax: 100 };

    const minVal = Math.min(...chartData.map(d => d.value));
    const maxVal = Math.max(...chartData.map(d => d.value));
    const valRange = maxVal - minVal === 0 ? 100 : maxVal - minVal;
    
    // Add 10% breathing room to min/max on chart layout
    const yMin = Math.max(0, minVal - valRange * 0.1);
    const yMax = maxVal + valRange * 0.1;
    const yRange = yMax - yMin;

    const points = chartData.map((d, index) => {
      const x = paddingX + (index / (chartData.length - 1)) * (chartWidth - paddingX * 2);
      const y = chartHeight - paddingY - ((d.value - yMin) / yRange) * (chartHeight - paddingY * 2);
      return { x, y };
    });

    const linePath = points.reduce((acc, p, idx) => {
      return acc + (idx === 0 ? `M ${p.x} ${p.y}` : ` L ${p.x} ${p.y}`);
    }, "");

    const areaPath = linePath + 
      ` L ${points[points.length - 1].x} ${chartHeight - paddingY}` + 
      ` L ${points[0].x} ${chartHeight - paddingY} Z`;

    return { line: linePath, area: areaPath, yMin, yMax };
  }, [chartData]);

  // Card icons helper
  const getMetricIcon = (name: string) => {
    const key = name.toLowerCase();
    if (key.includes("cpu")) return <Cpu size={16} />;
    if (key.includes("temp")) return <Thermometer size={16} />;
    if (key.includes("mem") || key.includes("ram") || key.includes("storage")) return <HardDrive size={16} />;
    if (key.includes("bat") || key.includes("power")) return <Battery size={16} />;
    return <Activity size={16} />;
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", gap: "20px" }}>
      <PageHeader 
        title="Live Telemetry" 
        description="Monitor system performance, device health and telemetry streams in real-time."
      />

      <div style={{ display: "flex", flexWrap: "wrap", gap: "20px", width: "100%" }}>
        
        {/* Left Sidebar: Device Selector & Health Vitals */}
        <div style={{ flex: "0 0 320px", display: "flex", flexDirection: "column", gap: "20px", minWidth: "300px" }}>
          
          {/* Workspace & Device selection Panel */}
          <Panel title="Select Device">
            <div style={{ display: "flex", flexDirection: "column", gap: "16px" }}>

              {/* Devices list scroll area */}
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                <label style={{ display: "block", fontSize: "11px", fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", marginBottom: "2px" }}>
                  Devices ({devices?.length ?? 0})
                </label>
                
                {devicesLoading ? (
                  <p className="muted" style={{ fontSize: "13px" }}>Loading devices...</p>
                ) : !devices || devices.length === 0 ? (
                  <p className="muted" style={{ fontSize: "13px", padding: "12px 0" }}>No devices found in this workspace.</p>
                ) : (
                  <div style={{ maxHeight: "240px", overflowY: "auto", display: "flex", flexDirection: "column", gap: "6px", paddingRight: "4px" }}>
                    {devices.map(d => {
                      const isSelected = d.id === selectedDeviceId;
                      const isOnline = d.status?.toLowerCase() === "online";
                      return (
                        <button
                          key={d.id}
                          onClick={() => handleDeviceChange(d.id)}
                          style={{
                            width: "100%",
                            textAlign: "left",
                            padding: "10px 12px",
                            background: isSelected ? "var(--surface-2)" : "rgba(255,255,255,0.01)",
                            border: isSelected ? "1px solid var(--accent)" : "1px solid var(--line)",
                            borderRadius: "6px",
                            color: "inherit",
                            cursor: "pointer",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "space-between",
                            transition: "all 0.2s"
                          }}
                        >
                          <div>
                            <div style={{ fontWeight: isSelected ? 600 : 400, fontSize: "14px" }}>{d.name}</div>
                            <div className="muted" style={{ fontSize: "11px", marginTop: "2px" }}>{d.type}</div>
                          </div>
                          <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                            <span style={{
                              width: "8px",
                              height: "8px",
                              borderRadius: "50%",
                              backgroundColor: isOnline ? "var(--success)" : "var(--faint)"
                            }} />
                            <span style={{ fontSize: "11px", color: "var(--faint)" }}>{d.status}</span>
                          </div>
                        </button>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>
          </Panel>

          {/* Vitals Panel */}
          <Panel title="Health Vitals">
            <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
              {!selectedDeviceId ? (
                <p className="muted" style={{ fontSize: "13px" }}>Please select a device.</p>
              ) : latestTelemetryQuery.isLoading && telemetryHistory.length === 0 ? (
                <p className="muted" style={{ fontSize: "13px" }}>Loading health metrics...</p>
              ) : !currentMetrics ? (
                <div style={{ padding: "16px 0", textAlign: "center" }}>
                  <ShieldAlert size={28} style={{ color: "var(--warning)", marginBottom: "8px" }} />
                  <p className="muted" style={{ fontSize: "13px", margin: 0 }}>No active telemetry data.</p>
                </div>
              ) : (
                <div style={{ display: "flex", flexDirection: "column", gap: "8px", maxHeight: "320px", overflowY: "auto" }}>
                  {Object.keys(currentMetrics).map(key => {
                    const val = currentMetrics[key];
                    let displayVal = String(val);
                    
                    // Specific Uptime formatting
                    if (key.toLowerCase() === "uptime" && isNumeric(val)) {
                      displayVal = formatUptime(Number(val));
                    } else if (isNumeric(val)) {
                      displayVal = Number(val).toFixed(1);
                    }

                    return (
                      <div 
                        key={key} 
                        style={{ 
                          display: "flex", 
                          justifyContent: "space-between", 
                          alignItems: "center", 
                          padding: "8px 10px", 
                          background: "rgba(255,255,255,0.02)", 
                          borderRadius: "4px",
                          border: "1px solid rgba(255,255,255,0.02)"
                        }}
                      >
                        <span className="mono" style={{ fontSize: "12px", color: "var(--faint)", textTransform: "capitalize" }}>
                          {key.replace(/_/g, " ")}
                        </span>
                        <strong className="mono" style={{ fontSize: "13px" }}>{displayVal}</strong>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </Panel>
        </div>

        {/* Main Content Area */}
        <div style={{ flex: "1", minWidth: "500px", display: "flex", flexDirection: "column", gap: "20px" }}>
          
          {/* Connection Control Bar */}
          <div style={{
            background: "rgba(255,255,255,0.03)",
            border: "1px solid var(--line)",
            borderRadius: "10px",
            padding: "16px 20px",
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "space-between",
            alignItems: "center",
            gap: "16px",
            backdropFilter: "blur(10px)"
          }}>
            <div>
              {activeDevice ? (
                <>
                  <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                    <h3 style={{ margin: 0, fontSize: "18px", fontWeight: 600 }}>{activeDevice.name}</h3>
                    <span style={{
                      padding: "2px 8px",
                      background: "rgba(255,255,255,0.05)",
                      border: "1px solid var(--line)",
                      borderRadius: "12px",
                      fontSize: "11px",
                      color: "var(--faint)"
                    }}>
                      {activeDevice.type}
                    </span>
                  </div>
                  <div className="muted mono" style={{ fontSize: "11px", marginTop: "4px" }}>UUID: {activeDevice.id}</div>
                </>
              ) : (
                <h3 style={{ margin: 0, fontSize: "18px", color: "var(--faint)" }}>No device selected</h3>
              )}
            </div>

            <div style={{ display: "flex", alignItems: "center", gap: "16px" }}>
              {/* WS Status Indicator */}
              <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <span style={{
                  width: "10px",
                  height: "10px",
                  borderRadius: "50%",
                  backgroundColor: 
                    wsStatus === "connected" ? "var(--success)" :
                    wsStatus === "connecting" ? "var(--warning)" : "var(--danger)",
                  boxShadow: wsStatus === "connected" ? "0 0 8px var(--success)" : "none",
                  transition: "all 0.3s"
                }} />
                <span className="mono" style={{ fontSize: "12px", textTransform: "capitalize" }}>
                  {wsStatus === "connected" ? "Streaming" : wsStatus}
                </span>
              </div>

              {/* Auto-Refresh Toggle */}
              <button 
                onClick={() => setAutoRefresh(!autoRefresh)}
                style={{
                  background: "transparent",
                  border: "none",
                  cursor: "pointer",
                  color: autoRefresh ? "var(--accent)" : "var(--faint)",
                  display: "flex",
                  alignItems: "center",
                  gap: "6px",
                  padding: "6px 10px",
                  borderRadius: "4px",
                  transition: "all 0.2s"
                }}
              >
                <Radio size={16} className={autoRefresh && wsStatus === "connected" ? "pulse" : ""} />
                <span style={{ fontSize: "12px", fontWeight: 500 }}>Live Feed</span>
              </button>
            </div>
          </div>

          {/* Metric Summary Cards (Max 4 numeric cards) */}
          <div style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))",
            gap: "16px"
          }}>
            {numericMetricKeys.slice(0, 4).map(key => {
              const val = currentMetrics ? currentMetrics[key] : null;
              let suffix = "";
              if (key.toLowerCase().includes("cpu") || key.toLowerCase().includes("mem") || key.toLowerCase().includes("ram") || key.toLowerCase().includes("util")) {
                suffix = "%";
              } else if (key.toLowerCase().includes("temp")) {
                suffix = "°C";
              } else if (key.toLowerCase().includes("volt")) {
                suffix = "V";
              }

              // Simple trend indication based on last two points
              let trend: "up" | "down" | "flat" = "flat";
              if (telemetryHistory.length >= 2) {
                const prevMetrics = telemetryHistory[1].metrics;
                if (prevMetrics && typeof prevMetrics === "object" && !Array.isArray(prevMetrics)) {
                  const prevVal = (prevMetrics as Record<string, any>)[key];
                  if (isNumeric(val) && isNumeric(prevVal)) {
                    if (Number(val) > Number(prevVal)) trend = "up";
                    else if (Number(val) < Number(prevVal)) trend = "down";
                  }
                }
              }

              return (
                <div key={key} style={{
                  background: "rgba(255,255,255,0.03)",
                  border: "1px solid var(--line)",
                  borderRadius: "10px",
                  padding: "16px",
                  display: "flex",
                  flexDirection: "column",
                  gap: "10px",
                  backdropFilter: "blur(10px)"
                }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", color: "var(--faint)" }}>
                    <span style={{ fontSize: "12px", fontWeight: 500, textTransform: "uppercase", letterSpacing: "0.5px" }}>
                      {key.replace(/_/g, " ")}
                    </span>
                    {getMetricIcon(key)}
                  </div>
                  <div style={{ display: "flex", alignItems: "baseline", gap: "6px" }}>
                    <span style={{ fontSize: "24px", fontWeight: 700, fontFamily: "monospace" }}>
                      {isNumeric(val) ? Number(val).toFixed(1) : "—"}
                    </span>
                    <span style={{ fontSize: "14px", color: "var(--faint)" }}>{suffix}</span>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: "4px", fontSize: "11px" }}>
                    {trend === "up" ? (
                      <span style={{ color: "var(--warning)" }}>▲ increasing</span>
                    ) : trend === "down" ? (
                      <span style={{ color: "var(--success)" }}>▼ decreasing</span>
                    ) : (
                      <span style={{ color: "var(--faint)" }}>■ stable</span>
                    )}
                  </div>
                </div>
              );
            })}

            {/* Placeholder card if no metrics */}
            {numericMetricKeys.length === 0 && (
              <div style={{
                gridColumn: "1 / -1",
                background: "rgba(255,255,255,0.03)",
                border: "1px dashed var(--line)",
                borderRadius: "10px",
                padding: "24px",
                textAlign: "center",
                color: "var(--faint)",
                fontSize: "14px"
              }}>
                Awaiting telemetry payload from active device stream...
              </div>
            )}
          </div>

          {/* SVG Real-time Time Series Chart */}
          <Panel 
            title="Real-time Stream" 
            actions={
              <div style={{ display: "flex", gap: "6px" }}>
                {numericMetricKeys.map(key => (
                  <button
                    key={key}
                    onClick={() => setSelectedMetric(key)}
                    style={{
                      background: selectedMetric === key ? "var(--accent)" : "rgba(255,255,255,0.04)",
                      border: selectedMetric === key ? "1px solid var(--accent)" : "1px solid var(--line)",
                      borderRadius: "4px",
                      color: selectedMetric === key ? "#fff" : "var(--faint)",
                      padding: "4px 8px",
                      fontSize: "12px",
                      textTransform: "capitalize",
                      cursor: "pointer"
                    }}
                  >
                    {key}
                  </button>
                ))}
              </div>
            }
          >
            <div style={{ display: "flex", flexDirection: "column", gap: "16px", padding: "10px 0" }}>
              {chartData.length < 2 ? (
                <div style={{ height: `${chartHeight}px`, display: "flex", alignItems: "center", justifyContent: "center", border: "1px dashed var(--line)", borderRadius: "8px", background: "rgba(255,255,255,0.01)" }}>
                  <p className="muted" style={{ fontSize: "14px" }}>
                    {selectedDeviceId ? "Awaiting telemetry data to populate the graph..." : "Select a device to view live charts"}
                  </p>
                </div>
              ) : (
                <div style={{ width: "100%", overflowX: "auto" }}>
                  <svg 
                    viewBox={`0 0 ${chartWidth} ${chartHeight}`} 
                    style={{ width: "100%", height: "auto", display: "block", overflow: "visible" }}
                  >
                    {/* Definitions for gradient fills */}
                    <defs>
                      <linearGradient id="chart-gradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="0%" stopColor="var(--accent)" stopOpacity="0.25" />
                        <stop offset="100%" stopColor="var(--accent)" stopOpacity="0.0" />
                      </linearGradient>
                    </defs>

                    {/* Grid Lines */}
                    {[0, 0.25, 0.5, 0.75, 1].map((ratio, i) => {
                      const y = paddingY + ratio * (chartHeight - paddingY * 2);
                      const yVal = svgPaths.yMax - ratio * (svgPaths.yMax - svgPaths.yMin);
                      return (
                        <g key={i}>
                          <line 
                            x1={paddingX} 
                            y1={y} 
                            x2={chartWidth - paddingX} 
                            y2={y} 
                            stroke="var(--line)" 
                            strokeWidth="1" 
                            strokeDasharray="4 4" 
                          />
                          <text 
                            x={paddingX - 8} 
                            y={y + 4} 
                            fill="var(--faint)" 
                            fontSize="10" 
                            fontFamily="monospace"
                            textAnchor="end"
                          >
                            {yVal.toFixed(0)}
                          </text>
                        </g>
                      );
                    })}

                    {/* Area under the line */}
                    <path 
                      d={svgPaths.area} 
                      fill="url(#chart-gradient)" 
                    />

                    {/* Line path */}
                    <path 
                      d={svgPaths.line} 
                      fill="none" 
                      stroke="var(--accent)" 
                      strokeWidth="2" 
                    />

                    {/* X axis labels */}
                    {chartData.map((d, index) => {
                      // Label only a subset of points to avoid cluttering
                      const labelInterval = Math.max(1, Math.floor(chartData.length / 5));
                      if (index % labelInterval !== 0 && index !== chartData.length - 1) return null;
                      
                      const x = paddingX + (index / (chartData.length - 1)) * (chartWidth - paddingX * 2);
                      return (
                        <text 
                          key={index}
                          x={x} 
                          y={chartHeight - paddingY + 16} 
                          fill="var(--faint)" 
                          fontSize="9" 
                          fontFamily="monospace"
                          textAnchor="middle"
                        >
                          {d.time}
                        </text>
                      );
                    })}
                  </svg>
                </div>
              )}
            </div>
          </Panel>
        </div>
      </div>
    </div>
  );
}
