import { startTransition, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  Bot,
  Brain,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Gauge,
  Loader2,
  MoreHorizontal,
  Plus,
  Search,
  Send,
  User,
  Wrench,
} from "lucide-react";
import type {
  AgentProfile,
  ChatMessage,
  ChatSessionDetail,
  ChatSessionSummary,
  Event as ApiEvent,
} from "@/types/apiV2";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { useWorkbench } from "@/contexts/WorkbenchContext";
import { cn } from "@/lib/utils";
import { getErrorMessage } from "@/lib/v2Workbench";

type SessionRecord = ChatSessionSummary;

interface ChatMessageView {
  id: string;
  role: "user" | "assistant";
  content: string;
  time: string;
  at: string;
}

interface RealtimeChatOutputPayload {
  session_id?: string;
  type?: string;
  content?: string;
  tool_call_id?: string;
  stderr?: string;
  exit_code?: number;
  usage_size?: number;
  usage_used?: number;
}

interface RealtimeChatAckPayload {
  request_id?: string;
  session_id?: string;
  ws_path?: string;
  status?: string;
}

interface RealtimeChatErrorPayload {
  request_id?: string;
  session_id?: string;
  error?: string;
  code?: string;
}

interface ChatActivityView {
  id: string;
  type: "agent_thought" | "tool_call" | "usage_update";
  title: string;
  detail?: string;
  time: string;
  at: string;
  status?: "running" | "completed" | "failed";
  toolCallId?: string;
  usageSize?: number;
  usageUsed?: number;
}

type ChatTimelineItem =
  | {
      kind: "message";
      id: string;
      at: string;
      message: ChatMessageView;
    }
  | {
      kind: "activity";
      id: string;
      at: string;
      activity: ChatActivityView;
    };

interface SessionGroup {
  key: string;
  label: string;
  updatedAt: string;
  sessions: SessionRecord[];
}

const UNKNOWN_PROJECT_GROUP = "project:unknown";
const EMPTY_PROFILE_VALUE = "__empty_profile__";

const formatMessageTime = (value: string): string => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
};

const formatActivityTime = (value: string): string => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

const toMessageView = (sessionId: string, message: ChatMessage, index: number): ChatMessageView => ({
  id: `${sessionId}-${message.role}-${index}-${message.time}`,
  role: message.role === "assistant" ? "assistant" : "user",
  content: message.content,
  time: formatMessageTime(message.time),
  at: message.time,
});

const toSummaryRecord = (session: ChatSessionSummary): SessionRecord => ({
  ...session,
  title: session.title?.trim() || "新会话",
});

const toDetailRecord = (session: ChatSessionDetail): SessionRecord => ({
  ...session,
  title: session.title?.trim() || "新会话",
});

const fallbackLabel = (value: string | null | undefined, fallback: string): string => {
  const trimmed = value?.trim();
  return trimmed ? trimmed : fallback;
};

const toProjectGroupKey = (projectId?: number | null): string => (
  projectId == null ? UNKNOWN_PROJECT_GROUP : `project:${projectId}`
);

const badgeVariantForStatus = (status?: string): "success" | "warning" | "secondary" => {
  switch (status) {
    case "running":
      return "success";
    case "alive":
      return "warning";
    default:
      return "secondary";
  }
};

const badgeLabelForStatus = (status?: string): string => {
  switch (status) {
    case "running":
      return "活跃";
    case "alive":
      return "空闲";
    default:
      return "已关闭";
  }
};

const toStringValue = (value: unknown): string => (
  typeof value === "string" ? value : ""
);

const toNumberValue = (value: unknown): number | undefined => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return undefined;
};

const formatUsageValue = (value?: number): string => {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return "--";
  }
  return value.toLocaleString("zh-CN");
};

const formatUsagePercent = (used?: number, size?: number): number | null => {
  if (
    typeof used !== "number" ||
    typeof size !== "number" ||
    !Number.isFinite(used) ||
    !Number.isFinite(size) ||
    size <= 0
  ) {
    return null;
  }
  return Math.max(0, Math.min(100, (used / size) * 100));
};

const buildToolResultDetail = (payload: RealtimeChatOutputPayload): string => {
  const parts: string[] = [];
  if (typeof payload.exit_code === "number") {
    parts.push(`退出码：${payload.exit_code}`);
  }
  if (payload.content?.trim()) {
    parts.push(payload.content.trim());
  }
  if (payload.stderr?.trim()) {
    parts.push(`stderr\n${payload.stderr.trim()}`);
  }
  return parts.join("\n\n");
};

const touchSessionList = (
  sessions: SessionRecord[],
  sessionId: string,
  status: "running" | "alive" | "closed",
  at: string,
): SessionRecord[] => (
  sessions.map((session) =>
    session.session_id === sessionId
      ? {
          ...session,
          status,
          updated_at: at,
        }
      : session,
  )
);

