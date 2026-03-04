export interface WsEnvelope<TPayload = unknown> {
  type: string;
  pipeline_id?: string;
  project_id?: string;
  plan_id?: string;
  issue_id?: string;
  session_id?: string;
  data?: TPayload;
  payload?: TPayload;
}

export type ChatEventType =
  | "chat_run_started"
  | "chat_run_update"
  | "chat_run_completed"
  | "chat_run_failed"
  | "chat_run_cancelled";

export interface ACPSessionUpdate {
  sessionUpdate?: string;
  content?: {
    type?: string;
    text?: string;
    [key: string]: unknown;
  };
  toolCallId?: string;
  title?: string;
  kind?: string;
  status?: string;
  entries?: Array<{
    content?: string;
    priority?: string;
    status?: string;
    [key: string]: unknown;
  }>;
  [key: string]: unknown;
}

export interface ChatEventPayload {
  session_id?: string;
  role?: string;
  agent_session_id?: string;
  reply?: string;
  error?: string;
  acp?: ACPSessionUpdate;
  timestamp?: string;
  [key: string]: unknown;
}

export interface ChatEventEnvelope extends WsEnvelope<ChatEventPayload> {
  type: ChatEventType;
}

export type ProjectCreateEventType =
  | "project_create_started"
  | "project_create_progress"
  | "project_create_succeeded"
  | "project_create_failed";

export interface ProjectCreateEventPayload {
  request_id?: string;
  project_id?: string;
  progress?: number;
  message?: string;
  error?: string;
}

export interface ProjectCreateEventEnvelope
  extends WsEnvelope<ProjectCreateEventPayload> {
  type: ProjectCreateEventType;
}

export interface WsClientMessage {
  type:
    | "subscribe_plan"
    | "unsubscribe_plan"
    | "subscribe_pipeline"
    | "unsubscribe_pipeline"
    | "subscribe_issue"
    | "unsubscribe_issue"
    | "subscribe_chat_session"
    | "unsubscribe_chat_session";
  plan_id?: string;
  pipeline_id?: string;
  issue_id?: string;
  session_id?: string;
}

export type WsEventHandler<TPayload = unknown> = (
  payload: TPayload,
  raw: MessageEvent<string>,
) => void;

export interface WsClientOptions {
  baseUrl: string;
  getToken?: () => string | null | undefined;
  reconnectIntervalMs?: number;
  maxReconnectIntervalMs?: number;
}
