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
  fleet_id?: string;
  last_seen?: string;
  created_at: string;
  updated_at: string;
  api_key?: string;
}

export interface CreateDeviceRequest {
  id?: string;
  name: string;
  type: string;
  firmware_version: string;
  status?: string;
  metadata?: JsonValue;
  fleet_id?: string;
}

export interface Fleet {
	id: string;
	workspace_id: string;
	name: string;
	node_role: string;
	hardware_type: string;
	node_prefix: string;
	firmware_version: string;
	firmware_template: string;
	node_count: number;
	total_nodes: number;
	online_nodes: number;
	offline_nodes: number;
	devices?: Device[];
	created_at: string;
}

export interface CreateFleetRequest {
	name: string;
	node_role: string;
	hardware_type: string;
	node_prefix: string;
	node_count: number;
	firmware_version: string;
	firmware_template: string;
	wifi_ssid?: string;
	wifi_password?: string;
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
  response: string;
  intent: QueryIntent;
  sources: string[];
  actions: ActionSuggestion[];
}

export interface ActionSuggestion {
  suggestion_id: string;
  action: string;
  device_id: string;
  description: string;
  severity: string;
}

export type QueryIntent =
  | "DEVICE_SUMMARY"
  | "ROOT_CAUSE_ANALYSIS"
  | "REMEDIATION"
  | "INCIDENT_LOOKUP"
  | "FLEET_SUMMARY"
  | "DEVICE_COMPARISON"
  | "UNKNOWN";

export interface AIDeviceSummary {
  deviceId: string;
  deviceName: string;
  deviceStatus: string;
  severity: string;
  openIncidents: number;
  recentIncidents: number;
  incidentsLast24h: number;
  incidentsLast7d: number;
  healthScore: number;
  keyFindings: string[];
  recommendations: string[];
}

export interface RootCauseAnalysis {
  confidence: number;
  primaryCause: string;
  supportingEvidence: string[];
  alternativeCauses: string[];
  recommendedActions: string[];
}

export interface RemediationAnalysis {
  pattern: string;
  reasoning: string;
  possibleCauses: string[];
  actions: string[];
}

export interface RelatedDevice {
  deviceId: string;
  deviceName: string;
  similarity: number;
  sharedPatterns: string[];
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
  average_battery?: number;
  minimum_battery?: number;
  maximum_battery?: number;
  average_temperature?: number;
  minimum_temperature?: number;
  maximum_temperature?: number;
  distance_travelled?: number;
  device_participation_count?: number;
  command_count?: number;
  anomaly_count?: number;
  updated_at: string;
}


export interface DeviceStatusInfo {
  device_id: string;
  status: string;
  severity: string;
  active_incidents: number;
  open_incidents: {
    metric: string;
    incident_type: string;
    severity: string;
  }[];
}

export interface ActiveIncident {
  device_id: string;
  metric: string;
  incident_type: string;
  severity: string;
  start_time: string;
  last_seen: string;
  occurrences: number;
  peak_score: number;
  summary: string;
}

export interface DeviceSummaryArtifact {
  device_id: string;
  first_seen: string;
  last_seen: string;
  sample_count: number;
  warning_incidents_count: number;
  critical_incidents_count: number;
  active_at_end: boolean;
}

export interface ArtifactIncident {
  device_id: string;
  metric: string;
  incident_type: string;
  severity: string;
  start_time: string;
  resolved_at?: string;
  occurrences: number;
  peak_score: number;
  summary: string;
}

export interface MetricAggregate {
  count: number;
  min: number;
  max: number;
  average: number;
  variance: number;
}

export interface HourlySummaryArtifact {
  device_id: string;
  hour: string;
  metric: string;
  sample_count: number;
  min: number;
  max: number;
  average: number;
  variance: number;
  stddev: number;
  first_timestamp: string;
  last_timestamp: string;
}

export interface TelemetryRow {
  timestamp: string;
  device_id: string;
  metrics: Record<string, number>;
}

export interface TelemetryExportPaths {
  json: string;
  csv: string;
}

export interface SessionArtifact {
  session_id: string;
  generated_at: string;
  report_version: string;
  workspace_id: string;
  session_summary: string;
  device_summaries: Record<string, DeviceSummaryArtifact>;
  incidents_archive: ArtifactIncident[];
  metrics_aggregates: Record<string, Record<string, MetricAggregate>>;
  hourly_summaries?: Record<string, HourlySummaryArtifact[]>;
  telemetry_export_paths?: TelemetryExportPaths;
  exports_expired?: boolean;
}
