import { useEffect, useMemo, useRef, useState } from "react";
import type { ApiClient } from "../lib/apiClient";
import type { ChatMessage } from "../types/workflow";

interface ChatViewProps {
  apiClient: ApiClient;
  projectId: string;
}

const roleLabel: Record<ChatMessage["role"], string> = {
  user: "用户",
  assistant: "助手",
};

const roleStyle: Record<ChatMessage["role"], string> = {
  user: "bg-slate-900 text-white",
  assistant: "border border-slate-200 bg-white text-slate-900",
};

const formatTime = (time: string): string => {
  const date = new Date(time);
  if (Number.isNaN(date.getTime())) {
    return time;
  }
  return date.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
};

const getErrorMessage = (error: unknown): string => {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "请求失败，请稍后重试";
};

const ChatView = ({ apiClient, projectId }: ChatViewProps) => {
  const [draft, setDraft] = useState("");
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [chatLoading, setChatLoading] = useState(false);
  const [planLoading, setPlanLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [planNotice, setPlanNotice] = useState<string | null>(null);
  const chatRequestIdRef = useRef(0);
  const planRequestIdRef = useRef(0);

  useEffect(() => {
    chatRequestIdRef.current += 1;
    planRequestIdRef.current += 1;
    setDraft("");
    setSessionId(null);
    setMessages([]);
    setError(null);
    setPlanNotice(null);
    setChatLoading(false);
    setPlanLoading(false);
  }, [projectId]);

  const hasMessages = messages.length > 0;
  const canSubmit = draft.trim().length > 0 && !chatLoading;
  const canCreatePlan = !!sessionId && !planLoading;

  const sortedMessages = useMemo(
    () =>
      [...messages].sort((a, b) => {
        return new Date(a.time).getTime() - new Date(b.time).getTime();
      }),
    [messages],
  );

  const handleStartChat = async () => {
    const message = draft.trim();
    if (!message) {
      return;
    }

    setChatLoading(true);
    setError(null);
    setPlanNotice(null);
    const requestId = chatRequestIdRef.current + 1;
    chatRequestIdRef.current = requestId;
    const targetProjectId = projectId;

    try {
      const created = await apiClient.createChat(targetProjectId, { message });
      if (chatRequestIdRef.current !== requestId) {
        return;
      }
      const session = await apiClient.getChat(targetProjectId, created.session_id);
      if (chatRequestIdRef.current !== requestId) {
        return;
      }
      setSessionId(created.session_id);
      setMessages(session.messages);
      setDraft("");
    } catch (requestError) {
      if (chatRequestIdRef.current !== requestId) {
        return;
      }
      setError(getErrorMessage(requestError));
    } finally {
      if (chatRequestIdRef.current === requestId) {
        setChatLoading(false);
      }
    }
  };

  const handleCreatePlan = async () => {
    if (!sessionId) {
      return;
    }

    setPlanLoading(true);
    setError(null);
    setPlanNotice(null);
    const requestId = planRequestIdRef.current + 1;
    planRequestIdRef.current = requestId;
    const targetProjectId = projectId;
    const targetSessionId = sessionId;
    try {
      const createdPlan = await apiClient.createPlan(targetProjectId, {
        session_id: targetSessionId,
      });
      if (planRequestIdRef.current !== requestId) {
        return;
      }
      setPlanNotice(`已创建计划：${createdPlan.id}`);
    } catch (requestError) {
      if (planRequestIdRef.current !== requestId) {
        return;
      }
      setError(getErrorMessage(requestError));
    } finally {
      if (planRequestIdRef.current === requestId) {
        setPlanLoading(false);
      }
    }
  };

  return (
    <section className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_320px]">
      <div className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h2 className="text-xl font-bold">Chat</h2>
        <p className="mt-1 text-sm text-slate-600">
          发送消息后调用 POST /chat 创建会话，再调用 GET /chat/:sid 获取完整历史。
        </p>

        <div className="mt-4 min-h-72 rounded-lg border border-slate-200 bg-slate-50 p-3">
          {hasMessages ? (
            <div className="flex flex-col gap-3">
              {sortedMessages.map((message, index) => (
                <article
                  key={`${message.time}-${index}`}
                  className={`max-w-[92%] rounded-lg px-3 py-2 text-sm ${
                    roleStyle[message.role]
                  } ${message.role === "user" ? "self-end" : "self-start"}`}
                >
                  <p className="mb-1 text-xs font-semibold opacity-80">
                    {roleLabel[message.role]} · {formatTime(message.time)}
                  </p>
                  <p className="whitespace-pre-wrap">{message.content}</p>
                </article>
              ))}
            </div>
          ) : (
            <p className="text-sm text-slate-500">当前会话暂无消息。</p>
          )}
        </div>

        <div className="mt-4">
          <label htmlFor="chat-message" className="mb-2 block text-sm font-medium">
            新消息
          </label>
          <textarea
            id="chat-message"
            rows={4}
            className="w-full resize-y rounded-lg border border-slate-300 px-3 py-2 text-sm"
            placeholder="请输入要拆分为计划的需求..."
            value={draft}
            onChange={(event) => {
              setDraft(event.target.value);
            }}
          />
          <div className="mt-3 flex justify-end">
            <button
              type="button"
              className="rounded-md bg-slate-900 px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:bg-slate-400"
              disabled={!canSubmit}
              onClick={() => {
                void handleStartChat();
              }}
            >
              {chatLoading ? "处理中..." : "发送并创建会话"}
            </button>
          </div>
        </div>
      </div>

      <aside className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
        <h3 className="text-lg font-semibold">会话与计划</h3>
        <p className="mt-2 break-all text-xs text-slate-600">
          Session ID: {sessionId ?? "未创建"}
        </p>
        <button
          type="button"
          className="mt-3 w-full rounded-md border border-slate-900 px-3 py-2 text-sm font-semibold text-slate-900 disabled:cursor-not-allowed disabled:border-slate-300 disabled:text-slate-400"
          disabled={!canCreatePlan}
          onClick={() => {
            void handleCreatePlan();
          }}
        >
          {planLoading ? "创建计划中..." : "基于当前会话创建计划"}
        </button>

        {planNotice ? (
          <p className="mt-3 rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {planNotice}
          </p>
        ) : null}
        {error ? (
          <p className="mt-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">
            {error}
          </p>
        ) : null}
      </aside>
    </section>
  );
};

export default ChatView;
