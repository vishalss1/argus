export type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

export type DeviceStatus = "online" | "offline" | "warning" | "critical" | string;

export interface Device {
  id: string;
  name: string;
  type: string;
  firmware_version: string;
  status: DeviceStatus;
  metadata?: JsonValue;
  workspace_id?: string;
  last_seen?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateDeviceRequest {
  id?: string;
  name: string;
  type: string;
  firmware_version: string;
  status?: string;
  metadata?: JsonValue;
}

export interface Telemetry {
  id: string;
  device_id: string;
  recorded_at: string;
  metrics: JsonValue;
  created_at: string;
}

export interface CreateTelemetryRequest {
  recorded_at?: string;
  metrics: JsonValue;
}

export type CommandStatus = "pending" | "acked" | "nacked" | string;
export type OTAStatus = "pending" | "available" | "downloading" | "flashing" | "rebooting" | "acked" | "nacked" | "timeout" | string;

export interface Command {
  id: string;
  device_id: string;
  type: string;
  payload?: JsonValue;
  status: CommandStatus;
  result_message?: string;
  created_at: string;
  sent_at?: string;
  acknowledged_at?: string;
  updated_at: string;
}

export interface FirmwareArtifact {
  id: string;
  version: string;
  filename: string;
  object_key: string;
  content_type: string;
  size_bytes: number;
  checksum_sha256: string;
  signature_alg?: string;
  signature?: string;
  signing_key_id?: string;
  created_at: string;
}

export interface Deployment {
  id: string;
  device_id: string;
  artifact_id: string;
  status: OTAStatus;
  progress: number;
  result_message?: string;
  failure_reason?: string;
  device_name?: string;
  version?: string;
  filename?: string;
  created_at: string;
  available_at?: string;
  downloading_at?: string;
  flashing_at?: string;
  rebooting_at?: string;
  acknowledged_at?: string;
  completed_at?: string;
  failed_at?: string;
  timed_out_at?: string;
  updated_at: string;
}

export interface DeploymentEvent {
  id: number;
  deployment_id: string;
  device_id: string;
  status: OTAStatus;
  progress?: number;
  message?: string;
  created_at: string;
}

export interface OTAFleetStats {
  total_deployments: number;
  successful_deployments: number;
  failed_deployments: number;
  success_rate: number;
  devices_pending_update: number;
}

export interface Manifest {
  deployment_id: string;
  device_id: string;
  firmware_id: string;
  version: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  checksum_sha256: string;
  signature_alg?: string;
  signature?: string;
  signing_key_id?: string;
  download_url: string;
  expires_at: string;
}

export interface Rule {
  id: string;
  name: string;
  metric: string;
  operator: string;
  threshold: number;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface Alert {
  id: string;
  rule_id: string;
  device_id: string;
  telemetry_id: string;
  metric: string;
  operator: string;
  threshold: number;
  observed_value: number;
  message: string;
  created_at: string;
}

export interface Shadow {
  device_id: string;
  desired: JsonValue;
  reported: JsonValue;
  drift: boolean;
  version: number;
  updated_at: string;
}

export interface ApiErrorBody {
  error?: string;
}

export interface MetricSample {
  name: string;
  labels: Record<string, string>;
  value: number;
}

export interface SemanticEvent {
  id: string;
  device_id: string;
  type: string;
  severity: string;
  title: string;
  summary: string;
  source: string;
  confidence_score: number;
  metadata: JsonValue;
  created_at: string;
}

export interface Incident {
  id: string;
  title: string;
  summary: string;
  severity: string;
  status: string;
  device_ids: string[];
  event_ids: string[];
  started_at: string;
  resolved_at?: string;
  created_at: string;
  updated_at: string;
}

export interface OperationalMemory {
  id: string;
  device_id?: string;
  type: string;
  summary: string;
  data: JsonValue;
  timestamp: string;
  created_at: string;
}

export interface ReasoningResponse {
  summary: string;
  confidence: number;
  evidence: string[];
  suggested_actions: string[];
}

export interface Workspace {
  id: string;
  name: string;
  description: string;
  device_count: number;
  created_at: string;
}

export interface DeviceSummary {
  id: string;
  name: string;
  type: string;
  status: string;
  firmware_version: string;
  last_seen?: string;
}

export type SessionStatus = "CREATED" | "RUNNING" | "COMPLETED" | "FAILED" | "CANCELLED";

export interface Session {
  id: string;
  workspace_id: string;
  status: SessionStatus;
  started_at?: string;
  ended_at?: string;
  created_by?: string;
  created_at: string;
}

export interface SessionEvent {
  id: string;
  session_id: string;
  device_id: string;
  event_type: string;
  severity: string;
  payload: JsonValue;
  created_at: string;
}

export interface SessionAlert {
  id: string;
  session_id: string;
  device_id: string;
  severity: string;
  message: string;
  resolved: boolean;
  created_at: string;
  resolved_at?: string;
}

export interface SessionCommand {
  id: string;
  session_id: string;
  device_id: string;
  command: string;
  issued_by?: string;
  status: string;
  issued_at: string;
  completed_at?: string;
}

export interface SessionStatistics {
  session_id: string;
  duration_seconds: number;
  messages_processed: number;
  alerts_count: number;
  critical_events: number;
  uptime_percentage: number;
  average_latency_ms: number;
  updated_at: string;
}

export interface SessionReport {
  id: string;
  session_id: string;
  report_json: JsonValue;
  generated_at: string;
}

export interface SessionAIReport {
  id: string;
  session_id: string;
  summary_text: string;
  metadata: JsonValue;
  generated_at: string;
}
