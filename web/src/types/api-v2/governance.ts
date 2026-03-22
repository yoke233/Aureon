import type { ActionBottleneck, ErrorKindCount, StatusCount } from "./insight";

export type FeatureStatus = "pending" | "pass" | "fail" | "skipped";

export interface FeatureEntry {
  id: number;
  project_id: number;
  key: string;
  description: string;
  status: FeatureStatus;
  work_item_id?: number | null;
  action_id?: number | null;
  tags?: string[];
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface FeatureManifestSummary {
  project_id: number;
  pass: number;
  fail: number;
  pending: number;
  skipped: number;
  total: number;
}

export interface FeatureManifestSnapshot {
  project_id: number;
  entries: FeatureEntry[];
}

// ---------------------------------------------------------------------------
// Notifications
// ---------------------------------------------------------------------------

export type NotificationLevel = "info" | "success" | "warning" | "error";

export type NotificationChannel = "browser" | "in_app" | "webhook" | "email";

export interface Notification {
  id: number;
  level: NotificationLevel;
  title: string;
  body?: string;
  category?: string;
  action_url?: string;
  project_id?: number | null;
  work_item_id?: number | null;
  run_id?: number | null;
  channels?: NotificationChannel[];
  read: boolean;
  read_at?: string | null;
  created_at: string;
}

export interface CreateNotificationRequest {
  level?: NotificationLevel;
  title: string;
  body?: string;
  category?: string;
  action_url?: string;
  project_id?: number;
  work_item_id?: number;
  run_id?: number;
  channels?: NotificationChannel[];
}

export interface UnreadCountResponse {
  count: number;
}


// ---------------------------------------------------------------------------
// Inspections (self-evolving inspection system)
// ---------------------------------------------------------------------------

export type InspectionStatus = "pending" | "running" | "completed" | "failed";
export type InspectionTrigger = "cron" | "manual";
export type FindingSeverity = "critical" | "high" | "medium" | "low" | "info";
export type FindingCategory = "blocker" | "failure" | "bottleneck" | "pattern" | "waste" | "skill_gap" | "drift";

export interface InspectionSnapshot {
  total_work_items: number;
  active_work_items: number;
  failed_work_items: number;
  blocked_work_items: number;
  success_rate: number;
  avg_duration_s: number;
  total_runs: number;
  failed_runs: number;
  total_tokens: number;
  top_errors?: ErrorKindCount[];
  top_bottlenecks?: ActionBottleneck[];
  status_distribution?: StatusCount[];
}

export interface InspectionFinding {
  id: number;
  inspection_id: number;
  category: FindingCategory;
  severity: FindingSeverity;
  title: string;
  description: string;
  evidence?: string;
  work_item_id?: number | null;
  action_id?: number | null;
  run_id?: number | null;
  project_id?: number | null;
  recommendation?: string;
  recurring: boolean;
  occurrence_count: number;
  created_at: string;
}

export interface InspectionInsight {
  id: number;
  inspection_id: number;
  type: string;
  title: string;
  description: string;
  trend?: string;
  action_items?: string[];
  created_at: string;
}

export interface SuggestedSkill {
  name: string;
  description: string;
  rationale: string;
  skill_md_draft?: string;
}

export interface InspectionReport {
  id: number;
  project_id?: number | null;
  status: InspectionStatus;
  trigger: InspectionTrigger;
  period_start: string;
  period_end: string;
  snapshot?: InspectionSnapshot | null;
  findings?: InspectionFinding[];
  insights?: InspectionInsight[];
  summary?: string;
  suggested_skills?: SuggestedSkill[];
  error_message?: string;
  created_at: string;
  finished_at?: string | null;
}

export interface TriggerInspectionRequest {
  project_id?: number;
  lookback_hours?: number;
}
