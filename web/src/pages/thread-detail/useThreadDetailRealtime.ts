import {
  startTransition,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type UIEvent,
} from "react";
import type { TFunction } from "i18next";
import { applyActivityPayload } from "@/components/chat/chatUtils";
import type { ChatActivityView } from "@/components/chat/chatTypes";
import type { WsClient } from "@/lib/wsClient";
import type { ThreadAckPayload, ThreadEventPayload } from "@/types/ws";
import type { ThreadMessage } from "@/types/apiV2";

type ThreadAgentLiveOutput = {
  thought?: string;
  message?: string;
  updatedAt: string;
};

type ThreadAgentChunkBuffer = {
  thought?: string;
  message?: string;
};

interface UseThreadDetailRealtimeOptions {
  wsClient: WsClient;
  threadId: number;
  messagesLength: number;
  t: TFunction;
  refreshProposals: () => Promise<void>;
  refreshAgentSessions: () => Promise<void>;
  setMessages: React.Dispatch<React.SetStateAction<ThreadMessage[]>>;
  setSending: React.Dispatch<React.SetStateAction<boolean>>;
  onError: (message: string | null) => void;
  clearComposerState: () => void;
}

export function useThreadDetailRealtime({
  wsClient,
  threadId,
  messagesLength,
  t,
  refreshProposals,
  refreshAgentSessions,
  setMessages,
  setSending,
  onError,
  clearComposerState,
}: UseThreadDetailRealtimeOptions) {
  const [thinkingAgentIDs, setThinkingAgentIDs] = useState<Set<string>>(
    new Set(),
  );
  const [agentActivitiesByID, setAgentActivitiesByID] = useState<
    Record<string, ChatActivityView[]>
  >({});
  const [liveAgentOutputsByID, setLiveAgentOutputsByID] = useState<
    Record<string, ThreadAgentLiveOutput>
  >({});
  const [collapsedAgentActivityPanels, setCollapsedAgentActivityPanels] =
    useState<Record<string, boolean>>({});
  const pendingThreadRequestIdRef = useRef<string | null>(null);
  const syntheticMessageIDRef = useRef(-1);
  const messageContainerRef = useRef<HTMLDivElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const isNearMessageListBottomRef = useRef(true);
  const pendingAgentChunkBuffersRef = useRef<
    Record<string, ThreadAgentChunkBuffer>
  >({});
  const agentChunkFlushFrameRef = useRef<number | null>(null);

  const handleMessageListScroll = useCallback(
    (event: UIEvent<HTMLDivElement>) => {
      const element = event.currentTarget;
      isNearMessageListBottomRef.current =
        element.scrollHeight - element.scrollTop - element.clientHeight < 80;
    },
    [],
  );

  useEffect(() => {
    if (!isNearMessageListBottomRef.current) {
      return;
    }
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messagesLength]);

  useEffect(() => {
    setThinkingAgentIDs(new Set());
    setAgentActivitiesByID({});
    setLiveAgentOutputsByID({});
    setCollapsedAgentActivityPanels({});
    pendingAgentChunkBuffersRef.current = {};
    pendingThreadRequestIdRef.current = null;
    isNearMessageListBottomRef.current = true;
    if (agentChunkFlushFrameRef.current != null) {
      cancelAnimationFrame(agentChunkFlushFrameRef.current);
      agentChunkFlushFrameRef.current = null;
    }
  }, [threadId]);

  useEffect(() => {
    if (!threadId || Number.isNaN(threadId)) {
      return;
    }

    const clearAgentActivityState = (profileID: string) => {
      setAgentActivitiesByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
      setLiveAgentOutputsByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
    };

    const clearLiveAgentOutputField = (
      profileID: string,
      field: keyof ThreadAgentChunkBuffer,
    ) => {
      setLiveAgentOutputsByID((prev) => {
        const current = prev[profileID];
        if (!current || !current[field]) {
          return prev;
        }
        const nextEntry = { ...current };
        delete nextEntry[field];
        if (!nextEntry.thought && !nextEntry.message) {
          const next = { ...prev };
          delete next[profileID];
          return next;
        }
        return {
          ...prev,
          [profileID]: nextEntry,
        };
      });
    };

    const clearLiveAgentOutput = (profileID: string) => {
      setLiveAgentOutputsByID((prev) => {
        if (!(profileID in prev)) {
          return prev;
        }
        const next = { ...prev };
        delete next[profileID];
        return next;
      });
    };

    const flushAgentChunkBuffers = () => {
      if (agentChunkFlushFrameRef.current != null) {
        cancelAnimationFrame(agentChunkFlushFrameRef.current);
        agentChunkFlushFrameRef.current = null;
      }
      const pending = pendingAgentChunkBuffersRef.current;
      const profileIDs = Object.keys(pending);
      if (profileIDs.length === 0) {
        return;
      }
      pendingAgentChunkBuffersRef.current = {};
      const nowISO = new Date().toISOString();
      startTransition(() => {
        setLiveAgentOutputsByID((prev) => {
          const next = { ...prev };
          for (const profileID of profileIDs) {
            const chunk = pending[profileID];
            if (!chunk) {
              continue;
            }
            const current = next[profileID];
            next[profileID] = {
              thought:
                `${current?.thought ?? ""}${chunk.thought ?? ""}` || undefined,
              message:
                `${current?.message ?? ""}${chunk.message ?? ""}` || undefined,
              updatedAt: nowISO,
            };
          }
          return next;
        });
      });
    };

    const scheduleAgentChunkFlush = () => {
      if (agentChunkFlushFrameRef.current != null) {
        return;
      }
      agentChunkFlushFrameRef.current = requestAnimationFrame(() => {
        agentChunkFlushFrameRef.current = null;
        flushAgentChunkBuffers();
      });
    };

    const appendRealtimeMessage = (
      payload: ThreadEventPayload,
      roleFallback: "human" | "agent",
    ) => {
      const content =
        typeof payload.content === "string" && payload.content.trim().length > 0
          ? payload.content
          : typeof payload.message === "string"
            ? payload.message
            : "";
      if (!content.trim()) {
        return;
      }

      const senderID =
        typeof payload.sender_id === "string" && payload.sender_id.trim().length > 0
          ? payload.sender_id.trim()
          : typeof payload.profile_id === "string" &&
              payload.profile_id.trim().length > 0
            ? payload.profile_id.trim()
            : roleFallback;
      const role =
        typeof payload.role === "string" && payload.role.trim().length > 0
          ? payload.role.trim()
          : roleFallback;

      const msgMetadata: Record<string, unknown> = {};
      if (payload.target_agent_id) {
        msgMetadata.target_agent_id = payload.target_agent_id;
      }
      if (
        Array.isArray(payload.target_agent_ids) &&
        payload.target_agent_ids.length > 0
      ) {
        msgMetadata.target_agent_ids = payload.target_agent_ids;
      }
      if (
        Array.isArray(payload.auto_routed_to) &&
        payload.auto_routed_to.length > 0
      ) {
        msgMetadata.auto_routed_to = payload.auto_routed_to;
      }
      if (payload.metadata && typeof payload.metadata === "object") {
        Object.assign(msgMetadata, payload.metadata);
      }

      setMessages((prev) => [
        ...prev,
        {
          id: syntheticMessageIDRef.current--,
          thread_id: threadId,
          sender_id: senderID,
          role,
          content,
          metadata:
            Object.keys(msgMetadata).length > 0 ? msgMetadata : undefined,
          created_at: new Date().toISOString(),
        },
      ]);
    };

    const sendThreadSubscription = (
      type: "subscribe_thread" | "unsubscribe_thread",
    ) => {
      try {
        wsClient.send({
          type,
          data: { thread_id: threadId },
        });
      } catch {
        // Ignore send errors here.
      }
    };

    const unsubscribeThreadMessage = wsClient.subscribe<ThreadEventPayload>(
      "thread.message",
      (payload) => {
        if (payload.thread_id !== threadId) return;
        appendRealtimeMessage(payload, "human");
        const proposalID = payload.metadata?.proposal_id;
        const metadataType =
          typeof payload.metadata?.type === "string"
            ? payload.metadata.type
            : "";
        if (
          typeof proposalID === "number" ||
          metadataType.startsWith("proposal_")
        ) {
          void refreshProposals();
        }
      },
    );
    const unsubscribeThreadOutput = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_output",
      (payload) => {
        if (payload.thread_id !== threadId) return;
        const agentID = payload.profile_id?.trim() || payload.sender_id?.trim();
        const updateType =
          typeof payload.type === "string" ? payload.type.trim() : "";
        const content =
          typeof payload.content === "string" ? payload.content : "";

        if (agentID && updateType) {
          if (
            updateType === "agent_message_chunk" ||
            updateType === "agent_thought_chunk"
          ) {
            const field =
              updateType === "agent_message_chunk" ? "message" : "thought";
            const existing = pendingAgentChunkBuffersRef.current[agentID] ?? {};
            pendingAgentChunkBuffersRef.current[agentID] = {
              ...existing,
              [field]: `${existing[field] ?? ""}${content}`,
            };
            scheduleAgentChunkFlush();
            return;
          }

          flushAgentChunkBuffers();
          if (updateType === "agent_message") {
            clearLiveAgentOutputField(agentID, "message");
          }
          if (updateType === "agent_thought") {
            clearLiveAgentOutputField(agentID, "thought");
          }
          startTransition(() => {
            setAgentActivitiesByID((prev) => ({
              ...prev,
              [agentID]: applyActivityPayload(
                prev[agentID] ?? [],
                `thread-${threadId}-${agentID}`,
                {
                  ...payload,
                  session_id: `thread-${threadId}-${agentID}`,
                },
                new Date().toISOString(),
                t,
              ),
            }));
          });
          return;
        }

        if (agentID) {
          flushAgentChunkBuffers();
          setThinkingAgentIDs((prev) => {
            if (!prev.has(agentID)) return prev;
            const next = new Set(prev);
            next.delete(agentID);
            return next;
          });
          clearLiveAgentOutput(agentID);
          setCollapsedAgentActivityPanels((prev) => ({
            ...prev,
            [agentID]: true,
          }));
        }
        appendRealtimeMessage(payload, "agent");
      },
    );
    const unsubscribeThreadAck = wsClient.subscribe<ThreadAckPayload>(
      "thread.ack",
      (payload) => {
        if (payload.thread_id !== threadId) return;
        if (
          pendingThreadRequestIdRef.current &&
          payload.request_id &&
          payload.request_id !== pendingThreadRequestIdRef.current
        ) {
          return;
        }
        pendingThreadRequestIdRef.current = null;
        setSending(false);
        clearComposerState();
      },
    );
    const unsubscribeThreadError = wsClient.subscribe<{
      request_id?: string;
      error?: string;
    }>("thread.error", (payload) => {
      if (
        pendingThreadRequestIdRef.current &&
        payload.request_id &&
        payload.request_id !== pendingThreadRequestIdRef.current
      ) {
        return;
      }
      pendingThreadRequestIdRef.current = null;
      setSending(false);
      clearComposerState();
      onError(
        payload.error?.trim() ||
          t("threads.sendFailed", "Thread message failed to send"),
      );
    });
    const refreshSessions = () => {
      void refreshAgentSessions().catch(() => {
        // Ignore background refresh failures.
      });
    };
    const unsubscribeThreadAgentEvent = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_joined",
      (payload) => {
        if (payload.thread_id === threadId) refreshSessions();
      },
    );
    const unsubscribeThreadAgentLeft = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_left",
      (payload) => {
        if (payload.thread_id === threadId) refreshSessions();
      },
    );
    const unsubscribeThreadAgentBooted = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_booted",
      (payload) => {
        if (payload.thread_id === threadId) refreshSessions();
      },
    );
    const unsubscribeThreadAgentFailed = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_failed",
      (payload) => {
        if (payload.thread_id !== threadId) return;
        const failedID = payload.profile_id?.trim();
        if (failedID) {
          flushAgentChunkBuffers();
          setThinkingAgentIDs((prev) => {
            if (!prev.has(failedID)) return prev;
            const next = new Set(prev);
            next.delete(failedID);
            return next;
          });
          clearLiveAgentOutput(failedID);
          setCollapsedAgentActivityPanels((prev) => ({
            ...prev,
            [failedID]: true,
          }));
        }
        onError(
          payload.error?.trim() ||
            t("threads.agentFailed", "An agent in this thread failed."),
        );
        refreshSessions();
      },
    );
    const unsubscribeThreadAgentThinking = wsClient.subscribe<ThreadEventPayload>(
      "thread.agent_thinking",
      (payload) => {
        if (payload.thread_id !== threadId) return;
        const thinkingID = payload.profile_id?.trim();
        if (thinkingID) {
          pendingAgentChunkBuffersRef.current[thinkingID] = {};
          clearAgentActivityState(thinkingID);
          setCollapsedAgentActivityPanels((prev) => ({
            ...prev,
            [thinkingID]: false,
          }));
          setThinkingAgentIDs((prev) => {
            if (prev.has(thinkingID)) return prev;
            const next = new Set(prev);
            next.add(thinkingID);
            return next;
          });
        }
      },
    );
    const unsubscribeStatus = wsClient.onStatusChange((status) => {
      if (status === "open") sendThreadSubscription("subscribe_thread");
    });

    if (wsClient.getStatus() === "open") {
      sendThreadSubscription("subscribe_thread");
    }

    return () => {
      unsubscribeThreadMessage();
      unsubscribeThreadOutput();
      unsubscribeThreadAck();
      unsubscribeThreadError();
      unsubscribeThreadAgentEvent();
      unsubscribeThreadAgentLeft();
      unsubscribeThreadAgentBooted();
      unsubscribeThreadAgentFailed();
      unsubscribeThreadAgentThinking();
      unsubscribeStatus();
      pendingThreadRequestIdRef.current = null;
      pendingAgentChunkBuffersRef.current = {};
      if (agentChunkFlushFrameRef.current != null) {
        cancelAnimationFrame(agentChunkFlushFrameRef.current);
        agentChunkFlushFrameRef.current = null;
      }
      if (wsClient.getStatus() === "open") {
        sendThreadSubscription("unsubscribe_thread");
      }
    };
  }, [
    clearComposerState,
    onError,
    refreshAgentSessions,
    refreshProposals,
    setMessages,
    setSending,
    t,
    threadId,
    wsClient,
  ]);

  const visibleAgentActivityIDs = useMemo(
    () =>
      [
        ...new Set([
          ...Object.keys(liveAgentOutputsByID),
          ...Object.keys(agentActivitiesByID),
          ...thinkingAgentIDs,
        ]),
      ]
        .filter((profileID) => {
          const live = liveAgentOutputsByID[profileID];
          const hasLive = Boolean(live?.thought?.trim() || live?.message?.trim());
          const hasActivities = (agentActivitiesByID[profileID] ?? []).length > 0;
          return hasLive || hasActivities || thinkingAgentIDs.has(profileID);
        })
        .sort((left, right) => {
          const leftTime =
            liveAgentOutputsByID[left]?.updatedAt ??
            agentActivitiesByID[left]?.at(-1)?.at ??
            "";
          const rightTime =
            liveAgentOutputsByID[right]?.updatedAt ??
            agentActivitiesByID[right]?.at(-1)?.at ??
            "";
          return new Date(rightTime).getTime() - new Date(leftTime).getTime();
        }),
    [agentActivitiesByID, liveAgentOutputsByID, thinkingAgentIDs],
  );

  const toggleAgentActivityPanel = useCallback((profileID: string) => {
    setCollapsedAgentActivityPanels((prev) => ({
      ...prev,
      [profileID]: !prev[profileID],
    }));
  }, []);

  return {
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
  };
}
