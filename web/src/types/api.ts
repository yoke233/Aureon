import type {
  ChatSession,
  Pipeline,
  Project,
  TaskItemStatus,
  TaskPlan,
} from "./workflow";

export interface CreateProjectRequest {
  name: string;
  repo_path: string;
  github?: {
    owner?: string;
    repo?: string;
  };
}

export interface CreatePipelineRequest {
  name: string;
  description?: string;
  template: string;
  config?: Record<string, unknown>;
}

export interface CreateChatRequest {
  message: string;
}

export interface CreatePlanRequest {
  session_id: string;
  name?: string;
  fail_policy?: "block" | "skip" | "human";
}

export type ListProjectsResponse = Project[];

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  offset: number;
}

export type ListPipelinesResponse = PaginatedResponse<Pipeline>;
export type ListPlansResponse = PaginatedResponse<TaskPlan>;

export interface PlanDagNode {
  id: string;
  title: string;
  status: TaskItemStatus;
  pipeline_id: string;
}

export interface PlanDagEdge {
  from: string;
  to: string;
}

export interface PlanDagStats {
  total: number;
  pending: number;
  ready: number;
  running: number;
  done: number;
  failed: number;
}

export interface PlanDagResponse {
  nodes: PlanDagNode[];
  edges: PlanDagEdge[];
  stats: PlanDagStats;
}

export interface ApiStatsResponse {
  total_pipelines: number;
  active_pipelines: number;
  success_rate: number;
  avg_duration: string;
  tokens_used: {
    claude: number;
    codex: number;
  };
}

export interface CreateChatResponse {
  session_id: string;
  reply: string;
}

export type GetChatResponse = ChatSession;
export type CreatePlanResponse = TaskPlan;
