export interface WsEnvelope<TPayload = unknown> {
  type: string;
  pipeline_id?: string;
  project_id?: string;
  plan_id?: string;
  data?: TPayload;
  payload?: TPayload;
}

export interface WsClientMessage {
  type:
    | "subscribe_plan"
    | "unsubscribe_plan"
    | "subscribe_pipeline"
    | "unsubscribe_pipeline";
  plan_id?: string;
  pipeline_id?: string;
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
