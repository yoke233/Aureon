import { useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Brain,
  Check,
  ChevronDown,
  ChevronRight,
  ClipboardCopy,
  ListTodo,
  Loader2,
  Wrench,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { ChatActivityView, ChatFeedEntry } from "./chatTypes";
import { compactText } from "./chatUtils";

interface MessageFeedViewProps {
  entries: ChatFeedEntry[];
  submitting: boolean;
  copiedMessageId: string | null;
  collapsedActivityGroups: Record<string, boolean>;
  onCopyMessage: (id: string, content: string) => void;
  onCreateWorkItem: (id: string, content: string) => void;
  onActivityGroupToggle: (id: string) => void;
}

const TOOL_PREVIEW_LINES = 5;
const TOOL_PREVIEW_CHARS_PER_LINE = 72;

function estimateVisualLineCount(value: string): number {
  const lines = value.split(/\r?\n/);
  return lines.reduce((total, line) => {
    const normalizedLength = Math.max(line.trim().length, 1);
    return total + Math.max(1, Math.ceil(normalizedLength / TOOL_PREVIEW_CHARS_PER_LINE));
  }, 0);
}

function statusBadgeClass(status: ChatActivityView["status"]) {
  switch (status) {
    case "failed":
      return "border-red-200 bg-red-50 text-red-700";
    case "completed":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    case "running":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-border bg-muted/60 text-muted-foreground";
  }
}

function statusLabel(status: ChatActivityView["status"], t: (key: string) => string): string | null {
  switch (status) {
    case "failed":
      return t("status.failed");
    case "completed":
      return t("chat.completed");
    case "running":
      return t("status.running");
    default:
      return null;
  }
}

interface ToolCallCardProps {
  activity: ChatActivityView;
}

function ToolCallCard({ activity }: ToolCallCardProps) {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);

  const detailText = useMemo(() => {
    const detail = activity.detail?.trim();
    if (detail) {
      return detail;
    }
    return activity.title.trim();
  }, [activity.detail, activity.title]);

  const canToggle = useMemo(
    () => estimateVisualLineCount(detailText) > TOOL_PREVIEW_LINES,
    [detailText],
  );
  const badgeText = statusLabel(activity.status, t);

  return (
    <div className="rounded-md border border-amber-200/70 bg-amber-50/50 px-3 py-2 shadow-sm shadow-amber-100/40">
      <div className="flex items-start gap-2">
        <Wrench className="mt-0.5 h-3.5 w-3.5 shrink-0 text-amber-600" />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <span className="text-xs font-semibold text-foreground">{activity.title}</span>
            {badgeText && (
              <span className={cn(
                "rounded-full border px-1.5 py-0.5 text-[10px] font-medium",
                statusBadgeClass(activity.status),
              )}>
                {badgeText}
              </span>
            )}
            <span className="text-[10px] text-muted-foreground">{activity.time}</span>
          </div>
          <div className="relative mt-2">
            <div
              data-testid={`tool-call-detail-${activity.id}`}
              className={cn(
                "whitespace-pre-wrap break-words rounded-sm border-l-2 border-amber-300/70 bg-background/80 px-3 py-2 text-xs leading-6 text-foreground/80",
                !expanded && "line-clamp-5",
              )}
            >
              {detailText}
            </div>
            {!expanded && canToggle && (
              <div className="pointer-events-none absolute inset-x-0 bottom-0 h-12 rounded-b-sm bg-gradient-to-t from-amber-50 via-amber-50/80 to-transparent" />
            )}
          </div>
          {canToggle && (
            <div className={cn("mt-2 flex", expanded ? "sticky bottom-2 z-10 justify-end" : "justify-end")}>
              <button
                type="button"
                className="pointer-events-auto rounded-full border border-amber-200 bg-background/95 px-2.5 py-1 text-[11px] font-medium text-amber-700 shadow-sm backdrop-blur transition-colors hover:bg-amber-50"
                onClick={() => setExpanded((current) => !current)}
              >
                {t(expanded ? "chat.collapse" : "chat.expand")}
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export function MessageFeedView(props: MessageFeedViewProps) {
  const {
    entries,
    submitting,
    copiedMessageId,
    collapsedActivityGroups,
    onCopyMessage,
    onCreateWorkItem,
    onActivityGroupToggle,
  } = props;
  const { t } = useTranslation();

  return (
    <>
      {entries.map((entry) => {
        /* ── thought: italic one-liner ── */
        if (entry.type === "thought") {
          const act = entry.item.data;
          return (
            <div key={act.id} className="flex items-start gap-1.5 py-0.5 text-xs text-violet-500">
              <Brain className="mt-px h-3.5 w-3.5 shrink-0" />
              <span className="min-w-0 italic">{compactText(act.detail || act.title, 200)}</span>
            </div>
          );
        }

        /* ── tool_group: collapsible compact block ── */
        if (entry.type === "tool_group") {
          const isCollapsed = collapsedActivityGroups[entry.id] === true;
          const count = entry.items.length;
          return (
            <div key={entry.id} className="py-1">
              <button
                type="button"
                className="flex items-center gap-1.5 text-xs text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => onActivityGroupToggle(entry.id)}
              >
                {isCollapsed ? <ChevronRight className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
                <Wrench className="h-3 w-3 text-amber-500" />
                <span>{count} {t("chat.toolCalls").toLowerCase()}</span>
              </button>
              {!isCollapsed && (
                <div className="ml-6 mt-2 space-y-2 border-l-2 border-amber-200/80 pl-4">
                  {entry.items.map((item) => {
                    return <ToolCallCard key={item.data.id} activity={item.data} />;
                  })}
                </div>
              )}
            </div>
          );
        }

        /* ── message ── */
        const message = entry.item.data;
        const isUser = message.role === "user";
        return (
          <div
            key={message.id}
            {...(isUser ? { "data-user-msg": "true" } : {})}
            className={cn(
              "group/msg rounded-sm py-1.5",
              isUser ? "bg-blue-50/60" : "",
            )}
          >
            <div className="flex items-start gap-2">
              <span className={cn(
                "shrink-0 select-none text-xs font-bold tracking-wide",
                isUser ? "text-blue-600" : "text-emerald-600",
              )}>
                {isUser ? "❯ You" : "⦿ Agent"}
              </span>
              <span className="shrink-0 text-[10px] text-muted-foreground/50">{message.time}</span>
              {!isUser && (
                <div className="ml-auto flex shrink-0 items-center gap-1.5 opacity-0 transition-opacity group-hover/msg:opacity-100">
                  <button
                    type="button"
                    className={cn(
                      "flex h-6 w-6 items-center justify-center rounded transition-colors",
                      copiedMessageId === message.id ? "text-emerald-600" : "text-muted-foreground hover:text-foreground",
                    )}
                    title={t("chat.copy")}
                    onClick={() => onCopyMessage(message.id, message.content)}
                  >
                    {copiedMessageId === message.id ? <Check className="h-3.5 w-3.5" /> : <ClipboardCopy className="h-3.5 w-3.5" />}
                  </button>
                  <button
                    type="button"
                    className="flex h-6 w-6 items-center justify-center rounded text-muted-foreground transition-colors hover:text-amber-600"
                    title={t("chat.createWorkItem")}
                    onClick={() => onCreateWorkItem(message.id, message.content)}
                  >
                    <ListTodo className="h-3.5 w-3.5" />
                  </button>
                </div>
              )}
            </div>
            <div className={cn(
              "mt-0.5 whitespace-pre-wrap text-sm leading-relaxed",
              isUser ? "border-l-2 border-blue-300 pl-3 text-foreground" : "border-l-2 border-emerald-200 pl-3 text-foreground/90",
            )}>
              {message.content}
            </div>
          </div>
        );
      })}
      {submitting && (
        <div className="flex items-center gap-1.5 py-1 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>{t("chat.thinking")}...</span>
        </div>
      )}
    </>
  );
}
