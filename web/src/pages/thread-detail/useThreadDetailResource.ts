import { useCallback, useEffect, useState } from "react";
import type { ApiClient } from "@/lib/apiClient";
import { getErrorMessage } from "@/lib/v2Workbench";
import type {
  AgentProfile,
  Thread,
  ThreadAttachment,
  ThreadMember,
  ThreadMessage,
  ThreadWorkItemLink,
  WorkItem,
} from "@/types/apiV2";

interface UseThreadDetailResourceOptions {
  apiClient: ApiClient;
  threadId: number;
  onError: (message: string | null) => void;
}

export function useThreadDetailResource({
  apiClient,
  threadId,
  onError,
}: UseThreadDetailResourceOptions) {
  const [thread, setThread] = useState<Thread | null>(null);
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const [participants, setParticipants] = useState<ThreadMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [workItemLinks, setWorkItemLinks] = useState<ThreadWorkItemLink[]>([]);
  const [linkedWorkItems, setLinkedWorkItems] = useState<Record<number, WorkItem>>({});
  const [agentSessions, setAgentSessions] = useState<ThreadMember[]>([]);
  const [attachments, setAttachments] = useState<ThreadAttachment[]>([]);
  const [availableProfiles, setAvailableProfiles] = useState<AgentProfile[]>([]);

  useEffect(() => {
    setThread(null);
    setMessages([]);
    setParticipants([]);
    setWorkItemLinks([]);
    setLinkedWorkItems({});
    setAgentSessions([]);
    setAttachments([]);
    setAvailableProfiles([]);
    setLoading(true);
    onError(null);
  }, [threadId, onError]);

  useEffect(() => {
    if (!threadId || Number.isNaN(threadId)) {
      return;
    }

    let cancelled = false;
    const load = async () => {
      setLoading(true);
      onError(null);
      try {
        const [nextThread, nextMessages, nextParticipants, nextLinks, nextAgents, nextProfiles, nextAttachments] =
          await Promise.all([
            apiClient.getThread(threadId),
            apiClient.listThreadMessages(threadId, { limit: 100 }),
            apiClient.listThreadParticipants(threadId),
            apiClient.listWorkItemsByThread(threadId),
            apiClient.listThreadAgents(threadId),
            apiClient.listProfiles(),
            apiClient.listThreadAttachments(threadId),
          ]);
        if (cancelled) {
          return;
        }

        setThread(nextThread);
        setMessages(nextMessages);
        setParticipants(nextParticipants);
        setWorkItemLinks(nextLinks);
        setAgentSessions(nextAgents);
        setAvailableProfiles(nextProfiles);
        setAttachments(nextAttachments);

        const workItemMap: Record<number, WorkItem> = {};
        const results = await Promise.allSettled(
          nextLinks.map((link) => apiClient.getWorkItem(link.work_item_id)),
        );
        results.forEach((result, index) => {
          if (result.status === "fulfilled") {
            workItemMap[nextLinks[index].work_item_id] = result.value;
          }
        });
        if (!cancelled) {
          setLinkedWorkItems(workItemMap);
        }
      } catch (error) {
        if (!cancelled) {
          onError(getErrorMessage(error));
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [apiClient, threadId, onError]);

  const refreshAgentSessions = useCallback(async () => {
    if (!threadId || Number.isNaN(threadId)) {
      return;
    }
    const sessions = await apiClient.listThreadAgents(threadId);
    setAgentSessions(sessions);
  }, [apiClient, threadId]);

  return {
    thread,
    setThread,
    messages,
    setMessages,
    participants,
    setParticipants,
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
    setAvailableProfiles,
    refreshAgentSessions,
  };
}
