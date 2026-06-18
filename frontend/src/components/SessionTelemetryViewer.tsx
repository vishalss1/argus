import { useMemo, useState, useRef, useEffect, useCallback } from "react";
import { Search, ChevronDown, ChevronUp, ChevronLeft, ChevronRight, Activity, Thermometer, Battery, Wifi, Cpu, Clock, AlertCircle } from "lucide-react";
import { TelemetryChart } from "./TelemetryChart";
import { api } from "../services/api";
import type { TelemetryRow } from "../types/api";

interface SessionTelemetryViewerProps {
  sessionID: string;
  artifactExportsExpired?: boolean;
  artifactTelemetryExportPaths?: { json: string; csv: string } | null;
  isActive: boolean;
}

type SortOrder = "newest" | "oldest";

const PAGE_SIZES = [50, 100, 250, 500] as const;

const METRIC_CHARTS = [
  { metric: "temp", label: "Temperature", unit: "°C", color: "#ff6644", icon: Thermometer },
  { metric: "battery", label: "Battery", unit: "%", color: "#44ff66", icon: Battery },
  { metric: "cpu", label: "CPU", unit: "%", color: "#4488ff", icon: Cpu },
  { metric: "rssi", label: "RSSI", unit: "dBm", color: "#ffcc44", icon: Wifi },
];

