import type { Action, WorkItem } from "./workflow";

export interface ProjectErrorRank {
  project_id: number;
  project_name: string;
  total_work_items: number;
  failed_work_items: number;
  failure_rate: number;
  failed_runs: number;
}

export interface ActionBottleneck {
  action_id: number;
  action_name: string;
  work_item_id: number;
  work_item_title: string;
  project_id?: number | null;
  avg_duration_s: number;
  max_duration_s: number;
  run_count: number;
  fail_count: number;
  retry_count: number;
  fail_rate: number;
}

export interface WorkItemDurationStat {
  work_item_id: number;
  work_item_title: string;
  project_id?: number | null;
  run_count: number;
  avg_duration_s: number;
  min_duration_s: number;
  max_duration_s: number;
  p50_duration_s: number;
}

export interface ErrorKindCount {
  error_kind: string;
  count: number;
  pct: number;
}

export interface FailureRecord {
  run_id: number;
  action_id: number;
  action_name: string;
  work_item_id: number;
  work_item_title: string;
  project_id?: number | null;
  project_name?: string;
  error_message: string;
  error_kind: string;
  attempt: number;
  duration_s: number;
  failed_at: string;
}

export interface StatusCount {
  status: string;
  count: number;
}

export interface AnalyticsSummary {
  project_errors: ProjectErrorRank[];
  bottlenecks: ActionBottleneck[];
  duration_stats: WorkItemDurationStat[];
  error_breakdown: ErrorKindCount[];
  recent_failures: FailureRecord[];
  status_distribution: StatusCount[];
}

export interface AnalyticsFilter {
  project_id?: number;
  since?: string;
  until?: string;
  limit?: number;
}

// --- Usage / Token Tracking ---

export interface UsageRecord {
  id: number;
  run_id: number;
  work_item_id: number;
  action_id: number;
  project_id?: number | null;
  agent_id: string;
  profile_id?: string;
  model_id?: string;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens?: number;
  cache_write_tokens?: number;
  reasoning_tokens?: number;
  total_tokens: number;
  duration_ms?: number;
  created_at: string;
}

export interface ProjectUsageSummary {
  project_id: number;
  project_name: string;
  run_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  reasoning_tokens: number;
  total_tokens: number;
}

export interface AgentUsageSummary {
  agent_id: string;
  project_id?: number | null;
  project_name?: string;
  run_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  reasoning_tokens: number;
  total_tokens: number;
}

export interface ProfileUsageSummary {
  profile_id: string;
  agent_id: string;
  project_id?: number | null;
  project_name?: string;
  run_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  reasoning_tokens: number;
  total_tokens: number;
}

export interface UsageTotalSummary {
  run_count: number;
  input_tokens: number;
  output_tokens: number;
  cache_read_tokens: number;
  cache_write_tokens: number;
  reasoning_tokens: number;
  total_tokens: number;
}

export interface UsageAnalyticsSummary {
  totals: UsageTotalSummary;
  by_project: ProjectUsageSummary[];
  by_agent: AgentUsageSummary[];
  by_profile: ProfileUsageSummary[];
}

// Cron types

export interface CronStatus {
  work_item_id: number;
  enabled: boolean;
  is_template: boolean;
  schedule?: string;
  max_instances?: number;
  last_triggered?: string;
}

export interface SetupCronRequest {
  schedule: string;
  max_instances?: number;
}

// --- DAG Templates ---

export interface DAGTemplateAction {
  name: string;
  description?: string;
  type: "exec" | "gate" | "composite" | string;
  depends_on?: string[];
  agent_role?: string;
  required_capabilities?: string[];
  acceptance_criteria?: string[];
  profile_id?: string;
}

export interface DAGTemplate {
  id: number;
  name: string;
  description?: string;
  project_id?: number | null;
  tags?: string[];
  metadata?: Record<string, string>;
  actions: DAGTemplateAction[];
  created_at: string;
  updated_at: string;
}

export interface CreateDAGTemplateRequest {
  name: string;
  description?: string;
  project_id?: number;
  tags?: string[];
  metadata?: Record<string, string>;
  actions: DAGTemplateAction[];
}

export interface UpdateDAGTemplateRequest {
  name?: string;
  description?: string;
  project_id?: number;
  tags?: string[];
  metadata?: Record<string, string>;
  actions?: DAGTemplateAction[];
}

export interface SaveWorkItemAsTemplateRequest {
  name?: string;
  description?: string;
  tags?: string[];
  metadata?: Record<string, string>;
}

export interface CreateWorkItemFromTemplateRequest {
  title?: string;
  project_id?: number;
  metadata?: Record<string, unknown>;
}

export interface CreateWorkItemFromTemplateResponse {
  work_item: WorkItem;
  actions: Action[];
}

// --- Git Tags ---

export interface GitCommitEntry {
  sha: string;
  short: string;
  message: string;
  author: string;
  timestamp: string;
}

export interface GitTagEntry {
  name: string;
  sha: string;
  message?: string;
  timestamp?: string;
}

export interface CreateGitTagRequest {
  name: string;
  ref?: string;
  message?: string;
  push?: boolean;
}

export interface CreateGitTagResponse {
  name: string;
  sha: string;
  pushed: boolean;
  push_error?: string;
}

export interface PushGitTagRequest {
  name: string;
}

export interface PushGitTagResponse {
  name: string;
  pushed: boolean;
}

// ---------------------------------------------------------------------------
// Thread (multi-participant discussion)
// ---------------------------------------------------------------------------