const applyActivityPayload = (
  current: ChatActivityView[],
  sessionId: string,
  payload: RealtimeChatOutputPayload,
  at: string,
): ChatActivityView[] => {
  const updateType = payload.type?.trim();
  if (!updateType) {
    return current;
  }

  const next = [...current];
  const time = formatActivityTime(at);

  if (updateType === "agent_thought") {
    const detail = payload.content?.trim();
    if (!detail) {
      return current;
    }
    const previous = next.at(-1);
    if (previous?.type === "agent_thought") {
      next[next.length - 1] = {
        ...previous,
        detail: previous.detail ? `${previous.detail}\n${detail}` : detail,
        time,
        at,
      };
      return next;
    }
    next.push({
      id: `${sessionId}-thought-${Date.parse(at)}-${next.length}`,
      type: "agent_thought",
      title: "思考中",
      detail,
      time,
      at,
    });
    return next;
  }

  if (updateType === "tool_call") {
    const toolCallId = payload.tool_call_id?.trim();
    const existingIndex = toolCallId
      ? next.findIndex((activity) => activity.toolCallId === toolCallId)
      : -1;
    const previous = existingIndex >= 0 ? next[existingIndex] : null;
    const activity: ChatActivityView = {
      id: previous?.id ?? `${sessionId}-tool-${toolCallId ?? `${Date.parse(at)}-${next.length}`}`,
      type: "tool_call",
      title: payload.content?.trim() || previous?.title || "工具调用",
      detail: previous?.detail || "执行中...",
      time,
      at,
      status: "running",
      toolCallId,
    };
    if (existingIndex >= 0) {
      next[existingIndex] = activity;
      return next;
    }
    next.push(activity);
    return next;
  }

  if (updateType === "tool_call_completed") {
    const toolCallId = payload.tool_call_id?.trim();
    const existingIndex = toolCallId
      ? next.findIndex((activity) => activity.toolCallId === toolCallId)
      : -1;
    const previous = existingIndex >= 0 ? next[existingIndex] : null;
    const status = payload.exit_code && payload.exit_code !== 0 ? "failed" : "completed";
    const detail = buildToolResultDetail(payload) || previous?.detail || "已完成";
    const activity: ChatActivityView = {
      id: previous?.id ?? `${sessionId}-tool-${toolCallId ?? `${Date.parse(at)}-${next.length}`}`,
      type: "tool_call",
      title: previous?.title || payload.content?.trim() || "工具调用",
      detail,
      time,
      at,
      status,
      toolCallId,
    };
    if (existingIndex >= 0) {
      next[existingIndex] = activity;
      return next;
    }
    next.push(activity);
    return next;
  }

  if (updateType === "usage_update") {
    const usageSize = payload.usage_size;
    const usageUsed = payload.usage_used;
    if (typeof usageSize !== "number" || typeof usageUsed !== "number") {
      return current;
    }
    const activity: ChatActivityView = {
      id: `${sessionId}-usage`,
      type: "usage_update",
      title: "上下文用量",
      detail: `已用 ${formatUsageValue(usageUsed)} / ${formatUsageValue(usageSize)}`,
      time,
      at,
      usageSize,
      usageUsed,
    };
    const existingIndex = next.findIndex((item) => item.id === activity.id);
    if (existingIndex >= 0) {
      next[existingIndex] = activity;
      return next;
    }
    next.push(activity);
    return next;
  }

  return current;
};

const toRealtimePayload = (event: ApiEvent): RealtimeChatOutputPayload => ({
  session_id: toStringValue(event.data?.session_id),
  type: toStringValue(event.data?.type),
  content: toStringValue(event.data?.content),
  tool_call_id: toStringValue(event.data?.tool_call_id),
  stderr: toStringValue(event.data?.stderr),
  exit_code: toNumberValue(event.data?.exit_code),
  usage_size: toNumberValue(event.data?.usage_size),
  usage_used: toNumberValue(event.data?.usage_used),
});

const buildActivityHistory = (
  sessionId: string,
  events: ApiEvent[],
): ChatActivityView[] => {
  const sorted = [...events].sort((left, right) => (
    new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime()
  ));
  return sorted.reduce<ChatActivityView[]>((activities, event) => {
    if (event.type !== "chat.output") {
      return activities;
    }
    const payload = toRealtimePayload(event);
    if (payload.session_id?.trim() !== sessionId) {
      return activities;
    }
    return applyActivityPayload(activities, sessionId, payload, event.timestamp);
  }, []);
};

