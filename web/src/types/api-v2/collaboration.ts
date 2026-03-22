import type { WorkItem, WorkItemPriority } from "./workflow";

export type ThreadStatus = "active" | "closed" | "archived" | string;

export interface Thread {
  id: number;
  title: string;
  status: ThreadStatus;
  owner_id?: string;
  focus_project_id?: number;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateThreadRequest {
  title: string;
  owner_id?: string;
  focus_project_id?: number;
  metadata?: Record<string, unknown>;
}

export interface UpdateThreadRequest {
  title?: string;
  status?: string;
  owner_id?: string;
  focus_project_id?: number;
  metadata?: Record<string, unknown>;
}

export interface ThreadMessage {
  id: number;
  thread_id: number;
  sender_id: string;
  role: string;
  content: string;
  reply_to_msg_id?: number;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface CreateThreadMessageRequest {
  sender_id?: string;
  role?: string;
  content: string;
  reply_to_msg_id?: number;
  target_agent_id?: string;
  metadata?: Record<string, unknown>;
}

// ---------------------------------------------------------------------------
// Thread Members (unified human + agent)
// ---------------------------------------------------------------------------

export type ThreadAgentSessionStatus =
  | "joining"
  | "booting"
  | "active"
  | "paused"
  | "left"
  | "failed"
  | string;

export interface ThreadMember {
  id: number;
  thread_id: number;
  kind: "human" | "agent" | string;
  user_id?: string;
  agent_profile_id?: string;
  role: string;
  status?: ThreadAgentSessionStatus;
  agent_data?: Record<string, unknown>;
  turn_count?: number;
  total_input_tokens?: number;
  total_output_tokens?: number;
  joined_at: string;
  last_active_at: string;
}

export interface AddThreadParticipantRequest {
  user_id: string;
  role?: string;
}

export interface ThreadAttachment {
  id: number;
  thread_id: number;
  message_id?: number;
  file_name: string;
  file_path: string;
  file_size: number;
  content_type: string;
  is_directory: boolean;
  uploaded_by?: string;
  note?: string;
  created_at: string;
}

// ---------------------------------------------------------------------------
// Thread File References (for # trigger and file picker)
// ---------------------------------------------------------------------------

export interface ThreadFileRef {
  source: "attachment" | "project" | "workspace";
  name: string;
  path: string;
  size?: number;
  content_type?: string;
  is_directory?: boolean;
  project?: string;
  note?: string;
}

export interface MessageFileRef {
  source: "attachment" | "project" | "workspace";
  name: string;
  path: string;
}

// ---------------------------------------------------------------------------
// Thread-WorkItem Links
// ---------------------------------------------------------------------------

export interface ThreadWorkItemLink {
  id: number;
  thread_id: number;
  work_item_id: number;
  relation_type: string;
  is_primary: boolean;
  created_at: string;
}

export interface CreateThreadWorkItemLinkRequest {
  work_item_id: number;
  relation_type?: string;
  is_primary?: boolean;
}

// ---------------------------------------------------------------------------
// Thread Proposals
// ---------------------------------------------------------------------------

export type ProposalStatus =
  | "draft"
  | "open"
  | "approved"
  | "rejected"
  | "revised"
  | "merged"
  | string;

export interface ProposalWorkItemDraft {
  temp_id: string;
  project_id?: number | null;
  title: string;
  body: string;
  priority: WorkItemPriority;
  depends_on?: string[];
  labels?: string[];
}

export interface ThreadProposal {
  id: number;
  thread_id: number;
  title: string;
  summary: string;
  content: string;
  proposed_by: string;
  status: ProposalStatus;
  reviewed_by?: string | null;
  reviewed_at?: string | null;
  review_note?: string;
  work_item_drafts?: ProposalWorkItemDraft[];
  source_message_id?: number | null;
  initiative_id?: number | null;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface CreateThreadProposalRequest {
  title: string;
  summary?: string;
  content?: string;
  proposed_by?: string;
  work_item_drafts?: ProposalWorkItemDraft[];
  source_message_id?: number;
  metadata?: Record<string, unknown>;
}

export interface UpdateThreadProposalRequest {
  title?: string;
  summary?: string;
  content?: string;
  proposed_by?: string;
  work_item_drafts?: ProposalWorkItemDraft[];
  source_message_id?: number;
  metadata?: Record<string, unknown>;
}

export interface ReplaceProposalDraftsRequest {
  work_item_drafts: ProposalWorkItemDraft[];
}

export interface ReviewProposalRequest {
  reviewed_by?: string;
  review_note?: string;
}

// ---------------------------------------------------------------------------
// Initiatives
// ---------------------------------------------------------------------------

export type InitiativeStatus =
  | "draft"
  | "proposed"
  | "approved"
  | "executing"
  | "blocked"
  | "done"
  | "failed"
  | "cancelled"
  | string;

export interface Initiative {
  id: number;
  title: string;
  description: string;
  status: InitiativeStatus;
  created_by: string;
  approved_by?: string | null;
  approved_at?: string | null;
  review_note?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface InitiativeItem {
  id: number;
  initiative_id: number;
  work_item_id: number;
  role?: string;
  created_at: string;
}

export interface InitiativeProgress {
  total: number;
  pending: number;
  running: number;
  blocked: number;
  done: number;
  failed: number;
  cancelled: number;
}

export interface ThreadInitiativeLink {
  id: number;
  thread_id: number;
  initiative_id: number;
  relation_type: string;
  created_at: string;
}

export interface InitiativeDetail {
  initiative: Initiative;
  items: InitiativeItem[];
  work_items: WorkItem[];
  threads: ThreadInitiativeLink[];
  progress: InitiativeProgress;
}

export interface CreateInitiativeRequest {
  title: string;
  description?: string;
  created_by?: string;
  metadata?: Record<string, unknown>;
}

export interface UpdateInitiativeRequest {
  title?: string;
  description?: string;
  metadata?: Record<string, unknown>;
}

export interface ApproveInitiativeRequest {
  approved_by?: string;
}

export interface RejectInitiativeRequest {
  review_note?: string;
}