export function SessionTelemetryViewer({
  sessionID,
  artifactExportsExpired,
  artifactTelemetryExportPaths,
  isActive,
}: SessionTelemetryViewerProps) {
  const [rawData, setRawData] = useState<TelemetryRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [pageIndex, setPageIndex] = useState(0);
  const [pageSize, setPageSize] = useState<number>(100);
  const [sortOrder, setSortOrder] = useState<SortOrder>("newest");
  const [deviceFilter, setDeviceFilter] = useState<string>("all");
  const [metricSearch, setMetricSearch] = useState("");
  const [timeStart, setTimeStart] = useState("");
  const [timeEnd, setTimeEnd] = useState("");
  const [selectedDevices, setSelectedDevices] = useState<string[]>([]);
  const fetchedRef = useRef(false);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const raw = await api.sessions.getTelemetryJSON(sessionID);
      const json = JSON.parse(raw);

      let rows: TelemetryRow[];
      if (Array.isArray(json)) {
        rows = json;
      } else if (json && typeof json === "object") {
        rows = [];
        for (const [devID, samples] of Object.entries(json)) {
          const sArr = samples as any;
          const sItems = sArr.samples || sArr;
          if (Array.isArray(sItems)) {
            for (const s of sItems) {
              const ts = s.timestamp?.seconds
                ? new Date(Number(s.timestamp.seconds) * 1000).toISOString()
                : s.timestamp || "";
              rows.push({ timestamp: ts, device_id: devID, metrics: s.metrics || {} });
            }
          }
        }
      } else {
        rows = [];
      }
      setRawData(rows);
    } catch (err: any) {
      if (err.status === 410) {
        setError("telemetry export expired");
      } else {
        setError(err.message || "failed to load telemetry");
      }
      setRawData([]);
    } finally {
      setLoading(false);
    }
  }, [sessionID]);

  useEffect(() => {
    if (artifactExportsExpired) {
      setLoading(false);
      setError("telemetry export expired");
      return;
    }
    if (!fetchedRef.current) {
      fetchedRef.current = true;
      loadData();
    }
  }, [artifactExportsExpired, loadData]);

  const allDevices = useMemo(() => {
    const set = new Set<string>();
    for (const r of rawData) set.add(r.device_id);
    return Array.from(set).sort();
  }, [rawData]);

  const allMetrics = useMemo(() => {
    const set = new Set<string>();
    for (const r of rawData) {
      for (const k of Object.keys(r.metrics)) set.add(k);
    }
    return Array.from(set).sort();
  }, [rawData]);

  useEffect(() => {
    if (selectedDevices.length === 0 && allDevices.length > 0) {
      setSelectedDevices(allDevices.slice(0, 5));
    }
  }, [allDevices, selectedDevices.length]);

  const filteredData = useMemo(() => {
    let data = rawData;

    if (deviceFilter !== "all") {
      data = data.filter((r) => r.device_id === deviceFilter);
    }

    if (metricSearch) {
      const q = metricSearch.toLowerCase();
      data = data.filter((r) =>
        Object.keys(r.metrics).some((k) => k.toLowerCase().includes(q))
      );
    }

    if (timeStart) {
      data = data.filter((r) => r.timestamp >= timeStart);
    }
    if (timeEnd) {
      data = data.filter((r) => r.timestamp <= timeEnd);
    }

    data = [...data].sort((a, b) => {
      const cmp = a.timestamp.localeCompare(b.timestamp);
      return sortOrder === "newest" ? -cmp : cmp;
    });

    return data;
  }, [rawData, deviceFilter, metricSearch, timeStart, timeEnd, sortOrder]);

  const totalRows = filteredData.length;
  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const safePage = Math.min(pageIndex, totalPages - 1);
  const pageData = filteredData.slice(safePage * pageSize, (safePage + 1) * pageSize);

  const columns = useMemo(() => {
    const base = ["timestamp", "device_id"];
    const extras = allMetrics.filter((m) => !base.includes(m));
    return [...base, ...extras];
  }, [allMetrics]);

  const metricNames = useMemo(
    () => allMetrics.filter((m) => m !== "device_id"),
    [allMetrics]
  );

  if (artifactExportsExpired) {
    return (
      <div className="report-empty-msg" style={{ padding: 48, textAlign: "center" }}>
        <AlertCircle size={32} strokeWidth={1.5} style={{ marginBottom: 12, opacity: 0.5 }} />
        <p>Telemetry exports have expired after 7 days.</p>
        <p className="muted">Statistical summaries remain permanently available.</p>
      </div>
    );
  }

  if (loading) {
    return <div className="report-empty-msg" style={{ padding: 48, textAlign: "center" }}>Loading telemetry data...</div>;
  }

  if (error) {
    return (
      <div className="report-empty-msg" style={{ padding: 48, textAlign: "center" }}>
        <p style={{ color: "var(--danger)" }}>{error}</p>
        <button className="button secondary compact" onClick={loadData} style={{ marginTop: 12 }}>
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="telemetry-viewer">
      {/* Charts */}
      <div className="telemetry-charts">
        {METRIC_CHARTS.map((mc) => (
          <TelemetryChart
            key={mc.metric}
            data={rawData}
            metric={mc.metric}
            deviceIds={selectedDevices.length > 0 ? selectedDevices : allDevices.slice(0, 5)}
            unit={mc.unit}
            color={mc.color}
          />
        ))}
      </div>

      {/* Metrics Summary */}
      <div className="telemetry-metric-summary">
        {metricNames.map((m) => {
          const vals = rawData
            .map((r) => r.metrics[m])
            .filter((v): v is number => typeof v === "number");
          if (vals.length === 0) return null;
          const avg = vals.reduce((s, v) => s + v, 0) / vals.length;
          const min = Math.min(...vals);
          const max = Math.max(...vals);
          const chartMeta = METRIC_CHARTS.find((c) => c.metric === m);
          return (
            <div key={m} className="telemetry-metric-chip">
              {chartMeta && <chartMeta.icon size={14} strokeWidth={1.5} />}
              <span className="mono">{m}</span>
              <span className="muted" style={{ fontSize: 11 }}>
                avg {avg.toFixed(1)} min {min.toFixed(1)} max {max.toFixed(1)}
              </span>
            </div>
          );
        })}
      </div>

      {/* Filters */}
      <div className="telemetry-filters">
        <div className="telemetry-filter-group">
          <label className="telemetry-filter-label">Device</label>
          <select
            className="telemetry-select"
            value={deviceFilter}
            onChange={(e) => { setDeviceFilter(e.target.value); setPageIndex(0); }}
          >
            <option value="all">All Devices</option>
            {allDevices.map((d) => (
              <option key={d} value={d}>{d}</option>
            ))}
          </select>
        </div>

        <div className="telemetry-filter-group">
          <label className="telemetry-filter-label">Metric</label>
          <div className="telemetry-search-wrap">
            <Search size={13} strokeWidth={1.5} />
            <input
              className="telemetry-search-input"
              placeholder="temperature, cpu..."
              value={metricSearch}
              onChange={(e) => { setMetricSearch(e.target.value); setPageIndex(0); }}
            />
          </div>
        </div>

        <div className="telemetry-filter-group">
          <label className="telemetry-filter-label">Start</label>
          <input
            type="datetime-local"
            className="telemetry-input"
            value={timeStart}
            onChange={(e) => { setTimeStart(e.target.value); setPageIndex(0); }}
          />
        </div>

        <div className="telemetry-filter-group">
          <label className="telemetry-filter-label">End</label>
          <input
            type="datetime-local"
            className="telemetry-input"
            value={timeEnd}
            onChange={(e) => { setTimeEnd(e.target.value); setPageIndex(0); }}
          />
        </div>

        <div className="telemetry-filter-group">
          <label className="telemetry-filter-label">Sort</label>
          <button
            className="telemetry-sort-btn"
            onClick={() => setSortOrder(sortOrder === "newest" ? "oldest" : "newest")}
          >
            {sortOrder === "newest" ? "Newest First" : "Oldest First"}
            {sortOrder === "newest" ? <ChevronDown size={13} /> : <ChevronUp size={13} />}
          </button>
        </div>
      </div>

      {/* Chart device selector */}
      <div className="telemetry-device-chips">
        <span className="muted" style={{ fontSize: 11 }}>Chart devices:</span>
        {allDevices.slice(0, 10).map((dev) => (
          <button
            key={dev}
            className={`telemetry-device-chip ${selectedDevices.includes(dev) ? "active" : ""}`}
            onClick={() => {
              setSelectedDevices((prev) =>
                prev.includes(dev) ? prev.filter((d) => d !== dev) : [...prev, dev]
              );
            }}
          >
            {dev}
          </button>
        ))}
      </div>

      {/* Table */}
      <div className="telemetry-table-wrap">
        <table className="telemetry-table">
          <thead>
            <tr>
              {columns.map((col) => (
                <th key={col} className="telemetry-th">
                  {col === "timestamp" ? (
                    <span><Clock size={11} strokeWidth={1.5} /> Timestamp</span>
                  ) : col === "device_id" ? (
                    "Device"
                  ) : (
                    col
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pageData.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="telemetry-empty-cell">
                  No telemetry data matches filters.
                </td>
              </tr>
            ) : (
              pageData.map((row, i) => (
                <tr key={i} className="telemetry-row">
                  {columns.map((col) => (
                    <td key={col} className="telemetry-td">
                      {col === "timestamp"
                        ? new Date(row.timestamp).toLocaleString()
                        : col === "device_id"
                        ? row.device_id
                        : row.metrics[col] !== undefined
                        ? Number(row.metrics[col]).toFixed(2)
                        : "-"}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      <div className="telemetry-pagination">
        <div className="telemetry-page-info">
          <span className="muted" style={{ fontSize: 12 }}>
            {totalRows.toLocaleString()} rows
          </span>
          <select
            className="telemetry-page-size"
            value={pageSize}
            onChange={(e) => { setPageSize(Number(e.target.value)); setPageIndex(0); }}
          >
            {PAGE_SIZES.map((s) => (
              <option key={s} value={s}>{s} / page</option>
            ))}
          </select>
        </div>
        <div className="telemetry-page-nav">
          <button
            className="telemetry-nav-btn"
            disabled={safePage === 0}
            onClick={() => setPageIndex(0)}
          >
            <ChevronLeft size={13} strokeWidth={1.5} />
            <ChevronLeft size={13} strokeWidth={1.5} style={{ marginLeft: -6 }} />
          </button>
          <button
            className="telemetry-nav-btn"
            disabled={safePage === 0}
            onClick={() => setPageIndex(safePage - 1)}
          >
            <ChevronLeft size={13} strokeWidth={1.5} />
          </button>
          <span className="telemetry-page-num">
            {safePage + 1} / {totalPages.toLocaleString()}
          </span>
          <button
            className="telemetry-nav-btn"
            disabled={safePage >= totalPages - 1}
            onClick={() => setPageIndex(safePage + 1)}
          >
            <ChevronRight size={13} strokeWidth={1.5} />
          </button>
          <button
            className="telemetry-nav-btn"
            disabled={safePage >= totalPages - 1}
            onClick={() => setPageIndex(totalPages - 1)}
          >
            <ChevronRight size={13} strokeWidth={1.5} />
            <ChevronRight size={13} strokeWidth={1.5} style={{ marginLeft: -6 }} />
          </button>
        </div>
      </div>
    </div>
  );
}
