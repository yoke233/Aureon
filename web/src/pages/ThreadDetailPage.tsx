import { useCallback, useEffect, useRef, useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  ArrowLeft,
  Loader2,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { ThreadComposerPanel, type ThreadComposerMentionCandidate } from "@/components/threads/ThreadComposerPanel";
import { ThreadDetailHeader } from "@/components/threads/ThreadDetailHeader";
import { ThreadDetailShell } from "@/components/threads/ThreadDetailShell";
import { ThreadSidebar } from "@/components/threads/ThreadSidebar";
import { ThreadMessageList } from "@/components/threads/ThreadMessageList";
import { InvitePickerDialog } from "@/components/threads/InvitePickerDialog";
import { cn } from "@/lib/utils";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { formatRelativeTime, getErrorMessage } from "@/lib/v2Workbench";
import { useThreadDetailResource } from "./thread-detail/useThreadDetailResource";
import { useThreadDetailRealtime } from "./thread-detail/useThreadDetailRealtime";
import { useThreadProposalController } from "./thread-detail/useThreadProposalController";
import type {
  AgentProfile,
  Thread,
  ThreadMessage,
  ThreadMember,
  ThreadFileRef,
  MessageFileRef,
} from "@/types/apiV2";

/* ── helper functions (unchanged) ── */

function deriveWorkItemTitle(thread: Thread): string {
  const title = thread.title.trim();
  return title.length > 80 ? `${title.slice(0, 77)}...` : title;
}

function readTargetAgentID(
  metadata: Record<string, unknown> | undefined,
): string | null {
  const value = metadata?.target_agent_id;
  return typeof value === "string" && value.trim().length > 0
    ? value.trim()
    : null;
}

function readTargetAgentIDs(
  metadata: Record<string, unknown> | undefined,
): string[] {
  const value = metadata?.target_agent_ids;
  if (!Array.isArray(value)) {
    const single = readTargetAgentID(metadata);
    return single ? [single] : [];
  }
  return value
    .filter((item): item is string => typeof item === "string")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

function readAutoRoutedTo(
  metadata: Record<string, unknown> | undefined,
): string[] {
  const value = metadata?.auto_routed_to;
  if (!Array.isArray(value)) return [];
  return value
    .filter((v): v is string => typeof v === "string" && v.trim().length > 0)
    .map((v) => v.trim());
}

function readMetadataType(
  metadata: Record<string, unknown> | undefined,
): string | null {
  const value = metadata?.type;
  return typeof value === "string" && value.trim().length > 0
    ? value.trim()
    : null;
}

function parseMentionTarget(
  message: string,
  activeAgentProfileIDs: string[],
): { targetAgentID: string | null; broadcast: boolean; error: string | null } {
  const trimmed = message.trim();
  const match = trimmed.match(/^@([A-Za-z0-9._:-]+)\s+(.+)$/s);
  if (!match) {
    return { targetAgentID: null, broadcast: false, error: null };
  }

  const targetAgentID = match[1].trim();
  if (targetAgentID === "all") {
    return { targetAgentID: null, broadcast: true, error: null };
  }
  if (!activeAgentProfileIDs.includes(targetAgentID)) {
    return {
      targetAgentID: null,
      broadcast: false,
      error: `未找到活跃 agent：${targetAgentID}`,
    };
  }

  return { targetAgentID, broadcast: false, error: null };
}

function readAgentRoutingMode(
  thread: Thread | null,
): "mention_only" | "broadcast" | "auto" {
  const value = thread?.metadata?.agent_routing_mode;
  if (value === "broadcast") return "broadcast";
  if (value === "auto") return "auto";
  return "mention_only";
}

function readMeetingMode(
  thread: Thread | null,
): "direct" | "concurrent" | "group_chat" {
  const value = thread?.metadata?.meeting_mode;
  if (value === "concurrent") return "concurrent";
  if (value === "group_chat") return "group_chat";
  return "direct";
}

function detectMentionDraft(
  message: string,
  caretPosition: number | null,
): { start: number; end: number; query: string } | null {
  if (caretPosition == null || caretPosition < 0) {
    return null;
  }

  const left = message.slice(0, caretPosition);
  const leftMatch = left.match(/(^|\s)@([A-Za-z0-9._:-]*)$/);
  if (!leftMatch) {
    return null;
  }

  const prefixLength = leftMatch[1]?.length ?? 0;
  const fullMatchLength = leftMatch[0]?.length ?? 0;
  const start = left.length - fullMatchLength + prefixLength;
  const right = message.slice(caretPosition);
  const rightMatch = right.match(/^[A-Za-z0-9._:-]*/);
  const end = caretPosition + (rightMatch?.[0]?.length ?? 0);

  return {
    start,
    end,
    query: message.slice(start + 1, end),
  };
}

function replaceMentionDraft(
  message: string,
  draft: { start: number; end: number },
  profileID: string,
): { nextMessage: string; caretPosition: number } {
  const replacement = `@${profileID} `;
  const nextMessage = `${message.slice(0, draft.start)}${replacement}${message.slice(draft.end)}`;
  return {
    nextMessage,
    caretPosition: draft.start + replacement.length,
  };
}

function splitMessageMentions(
  content: string,
): Array<{ type: "text" | "mention"; value: string; profileID?: string }> {
  const parts: Array<{
    type: "text" | "mention";
    value: string;
    profileID?: string;
  }> = [];
  const mentionPattern = /@([A-Za-z0-9._:-]+)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null = mentionPattern.exec(content);
  while (match) {
    if (match.index > lastIndex) {
      parts.push({
        type: "text",
        value: content.slice(lastIndex, match.index),
      });
    }
    parts.push({ type: "mention", value: match[0], profileID: match[1] });
    lastIndex = match.index + match[0].length;
    match = mentionPattern.exec(content);
  }
  if (lastIndex < content.length) {
    parts.push({ type: "text", value: content.slice(lastIndex) });
  }
  return parts.length > 0 ? parts : [{ type: "text", value: content }];
}

function detectHashDraft(
  message: string,
  caretPosition: number | null,
): { start: number; end: number; query: string } | null {
  if (caretPosition == null || caretPosition < 0) return null;
  const left = message.slice(0, caretPosition);
  const leftMatch = left.match(/(^|\s)#([^\s#]*)$/);
  if (!leftMatch) return null;
  const prefixLength = leftMatch[1]?.length ?? 0;
  const fullMatchLength = leftMatch[0]?.length ?? 0;
  const start = left.length - fullMatchLength + prefixLength;
  const right = message.slice(caretPosition);
  const rightMatch = right.match(/^[^\s#]*/);
  const end = caretPosition + (rightMatch?.[0]?.length ?? 0);
  return { start, end, query: message.slice(start + 1, end) };
}

function readCommittedMentionTarget(
  message: string,
  activeAgentProfileIDs: string[],
): string | null {
  const trimmed = message.trimStart();
  const match = trimmed.match(/^@([A-Za-z0-9._:-]+)(?:\s|$)/);
  if (!match) {
    return null;
  }
  const profileID = match[1].trim();
  return activeAgentProfileIDs.includes(profileID) ? profileID : null;
}

function agentStatusColor(status: string): string {
  switch (status) {
    case "active":
      return "bg-emerald-500";
    case "booting":
      return "bg-amber-500";
    case "paused":
      return "bg-slate-400";
    case "joining":
      return "bg-blue-400";
    default:
      return "bg-rose-500";
  }
}

function canStartDiscussionWithAgent(status: string): boolean {
  return status === "active";
}

// Invite intent detection: match phrases like "把 XX 拉进来", "invite XX", "加个 XX" etc.
const INVITE_PATTERNS = [
  // Chinese patterns
  /(?:把|让|请|叫|邀请)\s*(.+?)\s*(?:拉进来|加进来|拉入|加入|进来|进群|加到|拉到)/,
  /(?:拉|加|邀请)\s*(?:个|一个|一位)?\s*(.+?)\s*(?:进来|进群|到群里|到线程|吧|$)/,
  /(?:需要|想要|想)\s*(.+?)\s*(?:加入|参与|进来|帮忙)/,
  // English patterns
  /(?:invite|add|bring|pull)\s+(?:in\s+)?(.+?)(?:\s+(?:in|to\s+(?:the\s+)?(?:thread|chat|group))|\s*$)/i,
  /(?:let's?\s+)?(?:get|bring)\s+(.+?)\s+(?:in|here|on\s+board)/i,
];

interface InviteIntentMatch {
  query: string;
  matchedProfiles: AgentProfile[];
}

function detectInviteIntent(
  message: string,
  inviteableProfiles: AgentProfile[],
): InviteIntentMatch | null {
  const trimmed = message.trim();
  if (!trimmed) return null;

  for (const pattern of INVITE_PATTERNS) {
    const match = trimmed.match(pattern);
    if (!match || !match[1]) continue;

    const query = match[1].trim().toLowerCase();
    if (!query) continue;

    // Match query against profile name, id, role, capabilities
    const matched = inviteableProfiles.filter((profile) => {
      const name = (profile.name ?? "").toLowerCase();
      const id = profile.id.toLowerCase();
      const role = (
        typeof profile.role === "string" ? profile.role : ""
      ).toLowerCase();
      const caps = (profile.capabilities ?? []).map((c) => c.toLowerCase());

      const includesNonEmpty = (candidate: string) =>
        candidate.length > 0 &&
        (candidate.includes(query) || query.includes(candidate));

      // Check if query contains or is contained by any field
      return (
        includesNonEmpty(name) ||
        includesNonEmpty(id) ||
        includesNonEmpty(role) ||
        caps.some((c) => includesNonEmpty(c))
      );
    });

    if (matched.length > 0) {
      return { query, matchedProfiles: matched };
    }
  }
  return null;
}

type ThreadMemberWithProfileID = ThreadMember & {
  agent_profile_id: string;
};

export function ThreadDetailPage() {
  const { t } = useTranslation();
  const { threadId } = useParams<{ threadId: string }>();
  const navigate = useNavigate();
  const { apiClient, wsClient } = useWorkbench();
  const id = Number(threadId);

  const [error, setError] = useState<string | null>(null);
  const {
    thread,
    setThread,
    messages,
    setMessages,
    participants,
    loading,
    workItemLinks,
    setWorkItemLinks,
    linkedWorkItems,
    setLinkedWorkItems,
    agentSessions,
    setAgentSessions,
    attachments,
    setAttachments,
    availableProfiles,
    refreshAgentSessions,
  } = useThreadDetailResource({
    apiClient,
    threadId: id,
    onError: setError,
  });
  const {
    proposals,
    proposalsLoading,
    showProposalEditor,
    setShowProposalEditor,
    proposalEditor,
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
    resetProposalEditor,
  } = useThreadProposalController({
    apiClient,
    threadId: id,
    ownerId: thread?.owner_id,
    onError: setError,
  });
  const [newMessage, setNewMessage] = useState("");
  const [sending, setSending] = useState(false);
  const [showCreateWI, setShowCreateWI] = useState(false);
  const [newWITitle, setNewWITitle] = useState("");
  const [newWIBody, setNewWIBody] = useState("");
  const [showLinkWI, setShowLinkWI] = useState(false);
  const [linkWIId, setLinkWIId] = useState("");
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [selectedInviteIDs, setSelectedInviteIDs] = useState<Set<string>>(
    new Set(),
  );
  const [selectedDiscussionAgentIDs, setSelectedDiscussionAgentIDs] = useState<
    Set<string>
  >(new Set());
  const [invitingAgent, setInvitingAgent] = useState(false);
  const [removingAgentID, setRemovingAgentID] = useState<number | null>(null);
  const [savingRoutingMode, setSavingRoutingMode] = useState(false);
  const [savingMeetingMode, setSavingMeetingMode] = useState(false);
  const [mentionDraft, setMentionDraft] = useState<{
    start: number;
    end: number;
    query: string;
  } | null>(null);
  const [selectedMentionIndex, setSelectedMentionIndex] = useState(0);
  const [hashDraft, setHashDraft] = useState<{
    start: number;
    end: number;
    query: string;
  } | null>(null);
  const [selectedHashIndex, setSelectedHashIndex] = useState(0);
  const [fileCandidates, setFileCandidates] = useState<ThreadFileRef[]>([]);
  const [selectedFileRefs, setSelectedFileRefs] = useState<MessageFileRef[]>(
    [],
  );
  const [highlightedAgentProfileID, setHighlightedAgentProfileID] = useState<
    string | null
  >(null);
  const [hoveredMentionProfileID, setHoveredMentionProfileID] = useState<
    string | null
  >(null);
  const [invitePickerCandidates, setInvitePickerCandidates] = useState<
    AgentProfile[]
  >([]);
  const [invitePickerSelected, setInvitePickerSelected] = useState<Set<string>>(
    new Set(),
  );
  const [invitePickerBusy, setInvitePickerBusy] = useState(false);
  const messageInputRef = useRef<HTMLTextAreaElement | null>(null);
  const agentCardRefs = useRef<Record<string, HTMLDivElement | null>>({});
  const composerBlurTimeoutRef = useRef<number | null>(null);
  const fileSearchRequestRef = useRef(0);
  const clearComposerState = useCallback(() => {
    setNewMessage("");
    setMentionDraft(null);
    setSelectedMentionIndex(0);
    setHashDraft(null);
    setSelectedHashIndex(0);
    setFileCandidates([]);
    setSelectedFileRefs([]);
  }, []);
  const {
    thinkingAgentIDs,
    agentActivitiesByID,
    liveAgentOutputsByID,
    collapsedAgentActivityPanels,
    visibleAgentActivityIDs,
    pendingThreadRequestIdRef,
    syntheticMessageIDRef,
    messageContainerRef,
    messagesEndRef,
    handleMessageListScroll,
    toggleAgentActivityPanel,
  } = useThreadDetailRealtime({
    wsClient,
    threadId: id,
    messagesLength: messages.length,
    t,
    refreshProposals,
    refreshAgentSessions,
    setMessages,
    setSending,
    onError: setError,
    clearComposerState,
  });

  const agentSessionsWithProfileID = agentSessions.filter(
    (session): session is ThreadMemberWithProfileID =>
      typeof session.agent_profile_id === "string" &&
      session.agent_profile_id.trim().length > 0,
  );
  const joinedAgentProfileIDs = new Set(
    agentSessionsWithProfileID.map((session) => session.agent_profile_id),
  );
  const inviteableProfiles = availableProfiles.filter(
    (profile) => !joinedAgentProfileIDs.has(profile.id),
  );
  const activeAgentProfileIDs = agentSessionsWithProfileID
    .filter(
      (session) => session.status === "active" || session.status === "booting",
    )
    .map((session) => session.agent_profile_id);
  const agentRoutingMode = readAgentRoutingMode(thread);
  const meetingMode = readMeetingMode(thread);
  const profileByID = new Map(
    availableProfiles.map((profile) => [profile.id, profile]),
  );
  const agentSessionByProfileID = new Map(
    agentSessionsWithProfileID.map((session) => [
      session.agent_profile_id,
      session,
    ]),
  );
  const selectedDiscussionAgents = activeAgentProfileIDs.filter((profileID) =>
    selectedDiscussionAgentIDs.has(profileID),
  );
  const committedMentionTargetID = readCommittedMentionTarget(
    newMessage,
    activeAgentProfileIDs,
  );
  const committedMentionProfile = committedMentionTargetID
    ? profileByID.get(committedMentionTargetID)
    : undefined;
  const committedMentionSession = committedMentionTargetID
    ? agentSessionByProfileID.get(committedMentionTargetID)
    : undefined;
  const mentionCandidates: ThreadComposerMentionCandidate[] = (() => {
    if (!mentionDraft) return [];
    const query = mentionDraft.query.trim().toLowerCase();
    const agents = activeAgentProfileIDs
      .map((profileID) => {
        const profile = profileByID.get(profileID);
        const session = agentSessionByProfileID.get(profileID);
        return {
          id: profileID,
          label: profile?.name ? `${profile.name} (${profileID})` : profileID,
          status: session?.status ?? ("active" as string),
        };
      })
      .filter(
        (candidate) =>
          query === "" ||
          candidate.id.toLowerCase().includes(query) ||
          candidate.label.toLowerCase().includes(query),
      );
    // Prepend @all option when there are multiple active agents.
    const allEntry = {
      id: "all",
      label: "All agents (broadcast)",
      status: "active" as string,
    };
    const showAll =
      activeAgentProfileIDs.length > 1 &&
      (query === "" || "all".includes(query));
    return (showAll ? [allEntry, ...agents] : agents).slice(0, 8);
  })();
  const selectedMentionCandidate = mentionCandidates[selectedMentionIndex];
  const orderedWorkItemLinks = [...workItemLinks].sort((a, b) => {
    if (a.is_primary === b.is_primary) {
      return a.id - b.id;
    }
    return a.is_primary ? -1 : 1;
  });
  const orderedProposals = [...proposals].sort((a, b) => b.id - a.id);
  useEffect(() => {
    setSelectedInviteIDs(new Set());
    setSelectedDiscussionAgentIDs(new Set());
    setShowCreateWI(false);
    setShowLinkWI(false);
    setNewWITitle("");
    setNewWIBody("");
    setLinkWIId("");
    setNewMessage("");
    setSelectedFileRefs([]);
    setError(null);
  }, [id]);

  useEffect(() => {
    // Remove selections that are no longer inviteable (e.g. agent already joined)
    setSelectedInviteIDs((prev) => {
      const inviteableSet = new Set(inviteableProfiles.map((p) => p.id));
      const next = new Set([...prev].filter((id) => inviteableSet.has(id)));
      return next.size === prev.size ? prev : next;
    });
  }, [inviteableProfiles]);

  useEffect(() => {
    setSelectedDiscussionAgentIDs((prev) => {
      const selectable = new Set(
        agentSessionsWithProfileID
          .filter((session) =>
            canStartDiscussionWithAgent(session.status ?? ""),
          )
          .map((session) => session.agent_profile_id),
      );
      const next = new Set(
        [...prev].filter((profileID) => selectable.has(profileID)),
      );
      return next.size === prev.size ? prev : next;
    });
  }, [agentSessionsWithProfileID]);

  useEffect(() => {
    if (mentionCandidates.length === 0) {
      setSelectedMentionIndex(0);
      return;
    }
    if (selectedMentionIndex >= mentionCandidates.length) {
      setSelectedMentionIndex(0);
    }
  }, [mentionCandidates.length, selectedMentionIndex]);

  useEffect(() => {
    return () => {
      if (composerBlurTimeoutRef.current != null) {
        window.clearTimeout(composerBlurTimeoutRef.current);
        composerBlurTimeoutRef.current = null;
      }
      fileSearchRequestRef.current += 1;
    };
  }, []);

  /* ── handlers (unchanged) ── */

  const updateMentionDraft = (value: string, caretPosition: number | null) => {
    if (composerBlurTimeoutRef.current != null) {
      window.clearTimeout(composerBlurTimeoutRef.current);
      composerBlurTimeoutRef.current = null;
    }
    const nextMention = detectMentionDraft(value, caretPosition);
    setMentionDraft(nextMention);
    setSelectedMentionIndex(0);

    const nextHash = nextMention ? null : detectHashDraft(value, caretPosition);
    setHashDraft(nextHash);
    setSelectedHashIndex(0);
    if (nextHash && id) {
      const requestID = ++fileSearchRequestRef.current;
      apiClient
        .searchThreadFiles(id, nextHash.query || undefined, "all", 8)
        .then((candidates) => {
          if (fileSearchRequestRef.current !== requestID) {
            return;
          }
          setFileCandidates(candidates);
        })
        .catch(() => {
          if (fileSearchRequestRef.current !== requestID) {
            return;
          }
          setFileCandidates([]);
        });
    } else if (!nextHash) {
      fileSearchRequestRef.current += 1;
      setFileCandidates([]);
    }
  };

  const handleMessageInputChange = (
    value: string,
    caretPosition: number | null,
  ) => {
    setNewMessage(value);
    updateMentionDraft(value, caretPosition);
  };

  const applyMentionCandidate = (profileID: string) => {
    if (!mentionDraft) return;
    const { nextMessage, caretPosition } = replaceMentionDraft(
      newMessage,
      mentionDraft,
      profileID,
    );
    setNewMessage(nextMessage);
    setMentionDraft(null);
    setSelectedMentionIndex(0);
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
      messageInputRef.current?.setSelectionRange(caretPosition, caretPosition);
    });
  };

  const focusAgentProfile = (profileID: string) => {
    setHighlightedAgentProfileID(profileID);
    const node = agentCardRefs.current[profileID];
    if (node) {
      node.scrollIntoView({ behavior: "smooth", block: "nearest" });
    }
  };

  const applyHashCandidate = (file: ThreadFileRef) => {
    if (!hashDraft) return;
    // Remove the #query text from input (don't insert #filename — show chip instead).
    const nextMessage =
      newMessage.slice(0, hashDraft.start) + newMessage.slice(hashDraft.end);
    const caretPosition = hashDraft.start;
    setNewMessage(nextMessage);
    setHashDraft(null);
    setSelectedHashIndex(0);
    setFileCandidates([]);
    setSelectedFileRefs((prev) => {
      if (prev.some((r) => r.path === file.path)) return prev;
      return [
        ...prev,
        { source: file.source, name: file.name, path: file.path },
      ];
    });
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
      messageInputRef.current?.setSelectionRange(caretPosition, caretPosition);
    });
  };

  const removeFileRef = (path: string) => {
    setSelectedFileRefs((prev) => prev.filter((r) => r.path !== path));
  };

  const toggleDiscussionAgentSelection = (profileID: string) => {
    setSelectedDiscussionAgentIDs((prev) => {
      const next = new Set(prev);
      if (next.has(profileID)) {
        next.delete(profileID);
      } else {
        next.add(profileID);
      }
      return next;
    });
  };

  const startDiscussionWithSelectedAgents = () => {
    if (selectedDiscussionAgentIDs.size === 0) return;
    requestAnimationFrame(() => {
      messageInputRef.current?.focus();
    });
  };

  const handleSend = async () => {
    if (!newMessage.trim() || !id) return;

    // Detect invite intent before sending as a regular message.
    const inviteIntent = detectInviteIntent(newMessage, inviteableProfiles);
    if (inviteIntent) {
      if (inviteIntent.matchedProfiles.length === 1) {
        // Single match → auto-invite directly.
        const profile = inviteIntent.matchedProfiles[0];
        setNewMessage("");
        setInvitingAgent(true);
        setError(null);
        try {
          await apiClient.inviteThreadAgent(id, {
            agent_profile_id: profile.id,
          });
          // Agent is now booting — WS events (agent_booted/agent_joined/agent_failed)
          // will drive the UI updates via refreshAgentSessions().
          setMessages((prev) => [
            ...prev,
            {
              id: syntheticMessageIDRef.current--,
              thread_id: id,
              sender_id: "system",
              role: "system",
              content: `已邀请 ${profile.name ?? profile.id} 加入对话，正在初始化...`,
              created_at: new Date().toISOString(),
            },
          ]);
        } catch (e) {
          setError(getErrorMessage(e));
        } finally {
          setInvitingAgent(false);
        }
        return;
      }
      // Multiple matches → show picker dialog.
      setInvitePickerCandidates(inviteIntent.matchedProfiles);
      setInvitePickerSelected(new Set());
      return;
    }

    const mention = parseMentionTarget(newMessage, activeAgentProfileIDs);
    if (mention.error) {
      setError(mention.error);
      return;
    }
    const discussionTargets =
      mention.targetAgentID || mention.broadcast
        ? []
        : activeAgentProfileIDs.filter((profileID) =>
            selectedDiscussionAgentIDs.has(profileID),
          );
    setSending(true);
    setError(null);
    try {
      const requestId = `thread-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      pendingThreadRequestIdRef.current = requestId;
      const sendMetadata: Record<string, unknown> = {};
      if (selectedFileRefs.length > 0) {
        sendMetadata.file_refs = selectedFileRefs;
      }
      if (mention.broadcast) {
        sendMetadata.broadcast = true;
      }
      wsClient.send({
        type: "thread.send",
        data: {
          request_id: requestId,
          thread_id: id,
          message: mention.broadcast
            ? newMessage.trim().replace(/^@all\s+/i, "")
            : newMessage.trim(),
          sender_id: thread?.owner_id || "human",
          target_agent_ids:
            mention.targetAgentID == null &&
            !mention.broadcast &&
            discussionTargets.length > 1
              ? discussionTargets
              : undefined,
          target_agent_id:
            mention.targetAgentID ??
            (discussionTargets.length === 1 ? discussionTargets[0] : undefined),
          metadata:
            Object.keys(sendMetadata).length > 0 ? sendMetadata : undefined,
        },
      });
      if (discussionTargets.length > 0) {
        setSelectedDiscussionAgentIDs(new Set());
      }
    } catch (e) {
      pendingThreadRequestIdRef.current = null;
      setSending(false);
      setError(getErrorMessage(e));
    } finally {
      if (!pendingThreadRequestIdRef.current) {
        setSending(false);
      }
    }
  };

  const handleInvitePickerConfirm = async () => {
    if (!id || invitePickerSelected.size === 0) return;
    setInvitePickerBusy(true);
    setError(null);
    const ids = [...invitePickerSelected];
    try {
      for (const profileID of ids) {
        await apiClient.inviteThreadAgent(id, { agent_profile_id: profileID });
      }
      // Agents are now booting — WS events will drive UI updates.
      const names = ids.map((pid) => {
        const p = invitePickerCandidates.find((c) => c.id === pid);
        return p?.name ?? pid;
      });
      setMessages((prev) => [
        ...prev,
        {
          id: syntheticMessageIDRef.current--,
          thread_id: id,
          sender_id: "system",
          role: "system",
          content: `已邀请 ${names.join(", ")} 加入对话，正在初始化...`,
          created_at: new Date().toISOString(),
        },
      ]);
      setNewMessage("");
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setInvitePickerBusy(false);
      setInvitePickerCandidates([]);
      setInvitePickerSelected(new Set());
    }
  };

  const handleOpenCreateWorkItem = () => {
    if (!thread) return;
    setError(null);
    setShowCreateWI((prev) => {
      const next = !prev;
      if (next) {
        setNewWITitle(deriveWorkItemTitle(thread));
        setNewWIBody("");
      }
      return next;
    });
  };

  const handleCreateWorkItem = async () => {
    if (!newWITitle.trim() || !id) return;
    setError(null);
    try {
      const trimmedBody = newWIBody.trim();
      const workItem = await apiClient.createWorkItemFromThread(id, {
        title: newWITitle.trim(),
        body: trimmedBody !== "" ? trimmedBody : undefined,
      });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      setLinkedWorkItems((prev) => ({ ...prev, [workItem.id]: workItem }));
      setNewWITitle("");
      setNewWIBody("");
      setShowCreateWI(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  const handleLinkWorkItem = async () => {
    const wiId = Number(linkWIId);
    if (!wiId || isNaN(wiId) || !id) return;
    setError(null);
    try {
      await apiClient.createThreadWorkItemLink(id, {
        work_item_id: wiId,
        relation_type: "related",
      });
      const links = await apiClient.listWorkItemsByThread(id);
      setWorkItemLinks(links);
      try {
        const workItem = await apiClient.getWorkItem(wiId);
        setLinkedWorkItems((prev) => ({ ...prev, [wiId]: workItem }));
      } catch {
        /* ignore */
      }
      setLinkWIId("");
      setShowLinkWI(false);
    } catch (e) {
      setError(getErrorMessage(e));
    }
  };

  const toggleInviteSelection = (profileID: string) => {
    setSelectedInviteIDs((prev) => {
      const next = new Set(prev);
      if (next.has(profileID)) {
        next.delete(profileID);
      } else {
        next.add(profileID);
      }
      return next;
    });
  };

  const handleInviteAgent = async () => {
    if (!id || selectedInviteIDs.size === 0) return;
    setInvitingAgent(true);
    setError(null);
    const ids = [...selectedInviteIDs];
    try {
      for (const profileID of ids) {
        await apiClient.inviteThreadAgent(id, { agent_profile_id: profileID });
      }
      const sessions = await apiClient.listThreadAgents(id);
      setAgentSessions(sessions);
      setSelectedInviteIDs(new Set());
    } catch (e) {
      setError(getErrorMessage(e));
      // Refresh sessions in case some succeeded
      try {
        const sessions = await apiClient.listThreadAgents(id);
        setAgentSessions(sessions);
      } catch {
        /* ignore */
      }
    } finally {
      setInvitingAgent(false);
    }
  };

  const handleRemoveAgent = async (agentSessionID: number) => {
    if (!id) return;
    setRemovingAgentID(agentSessionID);
    setError(null);
    try {
      await apiClient.removeThreadAgent(id, agentSessionID);
      const sessions = await apiClient.listThreadAgents(id);
      setAgentSessions(sessions);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setRemovingAgentID(null);
    }
  };

  const handleUploadAttachment = async (file: File) => {
    if (!id) return;
    setAttachmentsLoading(true);
    try {
      const att = await apiClient.uploadThreadAttachment(id, file);
      setAttachments((prev) => [att, ...prev]);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setAttachmentsLoading(false);
    }
  };

  const handleDeleteAttachment = async (attachmentId: number) => {
    if (!id) return;
    setAttachmentsLoading(true);
    try {
      await apiClient.deleteThreadAttachment(id, attachmentId);
      setAttachments((prev) => prev.filter((a) => a.id !== attachmentId));
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setAttachmentsLoading(false);
    }
  };

  const handleSetRoutingMode = async (
    nextMode: "mention_only" | "broadcast" | "auto",
  ) => {
    if (!thread || !id || nextMode === agentRoutingMode) return;
    setSavingRoutingMode(true);
    setError(null);
    try {
      const updated = await apiClient.updateThread(id, {
        metadata: {
          ...(thread.metadata ?? {}),
          agent_routing_mode: nextMode,
        },
      });
      setThread(updated);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingRoutingMode(false);
    }
  };

  const handleSetMeetingMode = async (
    nextMode: "direct" | "concurrent" | "group_chat",
  ) => {
    if (!thread || !id || nextMode === meetingMode) return;
    setSavingMeetingMode(true);
    setError(null);
    try {
      const updated = await apiClient.updateThread(id, {
        metadata: {
          ...(thread.metadata ?? {}),
          meeting_mode: nextMode,
        },
      });
      setThread(updated);
    } catch (e) {
      setError(getErrorMessage(e));
    } finally {
      setSavingMeetingMode(false);
    }
  };

  /* ── render helpers ── */

  const renderMessageContent = (msg: ThreadMessage) => {
    return splitMessageMentions(msg.content).map((part, index) => {
      if (part.type === "text") {
        return <span key={`${msg.id}-text-${index}`}>{part.value}</span>;
      }
      const profileID = part.profileID ?? "";
      const session = agentSessionByProfileID.get(profileID);
      const profile = profileByID.get(profileID);
      return (
        <span
          key={`${msg.id}-mention-${index}`}
          className="relative mx-0.5 inline-flex align-baseline"
        >
          <button
            type="button"
            className="inline-flex items-center rounded-md bg-blue-100 px-1.5 py-0.5 text-xs font-semibold text-blue-800 transition-colors hover:bg-blue-200"
            onClick={() => focusAgentProfile(profileID)}
            onMouseEnter={() => setHoveredMentionProfileID(profileID)}
            onMouseLeave={() =>
              setHoveredMentionProfileID((c) => (c === profileID ? null : c))
            }
          >
            {part.value}
          </button>
          {hoveredMentionProfileID === profileID ? (
            <span
              data-testid={`mention-hover-card-${profileID}`}
              className="pointer-events-none absolute bottom-full left-0 z-30 mb-2 w-56 rounded-lg border border-slate-200 bg-white p-3 text-left shadow-xl"
            >
              <span className="block text-sm font-semibold text-slate-900">
                {profile?.name ?? profileID}
              </span>
              <span className="mt-0.5 block text-xs text-slate-500">
                @{profileID}
              </span>
              <span className="mt-2 inline-flex items-center gap-1.5 rounded-full bg-slate-100 px-2 py-0.5 text-[10px] font-medium text-slate-700">
                <span
                  className={cn(
                    "h-1.5 w-1.5 rounded-full",
                    agentStatusColor(session?.status ?? "unknown"),
                  )}
                />
                {session?.status ?? "not_joined"}
              </span>
              <span className="mt-2 block text-xs text-slate-500">
                {t("threads.turns", "Turns")}: {session?.turn_count ?? 0} |{" "}
                {(
                  ((session?.total_input_tokens ?? 0) +
                    (session?.total_output_tokens ?? 0)) /
                  1000
                ).toFixed(1)}
                k {t("threads.tokens", "tokens")}
              </span>
            </span>
          ) : null}
        </span>
      );
    });
  };

  /* ── loading / not-found states ── */

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-blue-500" />
          <span className="text-sm text-muted-foreground">
            {t("common.loading", "Loading...")}
          </span>
        </div>
      </div>
    );
  }

  if (!thread) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-4">
        <div className="rounded-xl border border-destructive/20 bg-destructive/5 px-6 py-4 text-center">
          <p className="text-sm text-destructive">
            {error || t("threads.notFound", "Thread not found")}
          </p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => navigate("/threads")}>
          <ArrowLeft className="mr-1.5 h-4 w-4" />
          {t("threads.backToList", "Back to Threads")}
        </Button>
      </div>
    );
  }

  /* ── main layout ── */

  return (
    <ThreadDetailShell
      messageContainerRef={messageContainerRef}
      messagesEndRef={messagesEndRef}
      onMessageListScroll={handleMessageListScroll}
      header={
        <ThreadDetailHeader
          thread={thread}
          participantsCount={participants.length}
          agentSessionsCount={agentSessions.length}
          agentRoutingMode={agentRoutingMode}
          meetingMode={meetingMode}
          savingRoutingMode={savingRoutingMode}
          savingMeetingMode={savingMeetingMode}
          formatRelativeTime={formatRelativeTime}
          onBack={() => navigate("/threads")}
          onSetRoutingMode={(mode) => {
            void handleSetRoutingMode(mode);
          }}
          onSetMeetingMode={(mode) => {
            void handleSetMeetingMode(mode);
          }}
        />
      }
      errorBanner={
        error ? (
          <div className="flex items-center justify-between border-b border-destructive/20 bg-destructive/5 px-5 py-2">
            <span className="text-xs text-destructive">{error}</span>
            <button
              type="button"
              className="text-destructive/60 hover:text-destructive"
              onClick={() => setError(null)}
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ) : null
      }
      invitePickerDialog={
        <InvitePickerDialog
          candidates={invitePickerCandidates}
          selectedIDs={invitePickerSelected}
          busy={invitePickerBusy}
          onToggle={(profileID) => {
            setInvitePickerSelected((prev) => {
              const next = new Set(prev);
              if (next.has(profileID)) next.delete(profileID);
              else next.add(profileID);
              return next;
            });
          }}
          onClose={() => {
            setInvitePickerCandidates([]);
            setInvitePickerSelected(new Set());
          }}
          onConfirm={handleInvitePickerConfirm}
        />
      }
      messageList={
        <ThreadMessageList
          messages={messages}
          profileByID={profileByID}
          thinkingAgentIDs={thinkingAgentIDs}
          visibleAgentActivityIDs={visibleAgentActivityIDs}
          agentActivitiesByID={agentActivitiesByID}
          liveAgentOutputsByID={liveAgentOutputsByID}
          collapsedAgentActivityPanels={collapsedAgentActivityPanels}
          sending={sending}
          renderMessageContent={renderMessageContent}
          onToggleAgentActivityPanel={toggleAgentActivityPanel}
          focusAgentProfile={focusAgentProfile}
          readTargetAgentID={readTargetAgentID}
          readTargetAgentIDs={readTargetAgentIDs}
          readAutoRoutedTo={readAutoRoutedTo}
          readMetadataType={readMetadataType}
          formatRelativeTime={formatRelativeTime}
        />
      }
      composer={
        <ThreadComposerPanel
          threadStatus={thread.status}
          agentRoutingMode={agentRoutingMode}
          meetingMode={meetingMode}
          sending={sending}
          newMessage={newMessage}
          messageInputRef={messageInputRef}
          selectedDiscussionAgents={selectedDiscussionAgents}
          profileByID={profileByID}
          selectedFileRefs={selectedFileRefs}
          committedMentionTargetID={committedMentionTargetID}
          committedMentionProfile={committedMentionProfile}
          committedMentionSession={committedMentionSession}
          agentStatusColor={agentStatusColor}
          hashDraftActive={Boolean(hashDraft)}
          fileCandidates={fileCandidates}
          selectedHashIndex={selectedHashIndex}
          mentionDraftActive={Boolean(mentionDraft)}
          mentionCandidates={mentionCandidates}
          selectedMentionIndex={selectedMentionIndex}
          onFocusAgentProfile={focusAgentProfile}
          onRemoveSelectedDiscussionAgent={toggleDiscussionAgentSelection}
          onRemoveFileRef={removeFileRef}
          onChooseHashCandidate={applyHashCandidate}
          onChooseMentionCandidate={applyMentionCandidate}
          onInputChange={(event) =>
            handleMessageInputChange(
              event.target.value,
              event.target.selectionStart,
            )
          }
          onInputClick={(event) =>
            updateMentionDraft(
              event.currentTarget.value,
              event.currentTarget.selectionStart,
            )
          }
          onInputKeyUp={(event) => {
            if (
              event.key === "ArrowDown" ||
              event.key === "ArrowUp" ||
              event.key === "Tab"
            ) {
              return;
            }
            updateMentionDraft(
              event.currentTarget.value,
              event.currentTarget.selectionStart,
            );
          }}
          onInputBlur={() => {
            if (composerBlurTimeoutRef.current != null) {
              window.clearTimeout(composerBlurTimeoutRef.current);
            }
            composerBlurTimeoutRef.current = window.setTimeout(() => {
              composerBlurTimeoutRef.current = null;
              setMentionDraft(null);
              setHashDraft(null);
              setFileCandidates([]);
            }, 120);
          }}
          onInputKeyDown={(event) => {
            if (hashDraft && fileCandidates.length > 0) {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedHashIndex(
                  (prev) => (prev + 1) % fileCandidates.length,
                );
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedHashIndex(
                  (prev) =>
                    (prev - 1 + fileCandidates.length) % fileCandidates.length,
                );
                return;
              }
              if (event.key === "Enter" || event.key === "Tab") {
                event.preventDefault();
                const selected = fileCandidates[selectedHashIndex];
                if (selected) applyHashCandidate(selected);
                return;
              }
              if (event.key === "Escape") {
                setHashDraft(null);
                setFileCandidates([]);
                return;
              }
            }
            if (mentionDraft && mentionCandidates.length > 0) {
              if (event.key === "ArrowDown") {
                event.preventDefault();
                setSelectedMentionIndex(
                  (prev) => (prev + 1) % mentionCandidates.length,
                );
                return;
              }
              if (event.key === "ArrowUp") {
                event.preventDefault();
                setSelectedMentionIndex(
                  (prev) =>
                    (prev - 1 + mentionCandidates.length) %
                    mentionCandidates.length,
                );
                return;
              }
              if (event.key === "Enter" || event.key === "Tab") {
                event.preventDefault();
                if (selectedMentionCandidate) {
                  applyMentionCandidate(selectedMentionCandidate.id);
                }
                return;
              }
              if (event.key === "Escape") {
                setMentionDraft(null);
                return;
              }
            }
            if (event.key === "Backspace" && selectedFileRefs.length > 0) {
              const input = event.currentTarget;
              if (input.selectionStart === 0 && input.selectionEnd === 0) {
                event.preventDefault();
                setSelectedFileRefs((prev) => prev.slice(0, -1));
                return;
              }
            }
            if (event.key === "Enter" && !event.shiftKey) {
              event.preventDefault();
              void handleSend();
            }
          }}
          onInputPaste={(event) => {
            const items = Array.from(event.clipboardData.items);
            const files = items
              .filter((item) => item.kind === "file")
              .map((item) => item.getAsFile())
              .filter((file): file is File => file !== null);
            if (files.length > 0) {
              event.preventDefault();
              files.forEach((file) => void handleUploadAttachment(file));
            }
          }}
          onUploadInputChange={(event) => {
            Array.from(event.target.files ?? []).forEach(
              (file) => void handleUploadAttachment(file),
            );
            event.target.value = "";
          }}
          onSend={() => {
            void handleSend();
          }}
        />
      }
      sidebar={
        <ThreadSidebar
          thread={thread}
          messagesCount={messages.length}
          inviteableProfiles={inviteableProfiles}
          selectedInviteIDs={selectedInviteIDs}
          invitingAgent={invitingAgent}
          onToggleInviteSelection={toggleInviteSelection}
          onInviteAgent={() => {
            void handleInviteAgent();
          }}
          onClearInviteSelection={() => setSelectedInviteIDs(new Set())}
          agentSessionsWithProfileID={agentSessionsWithProfileID}
          selectedDiscussionAgentIDs={selectedDiscussionAgentIDs}
          profileByID={profileByID}
          highlightedAgentProfileID={highlightedAgentProfileID}
          agentCardRefs={agentCardRefs}
          removingAgentID={removingAgentID}
          onRemoveAgent={(id) => {
            void handleRemoveAgent(id);
          }}
          onToggleDiscussionAgentSelection={toggleDiscussionAgentSelection}
          onStartDiscussionWithAgents={startDiscussionWithSelectedAgents}
          onClearDiscussionAgents={() =>
            setSelectedDiscussionAgentIDs(new Set())
          }
          canStartDiscussionWithAgent={canStartDiscussionWithAgent}
          agentStatusColor={agentStatusColor}
          participants={participants}
          proposals={orderedProposals}
          proposalsLoading={proposalsLoading}
          showProposalEditor={showProposalEditor}
          proposalEditor={proposalEditor}
          savingProposal={savingProposal}
          proposalActionLoadingID={proposalActionLoadingID}
          proposalReviewInputs={proposalReviewInputs}
          onOpenCreateProposal={handleOpenCreateProposal}
          onOpenEditProposal={handleOpenEditProposal}
          onShowProposalEditorChange={(open) => {
            setShowProposalEditor(open);
            if (!open) {
              resetProposalEditor();
            }
          }}
          onProposalEditorFieldChange={handleProposalEditorFieldChange}
          onProposalDraftChange={handleProposalDraftChange}
          onAddProposalDraft={handleAddProposalDraft}
          onRemoveProposalDraft={handleRemoveProposalDraft}
          onSaveProposal={handleSaveProposal}
          onProposalReviewInputChange={handleProposalReviewInputChange}
          onSubmitProposal={(proposalId) => {
            void runProposalAction(proposalId, "submit");
          }}
          onApproveProposal={(proposalId) => {
            void runProposalAction(proposalId, "approve");
          }}
          onRejectProposal={(proposalId) => {
            void runProposalAction(proposalId, "reject");
          }}
          onReviseProposal={(proposalId) => {
            void runProposalAction(proposalId, "revise");
          }}
          workItemLinks={workItemLinks}
          orderedWorkItemLinks={orderedWorkItemLinks}
          linkedWorkItems={linkedWorkItems}
          showCreateWI={showCreateWI}
          newWITitle={newWITitle}
          newWIBody={newWIBody}
          showLinkWI={showLinkWI}
          linkWIId={linkWIId}
          onOpenCreateWorkItem={handleOpenCreateWorkItem}
          onShowCreateWIChange={setShowCreateWI}
          onNewWITitleChange={setNewWITitle}
          onNewWIBodyChange={setNewWIBody}
          onCreateWorkItem={handleCreateWorkItem}
          onShowLinkWIChange={setShowLinkWI}
          onLinkWIIdChange={setLinkWIId}
          onLinkWorkItem={handleLinkWorkItem}
          onResetCreateWorkItemDraft={() => {
            setNewWITitle("");
            setNewWIBody("");
          }}
          attachments={attachments}
          attachmentsLoading={attachmentsLoading}
          onUploadAttachment={(file) => {
            void handleUploadAttachment(file);
          }}
          onDeleteAttachment={(attId) => {
            void handleDeleteAttachment(attId);
          }}
          getAttachmentDownloadUrl={apiClient.getThreadAttachmentDownloadUrl}
        />
      }
    />
  );
}
