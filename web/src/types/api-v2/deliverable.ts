export type DeliverableKind =
  | "document"
  | "code_change"
  | "pull_request"
  | "decision"
  | "meeting_summary"
  | "aggregate_report"
  | string;

export type DeliverableProducerType = "run" | "thread" | "workitem" | string;

export type DeliverableStatus = "draft" | "final" | "adopted" | string;

export interface Deliverable {
  id: number;
  work_item_id?: number | null;
  thread_id?: number | null;
  kind: DeliverableKind;
  title?: string;
  summary?: string;
  payload?: Record<string, unknown>;
  producer_type: DeliverableProducerType;
  producer_id: number;
  status: DeliverableStatus;
  created_at: string;
}
