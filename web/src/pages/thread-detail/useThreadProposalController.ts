import { useCallback, useEffect, useState } from "react";
import type { ApiClient } from "@/lib/apiClient";
import { getErrorMessage } from "@/lib/v2Workbench";
import type {
  ProposalWorkItemDraft,
  ThreadProposal,
  WorkItemPriority,
} from "@/types/apiV2";

type ProposalDraftForm = {
  temp_id: string;
  project_id: string;
  title: string;
  body: string;
  priority: WorkItemPriority;
  depends_on: string;
  labels: string;
};

type ProposalEditorState = {
  proposalId: number | null;
  title: string;
  summary: string;
  content: string;
  proposedBy: string;
  sourceMessageId: string;
  drafts: ProposalDraftForm[];
};

type ProposalReviewState = {
  reviewedBy: string;
  reviewNote: string;
};

interface UseThreadProposalControllerOptions {
  apiClient: ApiClient;
  threadId: number;
  ownerId?: string;
  onError: (message: string | null) => void;
}

function splitDelimitedValues(raw: string): string[] {
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function normalizeDraftTempID(raw: string, fallback: string): string {
  const normalized = raw
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9._:-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return normalized.length > 0 ? normalized : fallback;
}

function createEmptyProposalDraft(index = 1): ProposalDraftForm {
  return {
    temp_id: `draft-${index}`,
    project_id: "",
    title: "",
    body: "",
    priority: "medium",
    depends_on: "",
    labels: "",
  };
}

function toProposalDraftForm(
  draft: ProposalWorkItemDraft,
  index: number,
): ProposalDraftForm {
  return {
    temp_id: draft.temp_id || `draft-${index + 1}`,
    project_id:
      typeof draft.project_id === "number" ? String(draft.project_id) : "",
    title: draft.title ?? "",
    body: draft.body ?? "",
    priority: draft.priority ?? "medium",
    depends_on: (draft.depends_on ?? []).join(", "),
    labels: (draft.labels ?? []).join(", "),
  };
}

function createProposalEditorState(ownerId?: string): ProposalEditorState {
  return {
    proposalId: null,
    title: "",
    summary: "",
    content: "",
    proposedBy: ownerId?.trim() || "human",
    sourceMessageId: "",
    drafts: [createEmptyProposalDraft()],
  };
}

function createProposalEditorStateFromProposal(
  proposal: ThreadProposal,
  ownerId?: string,
): ProposalEditorState {
  return {
    proposalId: proposal.id,
    title: proposal.title,
    summary: proposal.summary ?? "",
    content: proposal.content ?? "",
    proposedBy: proposal.proposed_by || ownerId?.trim() || "human",
    sourceMessageId:
      typeof proposal.source_message_id === "number"
        ? String(proposal.source_message_id)
        : "",
    drafts:
      proposal.work_item_drafts && proposal.work_item_drafts.length > 0
        ? proposal.work_item_drafts.map(toProposalDraftForm)
        : [createEmptyProposalDraft()],
  };
}

function buildProposalDraftPayload(
  draft: ProposalDraftForm,
  index: number,
): ProposalWorkItemDraft | null {
  const title = draft.title.trim();
  const body = draft.body.trim();
  const projectRaw = draft.project_id.trim();
  const tempID = normalizeDraftTempID(
    draft.temp_id || draft.title,
    `draft-${index + 1}`,
  );
  if (
    !title &&
    !body &&
    !projectRaw &&
    !draft.depends_on.trim() &&
    !draft.labels.trim()
  ) {
    return null;
  }

  const projectID =
    projectRaw.length > 0 && Number.isFinite(Number(projectRaw))
      ? Number(projectRaw)
      : undefined;

  return {
    temp_id: tempID,
    project_id: typeof projectID === "number" ? projectID : undefined,
    title,
    body,
    priority: draft.priority ?? "medium",
    depends_on: splitDelimitedValues(draft.depends_on),
    labels: splitDelimitedValues(draft.labels),
  };
}

function createProposalReviewState(
  proposal: ThreadProposal,
  ownerId?: string,
): ProposalReviewState {
  return {
    reviewedBy: proposal.reviewed_by?.trim() || ownerId?.trim() || "human",
    reviewNote: proposal.review_note ?? "",
  };
}

export function useThreadProposalController({
  apiClient,
  threadId,
  ownerId,
  onError,
}: UseThreadProposalControllerOptions) {
  const [proposals, setProposals] = useState<ThreadProposal[]>([]);
  const [proposalsLoading, setProposalsLoading] = useState(false);
  const [showProposalEditor, setShowProposalEditor] = useState(false);
  const [proposalEditor, setProposalEditor] = useState<ProposalEditorState>(() =>
    createProposalEditorState(),
  );
  const [savingProposal, setSavingProposal] = useState(false);
  const [proposalActionLoadingID, setProposalActionLoadingID] = useState<number | null>(null);
  const [proposalReviewInputs, setProposalReviewInputs] = useState<Record<number, ProposalReviewState>>({});

  const refreshProposals = useCallback(async () => {
    if (!threadId || Number.isNaN(threadId)) {
      return;
    }
    if (typeof apiClient.listThreadProposals !== "function") {
      setProposals([]);
      return;
    }
    setProposalsLoading(true);
    try {
      const items = await apiClient.listThreadProposals(threadId);
      setProposals(items);
      setProposalReviewInputs((prev) => {
        const next: Record<number, ProposalReviewState> = {};
        items.forEach((proposal) => {
          next[proposal.id] = prev[proposal.id] ?? createProposalReviewState(proposal, ownerId);
        });
        return next;
      });
    } catch (error) {
      onError(getErrorMessage(error));
    } finally {
      setProposalsLoading(false);
    }
  }, [apiClient, threadId, ownerId, onError]);

  useEffect(() => {
    setProposals([]);
    setShowProposalEditor(false);
    setProposalEditor(createProposalEditorState(ownerId));
    setSavingProposal(false);
    setProposalActionLoadingID(null);
    setProposalReviewInputs({});
  }, [threadId, ownerId]);

  useEffect(() => {
    void refreshProposals();
  }, [refreshProposals]);

  useEffect(() => {
    setProposalEditor((current) =>
      current.proposalId == null ? createProposalEditorState(ownerId) : current,
    );
  }, [ownerId]);

  const handleOpenCreateProposal = useCallback(() => {
    onError(null);
    setProposalEditor(createProposalEditorState(ownerId));
    setShowProposalEditor(true);
  }, [ownerId, onError]);

  const handleOpenEditProposal = useCallback(
    (proposal: ThreadProposal) => {
      onError(null);
      setProposalEditor(createProposalEditorStateFromProposal(proposal, ownerId));
      setShowProposalEditor(true);
    },
    [ownerId, onError],
  );

  const handleProposalEditorFieldChange = useCallback(
    (field: Exclude<keyof ProposalEditorState, "drafts">, value: string | number | null) => {
      setProposalEditor((prev) => ({
        ...prev,
        [field]: value == null ? "" : String(value),
      }));
    },
    [],
  );

  const handleProposalDraftChange = useCallback(
    (index: number, field: keyof ProposalDraftForm, value: string) => {
      setProposalEditor((prev) => ({
        ...prev,
        drafts: prev.drafts.map((draft, draftIndex) =>
          draftIndex === index ? { ...draft, [field]: value } : draft,
        ),
      }));
    },
    [],
  );

  const handleAddProposalDraft = useCallback(() => {
    setProposalEditor((prev) => ({
      ...prev,
      drafts: [...prev.drafts, createEmptyProposalDraft(prev.drafts.length + 1)],
    }));
  }, []);

  const handleRemoveProposalDraft = useCallback((index: number) => {
    setProposalEditor((prev) => {
      if (prev.drafts.length === 1) {
        return { ...prev, drafts: [createEmptyProposalDraft()] };
      }
      return {
        ...prev,
        drafts: prev.drafts.filter((_, draftIndex) => draftIndex !== index),
      };
    });
  }, []);

  const handleSaveProposal = useCallback(async () => {
    if (!proposalEditor.title.trim() || !threadId) {
      return;
    }

    const sourceMessageID = proposalEditor.sourceMessageId.trim();
    if (sourceMessageID.length > 0 && !Number.isInteger(Number(sourceMessageID))) {
      onError("Source message ID 必须是数字。");
      return;
    }
    const invalidProjectDraft = proposalEditor.drafts.find(
      (draft) =>
        draft.project_id.trim().length > 0 &&
        !Number.isInteger(Number(draft.project_id.trim())),
    );
    if (invalidProjectDraft) {
      onError("Draft project_id 必须是数字。");
      return;
    }

    setSavingProposal(true);
    onError(null);
    const drafts = proposalEditor.drafts
      .map((draft, index) => buildProposalDraftPayload(draft, index))
      .filter((draft): draft is ProposalWorkItemDraft => draft !== null);
    try {
      if (proposalEditor.proposalId == null) {
        await apiClient.createThreadProposal(threadId, {
          title: proposalEditor.title.trim(),
          summary: proposalEditor.summary.trim(),
          content: proposalEditor.content.trim(),
          proposed_by: proposalEditor.proposedBy.trim() || ownerId || "human",
          source_message_id:
            sourceMessageID.length > 0 ? Number(sourceMessageID) : undefined,
          work_item_drafts: drafts,
        });
      } else {
        await apiClient.updateProposal(proposalEditor.proposalId, {
          title: proposalEditor.title.trim(),
          summary: proposalEditor.summary.trim(),
          content: proposalEditor.content.trim(),
          proposed_by: proposalEditor.proposedBy.trim() || ownerId || "human",
          work_item_drafts: drafts,
          source_message_id:
            sourceMessageID.length > 0 ? Number(sourceMessageID) : undefined,
        });
      }
      await refreshProposals();
      setProposalEditor(createProposalEditorState(ownerId));
      setShowProposalEditor(false);
    } catch (error) {
      onError(getErrorMessage(error));
    } finally {
      setSavingProposal(false);
    }
  }, [apiClient, onError, ownerId, proposalEditor, refreshProposals, threadId]);

  const handleProposalReviewInputChange = useCallback(
    (proposalId: number, field: keyof ProposalReviewState, value: string) => {
      setProposalReviewInputs((prev) => ({
        ...prev,
        [proposalId]: {
          ...(prev[proposalId] ?? {
            reviewedBy: ownerId || "human",
            reviewNote: "",
          }),
          [field]: value,
        },
      }));
    },
    [ownerId],
  );

  const runProposalAction = useCallback(
    async (proposalId: number, action: "submit" | "approve" | "reject" | "revise") => {
      setProposalActionLoadingID(proposalId);
      onError(null);
      try {
        const reviewInput = proposalReviewInputs[proposalId] ?? {
          reviewedBy: ownerId || "human",
          reviewNote: "",
        };
        if (action === "submit") {
          await apiClient.submitProposal(proposalId);
        } else if (action === "approve") {
          await apiClient.approveProposal(proposalId, {
            reviewed_by: reviewInput.reviewedBy.trim() || ownerId || "human",
            review_note: reviewInput.reviewNote.trim(),
          });
        } else if (action === "reject") {
          await apiClient.rejectProposal(proposalId, {
            reviewed_by: reviewInput.reviewedBy.trim() || ownerId || "human",
            review_note: reviewInput.reviewNote.trim(),
          });
        } else {
          await apiClient.reviseProposal(proposalId, {
            reviewed_by: reviewInput.reviewedBy.trim() || ownerId || "human",
            review_note: reviewInput.reviewNote.trim(),
          });
        }
        await refreshProposals();
      } catch (error) {
        onError(getErrorMessage(error));
      } finally {
        setProposalActionLoadingID(null);
      }
    },
    [apiClient, onError, ownerId, proposalReviewInputs, refreshProposals],
  );

  return {
    proposals,
    proposalsLoading,
    showProposalEditor,
    setShowProposalEditor,
    proposalEditor,
    setProposalEditor,
    savingProposal,
    proposalActionLoadingID,
    proposalReviewInputs,
    refreshProposals,
    handleOpenCreateProposal,
    handleOpenEditProposal,
    handleProposalEditorFieldChange,
    handleProposalDraftChange,
    handleAddProposalDraft,
    handleRemoveProposalDraft,
    handleSaveProposal,
    handleProposalReviewInputChange,
    runProposalAction,
    resetProposalEditor: () => setProposalEditor(createProposalEditorState(ownerId)),
  };
}
