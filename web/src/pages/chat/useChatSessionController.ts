import { useCallback, useEffect, useMemo, useState } from "react";
import type { TFunction } from "i18next";
import type {
  AgentProfile,
  ChatSessionDetail,
  ConfigOption,
  DriverConfig,
  Event as ApiEvent,
  SessionModeState,
  SlashCommand,
} from "@/types/apiV2";
import type { LLMConfigItem } from "@/types/system";
import type { ApiClient } from "@/lib/apiClient";
import { getErrorMessage } from "@/lib/v2Workbench";
import type {
  ChatActivityView,
  ChatMessageView,
  LeadDriverOption,
  SessionGroup,
  SessionRecord,
} from "@/components/chat/chatTypes";
import {
  buildActivityHistory,
  driverLabelForId,
  fallbackLabel,
  formatUsagePercent,
  normalizeDriverKey,
  toDetailRecord,
  toMessageView,
  toProjectGroupKey,
  toSummaryRecord,
} from "@/components/chat/chatUtils";

interface UseChatSessionControllerOptions {
  apiClient: ApiClient;
  projects: Array<{ id: number; name: string }>;
  selectedProjectId: number | null;
  onError: (message: string | null) => void;
  t: TFunction;
}

export function useChatSessionController({
  apiClient,
  projects,
  selectedProjectId,
  onError,
  t,
}: UseChatSessionControllerOptions) {
  const [sessions, setSessions] = useState<SessionRecord[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messagesBySession, setMessagesBySession] = useState<Record<string, ChatMessageView[]>>({});
  const [eventsBySession, setEventsBySession] = useState<Record<string, ApiEvent[]>>({});
  const [activitiesBySession, setActivitiesBySession] = useState<Record<string, ChatActivityView[]>>({});
  const [draftMessages, setDraftMessages] = useState<ChatMessageView[]>([]);
  const [loadedSessions, setLoadedSessions] = useState<Record<string, boolean>>({});
  const [sessionSearch, setSessionSearch] = useState("");
  const [drivers, setDrivers] = useState<DriverConfig[]>([]);
  const [leadProfiles, setLeadProfiles] = useState<AgentProfile[]>([]);
  const [draftProjectId, setDraftProjectId] = useState<number | null>(selectedProjectId);
  const [draftProfileId, setDraftProfileId] = useState("");
  const [draftDriverId, setDraftDriverId] = useState("");
  const [draftLLMConfigId, setDraftLLMConfigId] = useState("system");
  const [llmConfigs, setLLMConfigs] = useState<LLMConfigItem[]>([]);
  const [draftUseWorktree, setDraftUseWorktree] = useState(true);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);
  const [commandsBySession, setCommandsBySession] = useState<Record<string, SlashCommand[]>>({});
  const [configOptionsBySession, setConfigOptionsBySession] = useState<Record<string, ConfigOption[]>>({});
  const [modesBySession, setModesBySession] = useState<Record<string, SessionModeState | null>>({});

  const syncSessionDetail = useCallback((detail: ChatSessionDetail) => {
    const record = toDetailRecord(detail, t);
    const userViews = detail.messages
      .filter((message) => message.role === "user")
      .map((message, index) => toMessageView(detail.session_id, message, index));

    setSessions((current) => {
      const existing = current.filter((item) => item.session_id !== detail.session_id);
      return [record, ...existing].sort((left, right) => (
        new Date(right.created_at).getTime() - new Date(left.created_at).getTime()
      ));
    });
    setMessagesBySession((current) => {
      const existing = current[detail.session_id];
      if (existing && existing.length > 0) {
        return current;
      }
      return { ...current, [detail.session_id]: userViews };
    });
    setLoadedSessions((current) => ({
      ...current,
      [detail.session_id]: true,
    }));
    setCommandsBySession((current) => ({ ...current, [detail.session_id]: detail.available_commands ?? [] }));
    setConfigOptionsBySession((current) => ({ ...current, [detail.session_id]: detail.config_options ?? [] }));
    setModesBySession((current) => ({ ...current, [detail.session_id]: detail.modes ?? null }));
  }, [t]);

  const syncSessionEvents = useCallback((sessionId: string, events: ApiEvent[]) => {
    setEventsBySession((current) => {
      const latestLoadedAt = events.reduce((maxTime, event) => (
        Math.max(maxTime, new Date(event.timestamp).getTime())
      ), 0);
      const pendingRealtime = (current[sessionId] ?? []).filter((event) => (
        event.id < 0 && new Date(event.timestamp).getTime() > latestLoadedAt
      ));
      const merged = [...events, ...pendingRealtime].sort((left, right) => (
        new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime()
      ));
      setActivitiesBySession((activityCurrent) => ({
        ...activityCurrent,
        [sessionId]: buildActivityHistory(sessionId, merged, t),
      }));
      return {
        ...current,
        [sessionId]: merged,
      };
    });
  }, [t]);

  const loadSessionState = useCallback(async (sessionId: string) => {
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
    syncSessionEvents(sessionId, events);
  }, [apiClient, syncSessionDetail, syncSessionEvents]);

  const loadAgentCatalog = useCallback(async () => {
    try {
      const [profiles, driverList, llmConfigResp] = await Promise.all([
        apiClient.listProfiles(),
        apiClient.listDrivers(),
        apiClient.getLLMConfig(),
      ]);
      setDrivers(driverList);
      setLeadProfiles(profiles);
      setLLMConfigs(llmConfigResp.configs ?? []);
      setDraftProfileId((current) => {
        if (current && profiles.some((profile) => profile.id === current)) {
          return current;
        }
        return profiles[0]?.id ?? "";
      });
      setDraftDriverId((current) => {
        if (current && driverList.some((driver) => driver.id === current)) {
          return current;
        }
        const firstProfile = profiles[0];
        if (firstProfile?.driver_id && driverList.some((driver) => driver.id === firstProfile.driver_id)) {
          return firstProfile.driver_id;
        }
        return driverList[0]?.id ?? "";
      });
      setDraftLLMConfigId((current) => {
        if (current && current !== "system") {
          return current;
        }
        return profiles[0]?.llm_config_id || "system";
      });
    } catch (loadError) {
      onError(getErrorMessage(loadError));
    }
  }, [apiClient, onError]);

  const refreshSessions = useCallback(async (preferredSessionId?: string | null) => {
    setLoadingSessions(true);
    try {
      const list = await apiClient.listChatSessions();
      const next = list.map((session) => toSummaryRecord(session, t));
      setSessions(next.sort((left, right) => (
        new Date(right.created_at).getTime() - new Date(left.created_at).getTime()
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
      onError(getErrorMessage(loadError));
    } finally {
      setLoadingSessions(false);
      setInitialLoaded(true);
    }
  }, [apiClient, onError, t]);

  useEffect(() => {
    void refreshSessions();
  }, [refreshSessions]);

  useEffect(() => {
    void loadAgentCatalog();
  }, [loadAgentCatalog]);

  useEffect(() => {
    if (!activeSession || loadedSessions[activeSession]) {
      return;
    }
    let cancelled = false;
    void (async () => {
      try {
        if (!cancelled) {
          await loadSessionState(activeSession);
        }
      } catch (loadError) {
        if (!cancelled) {
          onError(getErrorMessage(loadError));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeSession, loadedSessions, loadSessionState, onError]);

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
  const leadDriverOptions = useMemo<LeadDriverOption[]>(() => {
    const grouped = new Map<string, LeadDriverOption>();
    for (const driver of drivers) {
      const key = normalizeDriverKey(driver.id);
      if (!key) {
        continue;
      }
      if (!grouped.has(key)) {
        grouped.set(key, {
          key,
          label: driverLabelForId(driver.id, t),
          driverId: driver.id,
        });
      }
    }
    const rank = (key: string): number => {
      if (key === "codex") return 0;
      if (key === "claude") return 1;
      return 9;
    };
    return Array.from(grouped.values()).sort((left, right) => {
      const rankDiff = rank(left.key) - rank(right.key);
      if (rankDiff !== 0) {
        return rankDiff;
      }
      return left.label.localeCompare(right.label, "zh-CN");
    });
  }, [drivers, t]);
  const leadDriverMap = useMemo(
    () => new Map(leadDriverOptions.map((option) => [option.driverId, option])),
    [leadDriverOptions],
  );

  const currentProjectId = currentSession?.project_id ?? draftProjectId ?? null;
  const currentProjectLabel = fallbackLabel(
    currentSession?.project_name ?? (currentProjectId != null ? projectNameMap.get(currentProjectId) : undefined),
    t("chat.noProject"),
  );
  const currentDriverId = currentSession?.driver_id ?? draftDriverId;
  const draftSessionReady = Boolean(draftProfileId && draftDriverId);
  const currentDriverLabel = currentDriverId
    ? leadDriverMap.get(currentDriverId)?.label ?? driverLabelForId(currentDriverId, t)
    : t("chat.noDriver");

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
          session.driver_id ? driverLabelForId(session.driver_id, t).toLowerCase() : "",
        ].some((value) => (value ?? "").toLowerCase().includes(query));
      }),
    [sessionSearch, sessions, t],
  );

  const groupedSessions = useMemo<SessionGroup[]>(() => {
    const groups = new Map<string, SessionGroup>();
    for (const session of filteredSessions) {
      const key = toProjectGroupKey(session.project_id);
      const existing = groups.get(key);
      if (existing) {
        existing.sessions.push(session);
        if (new Date(session.created_at).getTime() > new Date(existing.updatedAt).getTime()) {
          existing.updatedAt = session.created_at;
        }
        continue;
      }
      groups.set(key, {
        key,
        label: fallbackLabel(session.project_name, t("chat.noProject")),
        updatedAt: session.created_at,
        sessions: [session],
      });
    }
    return Array.from(groups.values())
      .map((group) => ({
        ...group,
        sessions: [...group.sessions].sort((left, right) => (
          new Date(right.created_at).getTime() - new Date(left.created_at).getTime()
        )),
      }))
      .sort((left, right) => {
        const timeDiff = new Date(right.updatedAt).getTime() - new Date(left.updatedAt).getTime();
        if (timeDiff !== 0) {
          return timeDiff;
        }
        return left.label.localeCompare(right.label, "zh-CN");
      });
  }, [filteredSessions, t]);

  const currentMessages = useMemo(
    () => (currentSession ? (messagesBySession[currentSession.session_id] ?? []) : draftMessages),
    [currentSession, draftMessages, messagesBySession],
  );
  const currentEvents = useMemo(
    () => (currentSession ? (eventsBySession[currentSession.session_id] ?? []) : []),
    [currentSession, eventsBySession],
  );
  const currentActivities = useMemo(
    () => (currentSession ? (activitiesBySession[currentSession.session_id] ?? []) : []),
    [activitiesBySession, currentSession],
  );
  const availableCommands = useMemo(
    () => (currentSession ? (commandsBySession[currentSession.session_id] ?? []) : []),
    [commandsBySession, currentSession],
  );
  const configOptions = useMemo(
    () => (currentSession ? (configOptionsBySession[currentSession.session_id] ?? []) : []),
    [configOptionsBySession, currentSession],
  );
  const sessionModes = useMemo(
    () => (currentSession ? (modesBySession[currentSession.session_id] ?? null) : null),
    [currentSession, modesBySession],
  );
  const isDraftSessionView = initialLoaded && !currentSession && currentMessages.length === 0;
  const currentUsage = useMemo(
    () => [...currentActivities].reverse().find((activity) => activity.type === "usage_update"),
    [currentActivities],
  );
  const currentUsagePercent = formatUsagePercent(currentUsage?.usageUsed, currentUsage?.usageSize);
  const lastActivityText = useMemo(() => {
    const last = [...currentActivities].reverse().find(
      (activity) => activity.type === "agent_thought" || activity.type === "tool_call",
    );
    return last ? (last.detail || last.title) : "";
  }, [currentActivities]);
  const lastUserMessage = useMemo(() => {
    const last = [...currentMessages].reverse().find((message) => message.role === "user");
    return last?.content.replace(/\s+/g, " ").trim() ?? "";
  }, [currentMessages]);

  const handleGroupToggle = useCallback((key: string) => {
    setCollapsedGroups((current) => ({ ...current, [key]: !current[key] }));
  }, []);

  const archiveSession = useCallback(async (sessionId: string) => {
    try {
      await apiClient.archiveChatSession(sessionId, true);
      setSessions((current) => current.filter((session) => session.session_id !== sessionId));
      if (activeSession === sessionId) {
        setActiveSession(null);
      }
    } catch {
      // Ignore and let the next refresh reconcile.
    }
  }, [activeSession, apiClient]);

  return {
    sessions,
    setSessions,
    activeSession,
    setActiveSession,
    messagesBySession,
    setMessagesBySession,
    eventsBySession,
    setEventsBySession,
    activitiesBySession,
    setActivitiesBySession,
    draftMessages,
    setDraftMessages,
    loadedSessions,
    setLoadedSessions,
    sessionSearch,
    setSessionSearch,
    drivers,
    leadProfiles,
    draftProjectId,
    setDraftProjectId,
    draftProfileId,
    setDraftProfileId,
    draftDriverId,
    setDraftDriverId,
    draftLLMConfigId,
    setDraftLLMConfigId,
    llmConfigs,
    draftUseWorktree,
    setDraftUseWorktree,
    collapsedGroups,
    loadingSessions,
    initialLoaded,
    commandsBySession,
    setCommandsBySession,
    configOptionsBySession,
    setConfigOptionsBySession,
    modesBySession,
    setModesBySession,
    currentSession,
    projectNameMap,
    leadDriverOptions,
    currentProjectLabel,
    draftSessionReady,
    currentDriverLabel,
    groupedSessions,
    currentMessages,
    currentEvents,
    currentActivities,
    availableCommands,
    configOptions,
    sessionModes,
    isDraftSessionView,
    currentUsage,
    currentUsagePercent,
    lastActivityText,
    lastUserMessage,
    loadAgentCatalog,
    refreshSessions,
    handleGroupToggle,
    archiveSession,
  };
}