function ActivityBlock({ activity }: { activity: ChatActivityView }) {
  const isCollapsible = activity.type === "tool_call";
  const [expanded, setExpanded] = useState(!isCollapsible);
  const usagePercent = formatUsagePercent(activity.usageUsed, activity.usageSize);

  const icon = (() => {
    switch (activity.type) {
      case "agent_thought":
        return <Brain className="h-4 w-4 text-sky-600" />;
      case "tool_call":
        if (activity.status === "failed") {
          return <AlertTriangle className="h-4 w-4 text-rose-600" />;
        }
        if (activity.status === "completed") {
          return <CheckCircle2 className="h-4 w-4 text-emerald-600" />;
        }
        return <Wrench className="h-4 w-4 text-amber-600" />;
      case "usage_update":
        return <Gauge className="h-4 w-4 text-violet-600" />;
      default:
        return <Bot className="h-4 w-4 text-muted-foreground" />;
    }
  })();

  const statusLabel = (() => {
    if (activity.type !== "tool_call") {
      return null;
    }
    if (activity.status === "failed") {
      return "失败";
    }
    if (activity.status === "completed") {
      return "完成";
    }
    return "执行中";
  })();

  return (
    <div className="max-w-[720px] pl-11">
      <div className="rounded-lg border bg-background/80 px-4 py-3 shadow-sm">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 shrink-0">{icon}</div>
          <div className="min-w-0 flex-1 space-y-2">
            <div className="flex items-center gap-2">
              <span className="truncate text-sm font-medium">{activity.title}</span>
              {statusLabel ? (
                <span
                  className={cn(
                    "rounded-full px-2 py-0.5 text-[10px] font-medium",
                    activity.status === "failed"
                      ? "bg-rose-100 text-rose-700"
                      : activity.status === "completed"
                        ? "bg-emerald-100 text-emerald-700"
                        : "bg-amber-100 text-amber-700",
                  )}
                >
                  {statusLabel}
                </span>
              ) : null}
              <span className="ml-auto shrink-0 text-[10px] text-muted-foreground">{activity.time}</span>
              {isCollapsible && activity.detail ? (
                <button
                  type="button"
                  className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
                  onClick={() => setExpanded((current) => !current)}
                  aria-label={expanded ? "收起" : "展开"}
                >
                  {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                </button>
              ) : null}
            </div>

            {activity.type === "usage_update" ? (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span>{activity.detail}</span>
                  <span>{usagePercent != null ? `${usagePercent.toFixed(1)}%` : "--"}</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-muted">
                  <div
                    className={cn(
                      "h-full rounded-full transition-[width] duration-300",
                      usagePercent != null && usagePercent >= 85
                        ? "bg-rose-500"
                        : usagePercent != null && usagePercent >= 60
                          ? "bg-amber-500"
                          : "bg-emerald-500",
                    )}
                    style={{ width: `${Math.max(usagePercent ?? 0, usagePercent == null ? 0 : 4)}%` }}
                  />
                </div>
              </div>
            ) : null}

            {activity.type !== "usage_update" && (!isCollapsible || expanded) && activity.detail ? (
              <pre className="overflow-x-auto whitespace-pre-wrap break-words rounded-md bg-muted/60 px-3 py-2 text-xs leading-6 text-foreground">
                {activity.detail}
              </pre>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  );
}

export function ChatPage() {
  const {
    apiClient,
    wsClient,
    projects,
    selectedProjectId,
    setSelectedProjectId,
  } = useWorkbench();
  const [sessions, setSessions] = useState<SessionRecord[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messagesBySession, setMessagesBySession] = useState<Record<string, ChatMessageView[]>>({});
  const [activitiesBySession, setActivitiesBySession] = useState<Record<string, ChatActivityView[]>>({});
  const [draftMessages, setDraftMessages] = useState<ChatMessageView[]>([]);
  const [loadedSessions, setLoadedSessions] = useState<Record<string, boolean>>({});
  const [sessionSearch, setSessionSearch] = useState("");
  const [messageInput, setMessageInput] = useState("");
  const [leadProfiles, setLeadProfiles] = useState<AgentProfile[]>([]);
  const [draftProjectId, setDraftProjectId] = useState<number | null>(selectedProjectId);
  const [draftProfileId, setDraftProfileId] = useState("");
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [newSessionDialogOpen, setNewSessionDialogOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const pendingChunkBuffersRef = useRef<Record<string, string>>({});
  const chunkFlushFrameRef = useRef<number | null>(null);
  const pendingRequestIdRef = useRef<string | null>(null);

  const syncSessionDetail = (detail: ChatSessionDetail) => {
    const record = toDetailRecord(detail);
    const views = detail.messages.map((message, index) =>
      toMessageView(detail.session_id, message, index),
    );

    setSessions((current) => {
      const existing = current.filter((item) => item.session_id !== detail.session_id);
      return [record, ...existing].sort((left, right) => (
        new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime()
      ));
    });
    setMessagesBySession((current) => ({
      ...current,
      [detail.session_id]: views,
    }));
    setLoadedSessions((current) => ({
      ...current,
      [detail.session_id]: true,
    }));
  };

  const syncSessionActivities = (sessionId: string, events: ApiEvent[]) => {
    setActivitiesBySession((current) => ({
      ...current,
      [sessionId]: buildActivityHistory(sessionId, events),
    }));
  };

  const loadSessionState = async (sessionId: string) => {
    const [detail, events] = await Promise.all([
      apiClient.getChatSession(sessionId),
      apiClient.listEvents({
        session_id: sessionId,
        types: ["chat.output"],
        limit: 200,
        offset: 0,
      }),
    ]);
    syncSessionDetail(detail);
    syncSessionActivities(sessionId, events);
  };

  const refreshSessions = async (preferredSessionId?: string | null) => {
    setLoadingSessions(true);
    try {
      const list = await apiClient.listChatSessions();
      const next = list.map(toSummaryRecord);
      setSessions(next.sort((left, right) => (
        new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime()
      )));
      setActiveSession((current) => {
        if (preferredSessionId) {
          return preferredSessionId;
        }
        if (current && next.some((item) => item.session_id === current)) {
          return current;
        }
        return next[0]?.session_id ?? null;
      });
    } catch (loadError) {
      setError(getErrorMessage(loadError));
    } finally {
      setLoadingSessions(false);
    }
  };

  useEffect(() => {
    void refreshSessions();
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const profiles = await apiClient.listProfiles();
        if (cancelled) {
          return;
        }
        const leads = profiles.filter((profile) => profile.role === "lead");
        setLeadProfiles(leads);
        setDraftProfileId((current) => {
          if (current && leads.some((profile) => profile.id === current)) {
            return current;
          }
          return leads[0]?.id ?? "";
        });
      } catch (loadError) {
        if (!cancelled) {
          setError(getErrorMessage(loadError));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [apiClient]);

  const flushBufferedChunks = () => {
    if (chunkFlushFrameRef.current != null) {
      cancelAnimationFrame(chunkFlushFrameRef.current);
      chunkFlushFrameRef.current = null;
    }

    const pending = pendingChunkBuffersRef.current;
    const sessionIds = Object.keys(pending);
    if (sessionIds.length === 0) {
      return;
    }
    pendingChunkBuffersRef.current = {};

    const now = new Date();
    const nowISO = now.toISOString();
    const nowTime = now.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });

    startTransition(() => {
      setMessagesBySession((current) => {
        const next = { ...current };
        for (const sessionId of sessionIds) {
          const chunk = pending[sessionId];
          if (!chunk) {
            continue;
          }
          const existing = next[sessionId] ?? [];
          const last = existing.at(-1);
          if (last && last.id === `${sessionId}-stream-assistant`) {
            next[sessionId] = [
              ...existing.slice(0, -1),
              {
                ...last,
                content: `${last.content}${chunk}`,
                time: nowTime,
                at: nowISO,
              },
            ];
            continue;
          }
          next[sessionId] = [
            ...existing,
            {
              id: `${sessionId}-stream-assistant`,
              role: "assistant",
              content: chunk,
              time: nowTime,
              at: nowISO,
            },
          ];
        }
        return next;
      });

      setSessions((current) =>
        current.map((session) =>
          pending[session.session_id]
            ? {
                ...session,
                status: "running",
                updated_at: nowISO,
              }
            : session,
        ),
      );
    });
  };

  const scheduleChunkFlush = () => {
    if (chunkFlushFrameRef.current != null) {
      return;
    }
    chunkFlushFrameRef.current = requestAnimationFrame(() => {
      chunkFlushFrameRef.current = null;
      flushBufferedChunks();
    });
  };

  const currentSession = useMemo(
    () => sessions.find((session) => session.session_id === activeSession) ?? null,
    [activeSession, sessions],
  );

  useEffect(() => {
    if (!currentSession) {
      setDraftProjectId(selectedProjectId);
    }
  }, [currentSession, selectedProjectId]);

  const projectNameMap = useMemo(
    () => new Map(projects.map((project) => [project.id, project.name])),
    [projects],
  );
  const leadProfileMap = useMemo(
    () => new Map(leadProfiles.map((profile) => [profile.id, profile])),
    [leadProfiles],
  );

  const currentProjectId = currentSession?.project_id ?? draftProjectId ?? null;
  const draftProjectLabel = fallbackLabel(
    draftProjectId != null ? projectNameMap.get(draftProjectId) : undefined,
    "未指定项目",
  );
  const currentProjectLabel = fallbackLabel(
    currentSession?.project_name ?? (currentProjectId != null ? projectNameMap.get(currentProjectId) : undefined),
    "未指定项目",
  );
  const currentProfileId = currentSession?.profile_id ?? draftProfileId;
  const draftProfileLabel = fallbackLabel(
    draftProfileId ? leadProfileMap.get(draftProfileId)?.name ?? draftProfileId : undefined,
    "未指定 ACP 实现",
  );
  const currentProfileLabel = fallbackLabel(
    currentSession?.profile_name
      ?? (currentProfileId ? leadProfileMap.get(currentProfileId)?.name ?? currentProfileId : undefined),
    "未指定 ACP 实现",
  );

  const filteredSessions = useMemo(
    () =>
      sessions.filter((session) => {
        const query = sessionSearch.trim().toLowerCase();
        if (!query) {
          return true;
        }
        return [
          session.title,
          session.project_name,
          session.profile_name,
        ].some((value) => (value ?? "").toLowerCase().includes(query));
      }),
    [sessionSearch, sessions],
  );

  const groupedSessions = useMemo<SessionGroup[]>(() => {
    const groups = new Map<string, SessionGroup>();
    for (const session of filteredSessions) {
      const key = toProjectGroupKey(session.project_id);
      const existing = groups.get(key);
      if (existing) {
        existing.sessions.push(session);
        if (new Date(session.updated_at).getTime() > new Date(existing.updatedAt).getTime()) {
          existing.updatedAt = session.updated_at;
        }
        continue;
      }
      groups.set(key, {
        key,
        label: fallbackLabel(session.project_name, "未指定项目"),
        updatedAt: session.updated_at,
        sessions: [session],
      });
    }
    return Array.from(groups.values())
      .map((group) => ({
        ...group,
        sessions: [...group.sessions].sort((left, right) => (
          new Date(right.updated_at).getTime() - new Date(left.updated_at).getTime()
        )),
      }))
      .sort((left, right) => {
        const timeDiff = new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime();
        if (timeDiff !== 0) {
          return timeDiff;
        }
        return left.label.localeCompare(right.label, "zh-CN");
      });
  }, [filteredSessions]);

  const currentMessages = currentSession ? (messagesBySession[currentSession.session_id] ?? []) : draftMessages;
  const currentActivities = currentSession ? (activitiesBySession[currentSession.session_id] ?? []) : [];

  const currentTimeline = useMemo<ChatTimelineItem[]>(() => {
    const messageItems: ChatTimelineItem[] = currentMessages.map((message) => ({
      kind: "message",
      id: message.id,
      at: message.at,
      message,
    }));
    const activityItems: ChatTimelineItem[] = currentActivities.map((activity) => ({
      kind: "activity",
      id: activity.id,
      at: activity.at,
      activity,
    }));
    return [...messageItems, ...activityItems].sort((left, right) => {
      const timeDiff = new Date(left.at).getTime() - new Date(right.at).getTime();
      if (timeDiff !== 0) {
        return timeDiff;
      }
      if (left.kind === right.kind) {
        return left.id.localeCompare(right.id);
      }
      return left.kind === "message" ? -1 : 1;
    });
  }, [currentActivities, currentMessages]);

  const currentUsage = useMemo(
    () =>
      [...currentActivities]
        .reverse()
        .find((activity) => activity.type === "usage_update"),
    [currentActivities],
  );

  const currentUsagePercent = formatUsagePercent(currentUsage?.usageUsed, currentUsage?.usageSize);

  useEffect(() => {
    const isStreaming = currentMessages.at(-1)?.id.endsWith("stream-assistant");
    messagesEndRef.current?.scrollIntoView({ behavior: isStreaming ? "auto" : "smooth" });
  }, [currentMessages, currentTimeline]);

  useEffect(() => {
    const unsubscribeOutput = wsClient.subscribe<RealtimeChatOutputPayload>(
      "chat.output",
      (payload) => {
        const sessionId = payload.session_id?.trim();
        if (!sessionId) {
          return;
        }

        const updateType = payload.type?.trim();
        const now = new Date();
        const nowISO = now.toISOString();
        if (updateType === "agent_message_chunk" && payload.content) {
          pendingChunkBuffersRef.current[sessionId] = `${pendingChunkBuffersRef.current[sessionId] ?? ""}${payload.content}`;
          scheduleChunkFlush();
          return;
        }

        if (updateType === "agent_message" && payload.content) {
          flushBufferedChunks();
          setMessagesBySession((current) => {
            const existing = current[sessionId] ?? [];
            const last = existing.at(-1);
            if (last && last.id === `${sessionId}-stream-assistant`) {
              return {
                ...current,
                [sessionId]: [
                  ...existing.slice(0, -1),
                  {
                    ...last,
                    content: payload.content ?? last.content,
                    time: now.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }),
                    at: nowISO,
                  },
                ],
              };
            }
            return current;
          });
          setSessions((current) => touchSessionList(current, sessionId, "running", nowISO));
          return;
        }

        if (updateType === "done") {
          flushBufferedChunks();
          setSessions((current) => touchSessionList(current, sessionId, "alive", nowISO));
          setSubmitting(false);
          pendingRequestIdRef.current = null;
          return;
        }

        if (updateType === "error") {
          flushBufferedChunks();
          setError(payload.content?.trim() || "会话执行失败");
          setSessions((current) => touchSessionList(current, sessionId, "closed", nowISO));
          setSubmitting(false);
          pendingRequestIdRef.current = null;
          return;
        }

        startTransition(() => {
          setActivitiesBySession((current) => ({
            ...current,
            [sessionId]: applyActivityPayload(
              current[sessionId] ?? [],
              sessionId,
              payload,
              nowISO,
            ),
          }));
          setSessions((current) => touchSessionList(current, sessionId, "running", nowISO));
        });
      },
    );
    const unsubscribeAck = wsClient.subscribe<RealtimeChatAckPayload>(
      "chat.ack",
      (payload) => {
        const requestId = payload.request_id?.trim();
        if (!pendingRequestIdRef.current && requestId) {
          return;
        }
        if (pendingRequestIdRef.current && requestId && pendingRequestIdRef.current !== requestId) {
          return;
        }
        const sessionId = payload.session_id?.trim();
        if (!sessionId) {
          return;
        }
        pendingRequestIdRef.current = null;
        setSubmitting(false);
        setActiveSession(sessionId);
        setDraftMessages([]);
        setLoadedSessions((current) => ({
          ...current,
          [sessionId]: false,
        }));
        void refreshSessions(sessionId);
      },
    );
    const unsubscribeError = wsClient.subscribe<RealtimeChatErrorPayload>(
      "chat.error",
      (payload) => {
        const requestId = payload.request_id?.trim();
        if (!pendingRequestIdRef.current && requestId) {
          return;
        }
        if (pendingRequestIdRef.current && requestId && pendingRequestIdRef.current !== requestId) {
          return;
        }
        pendingRequestIdRef.current = null;
        setSubmitting(false);
        setError(payload.error?.trim() || "发送消息失败");
        const sessionId = payload.session_id?.trim();
        if (sessionId) {
          setSessions((current) => touchSessionList(current, sessionId, "closed", new Date().toISOString()));
        }
      },
    );
    return () => {
      if (chunkFlushFrameRef.current != null) {
        cancelAnimationFrame(chunkFlushFrameRef.current);
        chunkFlushFrameRef.current = null;
      }
      pendingChunkBuffersRef.current = {};
      pendingRequestIdRef.current = null;
      unsubscribeOutput();
      unsubscribeAck();
      unsubscribeError();
    };
  }, [wsClient]);

  useEffect(() => {
    if (!activeSession || loadedSessions[activeSession]) {
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        if (!cancelled) {
          await loadSessionState(activeSession);
        }
      } catch (loadError) {
        if (!cancelled) {
          setError(getErrorMessage(loadError));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeSession, apiClient, loadedSessions]);

  const createSession = () => {
    setDraftProjectId(selectedProjectId);
    setDraftProfileId((current) => {
      if (current && leadProfiles.some((profile) => profile.id === current)) {
        return current;
      }
      return leadProfiles[0]?.id ?? "";
    });
    setNewSessionDialogOpen(true);
  };

  const startDraftSession = () => {
    setSelectedProjectId(draftProjectId);
    setActiveSession(null);
    setDraftMessages([]);
    setMessageInput("");
    setError(null);
    setNewSessionDialogOpen(false);
  };

  const appendMessage = (sessionId: string | null, role: "user" | "assistant", content: string) => {
    const now = new Date();
    const nowISO = now.toISOString();
    const view: ChatMessageView = {
      id: `${sessionId ?? "draft"}-${role}-${now.getTime()}`,
      role,
      content,
      time: now.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" }),
      at: nowISO,
    };

    if (!sessionId) {
      setDraftMessages((current) => [...current, view]);
      return;
    }

    setMessagesBySession((current) => ({
      ...current,
      [sessionId]: [...(current[sessionId] ?? []), view],
    }));
    setSessions((current) =>
      current.map((session) =>
        session.session_id === sessionId
          ? {
              ...session,
              title: session.title === "新会话" && role === "user"
                ? content.slice(0, 24)
                : session.title,
              updated_at: nowISO,
              message_count: session.message_count + 1,
              status: role === "user" ? "running" : "alive",
            }
          : session,
      ),
    );
  };

  const sendMessage = async () => {
    const content = messageInput.trim();
    if (!content || currentSession?.status === "running") {
      return;
    }

    const workingSessionId = activeSession;
    const resolvedProjectId = currentSession?.project_id ?? draftProjectId ?? undefined;
    const resolvedProjectName = currentSession?.project_name
      ?? (resolvedProjectId != null ? projectNameMap.get(resolvedProjectId) : undefined);
    const resolvedProfileId = currentSession?.profile_id ?? draftProfileId;

    if (!resolvedProfileId) {
      setError("请先选择 ACP 实现后再开始会话。");
      setNewSessionDialogOpen(true);
      return;
    }

    appendMessage(workingSessionId, "user", content);
    setMessageInput("");
    setSubmitting(true);
    setError(null);

    try {
      const requestId = `chat-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      pendingRequestIdRef.current = requestId;
      wsClient.send({
        type: "chat.send",
        data: {
          request_id: requestId,
          session_id: workingSessionId ?? undefined,
          message: content,
          project_id: resolvedProjectId,
          project_name: resolvedProjectName,
          profile_id: resolvedProfileId,
        },
      });
    } catch (sendError) {
      pendingRequestIdRef.current = null;
      setError(getErrorMessage(sendError));
      if (workingSessionId) {
        setSessions((current) => touchSessionList(current, workingSessionId, "closed", new Date().toISOString()));
      }
    } finally {
      if (!pendingRequestIdRef.current) {
        setSubmitting(false);
      }
    }
  };

  const closeSession = async () => {
    if (!currentSession) {
      return;
    }
    try {
      await apiClient.closeChat(currentSession.session_id);
      setSessions((current) =>
        current.map((session) =>
          session.session_id === currentSession.session_id
            ? { ...session, status: "closed" }
            : session,
        ),
      );
    } catch (closeError) {
      setError(getErrorMessage(closeError));
    }
  };

  return (
    <div className="flex h-full overflow-hidden">
      <div className="flex w-72 flex-col border-r bg-sidebar">
        <div className="border-b p-3">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold">会话列表</h2>
            <Button variant="ghost" size="icon" className="h-7 w-7" onClick={createSession}>
              <Plus className="h-4 w-4" />
            </Button>
          </div>
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              placeholder="搜索会话..."
              className="h-8 pl-8 text-xs"
              value={sessionSearch}
              onChange={(event) => setSessionSearch(event.target.value)}
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          {groupedSessions.map((group) => (
            <div key={group.key} className="border-b">
              <button
                type="button"
                className="flex w-full items-center gap-2 px-3 py-2 text-left transition-colors hover:bg-muted/50"
                onClick={() =>
                  setCollapsedGroups((current) => ({
                    ...current,
                    [group.key]: !current[group.key],
                  }))
                }
              >
                {collapsedGroups[group.key] ? (
                  <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
                ) : (
                  <ChevronDown className="h-3.5 w-3.5 text-muted-foreground" />
                )}
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
                    {group.label}
                  </div>
                </div>
                <Badge variant="secondary" className="text-[10px]">
                  {group.sessions.length}
                </Badge>
              </button>

              {!collapsedGroups[group.key] ? group.sessions.map((session) => {
                const preview = messagesBySession[session.session_id]?.at(-1)?.content ?? "暂无消息";
                return (
                  <button
                    key={session.session_id}
                    onClick={() => setActiveSession(session.session_id)}
                    className={cn(
                      "w-full border-t px-3 py-3 pl-8 text-left transition-colors",
                      activeSession === session.session_id ? "bg-accent" : "hover:bg-muted/50",
                    )}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="truncate text-sm font-medium">{session.title ?? "新会话"}</span>
                      <span className="shrink-0 text-[10px] text-muted-foreground">
                        {new Date(session.updated_at).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })}
                      </span>
                    </div>
                    <div className="mt-1 flex items-center gap-1.5">
                      <div
                        className={cn(
                          "h-1.5 w-1.5 shrink-0 rounded-full",
                          session.status === "running"
                            ? "bg-emerald-500"
                            : session.status === "alive"
                              ? "bg-amber-500"
                              : "bg-zinc-300",
                        )}
                      />
                      <p className="truncate text-xs text-muted-foreground">{preview}</p>
                    </div>
                    <div className="mt-2 flex flex-wrap items-center gap-1.5">
                      {session.profile_name ? (
                        <Badge variant="outline" className="text-[10px]">
                          ACP · {session.profile_name}
                        </Badge>
                      ) : null}
                    </div>
                  </button>
                );
              }) : null}
            </div>
          ))}
          {!loadingSessions && groupedSessions.length === 0 ? (
            <div className="px-3 py-4 text-xs text-muted-foreground">
              暂无会话。
            </div>
          ) : null}
        </div>
      </div>

      <div className="flex flex-1 flex-col">
        <div className="border-b px-5 py-3">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-primary text-primary-foreground">
                <Bot className="h-4 w-4" />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="truncate text-sm font-semibold">{currentSession?.title ?? "Lead Agent"}</span>
                  <Badge
                    variant={badgeVariantForStatus(currentSession?.status)}
                    className="text-[10px]"
                  >
                    {badgeLabelForStatus(currentSession?.status)}
                  </Badge>
                  {submitting ? <Loader2 className="h-3.5 w-3.5 animate-spin text-muted-foreground" /> : null}
                </div>
                <div className="mt-2 flex flex-wrap items-center gap-2">
                  <Badge variant="secondary" className="text-[10px]">
                    项目 · {currentProjectLabel}
                  </Badge>
                  <Badge variant="secondary" className="text-[10px]">
                    ACP · {currentProfileLabel}
                  </Badge>
                  {currentSession?.project_id != null ? (
                    <span className="text-[10px] text-muted-foreground">project_id={currentSession.project_id}</span>
                  ) : null}
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                  {currentSession ? "当前会话已锁定项目和 ACP 实现。" : "通过“新建会话”切换项目和 ACP 实现。"}
                </p>
                {currentSession?.ws_path ? (
                  <p className="truncate text-[10px] text-muted-foreground">
                    WS：{currentSession.ws_path}
                  </p>
                ) : null}
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => void closeSession()}>
                <MoreHorizontal className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {currentUsage ? (
            <div className="mt-3 rounded-lg border bg-background/70 px-3 py-2">
              <div className="flex items-center justify-between text-[11px] text-muted-foreground">
                <span>上下文进度</span>
                <span>
                  {formatUsageValue(currentUsage.usageUsed)} / {formatUsageValue(currentUsage.usageSize)}
                  {currentUsagePercent != null ? ` · ${currentUsagePercent.toFixed(1)}%` : ""}
                </span>
              </div>
              <div className="mt-2 h-2 overflow-hidden rounded-full bg-muted">
                <div
                  className={cn(
                    "h-full rounded-full transition-[width] duration-300",
                    currentUsagePercent != null && currentUsagePercent >= 85
                      ? "bg-rose-500"
                      : currentUsagePercent != null && currentUsagePercent >= 60
                        ? "bg-amber-500"
                        : "bg-emerald-500",
                  )}
                  style={{ width: `${Math.max(currentUsagePercent ?? 0, currentUsagePercent == null ? 0 : 4)}%` }}
                />
              </div>
            </div>
          ) : null}
        </div>

        {error ? <p className="mx-5 mt-4 rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">{error}</p> : null}

        <div className="flex-1 space-y-4 overflow-y-auto px-5 py-4">
          {currentTimeline.length === 0 ? (
            <div className="rounded-2xl border border-dashed bg-gradient-to-br from-white via-slate-50 to-slate-100 p-6 text-sm text-muted-foreground shadow-sm">
              <div className="max-w-xl space-y-3">
                <p className="text-base font-semibold text-foreground">准备开始新的 ACP 对话</p>
                <p>
                  先确认本次会话绑定的项目和 ACP 实现，开始后会固定在当前会话里。
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="secondary" className="text-[10px]">
                    项目 · {currentProjectLabel}
                  </Badge>
                  <Badge variant="secondary" className="text-[10px]">
                    ACP · {currentProfileLabel}
                  </Badge>
                </div>
                <Button onClick={createSession}>新建会话</Button>
              </div>
            </div>
          ) : (
            currentTimeline.map((item) => {
              if (item.kind === "activity") {
                return <ActivityBlock key={item.id} activity={item.activity} />;
              }

              const message = item.message;
              return (
                <div
                  key={message.id}
                  className={cn(
                    "flex max-w-[720px] gap-3",
                    message.role === "user" ? "ml-auto flex-row-reverse" : "",
                  )}
                >
                  <div
                    className={cn(
                      "flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
                      message.role === "user" ? "bg-zinc-200" : "bg-primary text-primary-foreground",
                    )}
                  >
                    {message.role === "user" ? <User className="h-4 w-4" /> : <Bot className="h-4 w-4" />}
                  </div>
                  <div className={cn("space-y-2", message.role === "user" ? "text-right" : "")}>
                    <div
                      className={cn(
                        "rounded-lg px-4 py-3 text-sm leading-relaxed",
                        message.role === "user" ? "bg-primary text-primary-foreground" : "bg-muted",
                      )}
                    >
                      {message.content.split("\n").map((line, index) => (
                        <span key={`${message.id}-${index}`} className="block">{line}</span>
                      ))}
                    </div>
                    <span className="text-[10px] text-muted-foreground">{message.time}</span>
                  </div>
                </div>
              );
            })
          )}
          <div ref={messagesEndRef} />
        </div>

        <div className="border-t p-4">
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <Badge variant="outline" className="text-[10px]">
              项目 · {currentProjectLabel}
            </Badge>
            <Badge variant="outline" className="text-[10px]">
              ACP · {currentProfileLabel}
            </Badge>
          </div>
          <div className="flex items-end gap-3">
            <div className="relative flex-1">
              <Input
                placeholder={
                  currentSession
                    ? "输入消息，与 Lead Agent 对话..."
                    : `输入消息，使用 ${currentProfileLabel} 在 ${currentProjectLabel} 下开始会话...`
                }
                className="pr-10"
                value={messageInput}
                disabled={submitting || currentSession?.status === "running" || (!currentSession && !draftProfileId)}
                onChange={(event) => setMessageInput(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" && !event.shiftKey) {
                    event.preventDefault();
                    void sendMessage();
                  }
                }}
              />
            </div>
            <Button
              size="icon"
              className="h-10 w-10 shrink-0"
              disabled={submitting || currentSession?.status === "running" || (!currentSession && !draftProfileId)}
              onClick={() => void sendMessage()}
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
          <div className="mt-2 text-[10px] text-muted-foreground">Enter 发送 · Shift+Enter 换行</div>
        </div>
      </div>

      <Dialog open={newSessionDialogOpen} onClose={() => setNewSessionDialogOpen(false)} className="max-w-3xl overflow-hidden border-0 bg-transparent shadow-none">
        <div className="rounded-[28px] border bg-card shadow-2xl">
          <DialogHeader className="border-b bg-gradient-to-br from-slate-950 via-slate-900 to-slate-800 pb-6 text-white">
            <DialogTitle className="text-2xl font-semibold tracking-tight">新建会话</DialogTitle>
            <DialogDescription className="max-w-2xl text-slate-300">
              先固定项目和 ACP 实现，再回到当前聊天框继续输入。会话开始后，这两个维度会跟着 session 一起持久化。
            </DialogDescription>
          </DialogHeader>

          <DialogBody className="bg-[radial-gradient(circle_at_top,_rgba(15,23,42,0.06),_transparent_55%)] pt-6">
            <div className="rounded-[24px] border bg-white p-6 shadow-sm">
              <div className="space-y-3">
                <div className="text-sm font-medium text-slate-950">会话上下文</div>
                <div className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4">
                  <p className="text-[15px] leading-7 text-slate-700">
                    新消息将直接通过 WebSocket 发送到 ACP，并在同一条链路里接收工具事件、思考过程和最终回复。
                  </p>
                </div>
              </div>

              <div className="mt-6 grid gap-4 md:grid-cols-2">
                <div className="space-y-2">
                  <label className="text-xs font-medium uppercase tracking-[0.18em] text-slate-500">项目</label>
                  <Select
                    value={draftProjectId == null ? "" : String(draftProjectId)}
                    onChange={(event) => {
                      const next = event.target.value;
                      setDraftProjectId(next ? Number(next) : null);
                    }}
                  >
                    <option value="">未指定项目</option>
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>{project.name}</option>
                    ))}
                  </Select>
                </div>

                <div className="space-y-2">
                  <label className="text-xs font-medium uppercase tracking-[0.18em] text-slate-500">ACP 实现</label>
                  <Select
                    value={draftProfileId || EMPTY_PROFILE_VALUE}
                    onChange={(event) => {
                      const next = event.target.value;
                      setDraftProfileId(next === EMPTY_PROFILE_VALUE ? "" : next);
                    }}
                  >
                    <option value={EMPTY_PROFILE_VALUE}>请选择 ACP 实现</option>
                    {leadProfiles.map((profile) => (
                      <option key={profile.id} value={profile.id}>
                        {profile.name?.trim() || profile.id}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>

              <div className="mt-6 flex flex-wrap items-center gap-2">
                <Badge variant="secondary" className="text-[10px]">
                  项目 · {draftProjectLabel}
                </Badge>
                <Badge variant="secondary" className="text-[10px]">
                  ACP · {draftProfileLabel}
                </Badge>
              </div>

              {leadProfiles.length === 0 ? (
                <div className="mt-4 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
                  还没有可用的 lead profile，请先到代理页面配置 ACP 实现。
                </div>
              ) : null}
            </div>
          </DialogBody>

          <DialogFooter className="border-t bg-slate-50/80">
            <Button variant="outline" onClick={() => setNewSessionDialogOpen(false)}>取消</Button>
            <Button onClick={startDraftSession} disabled={!draftProfileId}>
              开始新会话
            </Button>
          </DialogFooter>
        </div>
      </Dialog>
    </div>
  );
}
