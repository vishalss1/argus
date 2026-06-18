import { useMemo } from "react";
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend,
} from "recharts";
import type { TelemetryRow } from "../types/api";

interface TelemetryChartProps {
  data: TelemetryRow[];
  metric: string;
  deviceIds: string[];
  unit: string;
  color?: string;
}

export function TelemetryChart({ data, metric, deviceIds, unit, color = "#fff" }: TelemetryChartProps) {
  const chartData = useMemo(() => {
    if (data.length === 0) return [];

    const bucketed = new Map<string, Map<string, number[]>>();

    for (const row of data) {
      if (!deviceIds.includes(row.device_id)) continue;
      const val = row.metrics[metric];
      if (val === undefined) continue;

      const bucket = row.timestamp.substring(0, 16);
      if (!bucketed.has(bucket)) bucketed.set(bucket, new Map());
      const devMap = bucketed.get(bucket)!;
      if (!devMap.has(row.device_id)) devMap.set(row.device_id, []);
      devMap.get(row.device_id)!.push(val);
    }

    return Array.from(bucketed.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([bucket, devMap]) => {
        const point: Record<string, any> = { time: bucket.substring(11) };
        for (const [dev, vals] of devMap) {
          const avg = vals.reduce((s, v) => s + v, 0) / vals.length;
          point[dev] = Number(avg.toFixed(2));
        }
        return point;
      });
  }, [data, metric, deviceIds]);

  if (chartData.length === 0) {
    return <div className="muted" style={{ padding: 24, textAlign: "center" }}>No {metric} data available</div>;
  }

  const colors = ["#fff", "#888", "#4af", "#f84", "#8f4", "#f48", "#48f", "#ff4"];

  return (
    <div className="chart-container">
      <h4 className="chart-title">{metric.charAt(0).toUpperCase() + metric.slice(1)} ({unit})</h4>
      <ResponsiveContainer width="100%" height={200}>
        <LineChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" stroke="#222" />
          <XAxis dataKey="time" tick={{ fontSize: 10, fill: "#888" }} />
          <YAxis tick={{ fontSize: 10, fill: "#888" }} unit={unit} />
          <Tooltip
            contentStyle={{ background: "#111", border: "1px solid #333", borderRadius: 0, fontSize: 12 }}
            labelStyle={{ color: "#fff" }}
          />
          <Legend wrapperStyle={{ fontSize: 11 }} />
          {deviceIds.map((dev, i) => (
            <Line
              key={dev}
              type="monotone"
              dataKey={dev}
              stroke={colors[i % colors.length]}
              dot={false}
              strokeWidth={1}
              isAnimationActive={false}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
