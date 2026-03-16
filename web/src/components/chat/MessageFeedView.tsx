import { useState } from "react";
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
  X,
} from "lucide-react";
import { cn } from "@/lib/utils";
import type { ChatActivityView, ChatAttachmentView, ChatFeedEntry } from "./chatTypes";
import { compactText } from "./chatUtils";

interface MessageFeedViewProps {
  entries: ChatFeedEntry[];
  submitting: boolean;
  sessionRunning: boolean;
  lastActivityText: string;
  copiedMessageId: string | null;
  collapsedActivityGroups: Record<string, boolean>;
  onCopyMessage: (id: string, content: string) => void;
  onCreateWorkItem: (id: string, content: string) => void;
  onActivityGroupToggle: (id: string) => void;
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


function ImageLightbox({ src, alt, onClose }: { src: string; alt: string; onClose: () => void }) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70"
      onClick={onClose}
    >
      <button
        type="button"
        className="absolute right-4 top-4 rounded-full bg-black/50 p-1.5 text-white hover:bg-black/70"
        onClick={onClose}
      >
        <X className="h-5 w-5" />
      </button>
      <img
        src={src}
        alt={alt}
        className="max-h-[90vh] max-w-[90vw] rounded object-contain"
        onClick={(e) => e.stopPropagation()}
      />
    </div>
  );
}

function AttachmentImagePreviews({ attachments }: { attachments: ChatAttachmentView[] }) {
  const [lightboxSrc, setLightboxSrc] = useState<{ src: string; alt: string } | null>(null);

  const imageAttachments = attachments.filter((a) => a.mime_type.startsWith("image/"));
  if (imageAttachments.length === 0) return null;

  return (
    <>
      <div className="mt-1.5 flex flex-wrap gap-2">
        {imageAttachments.map((att, idx) => {
          const src = `data:${att.mime_type};base64,${att.data}`;
          return (
            <button
              key={idx}
              type="button"
              className="group/img overflow-hidden rounded border border-border/60 bg-muted/30 transition-shadow hover:shadow-md"
              onClick={() => setLightboxSrc({ src, alt: att.name })}
              title={att.name}
            >
              <img
                src={src}
                alt={att.name}
                className="max-h-48 max-w-xs object-contain"
              />
            </button>
          );
        })}
      </div>
      {lightboxSrc && (
        <ImageLightbox src={lightboxSrc.src} alt={lightboxSrc.alt} onClose={() => setLightboxSrc(null)} />
      )}
    </>
  );
}

export function MessageFeedView(props: MessageFeedViewProps) {
  const {
    entries,
    submitting,
    sessionRunning,
    lastActivityText,
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

        /* ── tool_group: collapsed = summary of running items; expanded = all items ── */
        if (entry.type === "tool_group") {
          const isExpanded = collapsedActivityGroups[entry.id] === true;
          const count = entry.items.length;
          const activeItems = entry.items.filter((item) => item.data.status !== "completed");
          const completedCount = count - activeItems.length;

          /* summary: show first + last active with ellipsis */
          const summaryItems = activeItems.length <= 2
            ? activeItems
            : [activeItems[0], activeItems[activeItems.length - 1]];
          const omitted = activeItems.length - summaryItems.length;

          const displayItems = isExpanded ? entry.items : summaryItems;

          return (
            <div key={entry.id} className="rounded border border-amber-200/70 bg-amber-50/50 px-2 py-1 shadow-sm shadow-amber-100/40">
              <button
                type="button"
                className="flex w-full items-center gap-1.5 text-[11px] text-muted-foreground transition-colors hover:text-foreground"
                onClick={() => onActivityGroupToggle(entry.id)}
              >
                {isExpanded ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                <Wrench className="h-3 w-3 text-amber-500" />
                <span className="font-medium">
                  {count} {t("chat.toolCalls").toLowerCase()}
                  {completedCount > 0 && (
                    <span className="ml-1 font-normal text-emerald-600">({completedCount} {t("chat.completed")})</span>
                  )}
                </span>
              </button>
              {displayItems.length > 0 && (
                <div className="mt-1">
                  {displayItems.map((item, idx) => {
                    const act = item.data;
                    const snippet = compactText(act.detail || act.title, 80);
                    const badgeText = statusLabel(act.status, t);
                    return (
                      <div key={act.id}>
                        <div className="flex items-baseline gap-1.5 py-0.5 pl-5">
                          <span className="shrink-0 text-[11px] font-semibold text-foreground">{act.title}</span>
                          {badgeText && (
                            <span className={cn(
                              "shrink-0 rounded-full border px-1 py-px text-[9px] font-medium leading-none",
                              statusBadgeClass(act.status),
                            )}>
                              {badgeText}
                            </span>
                          )}
                          <span className="min-w-0 truncate text-[10px] text-muted-foreground">{snippet}</span>
                        </div>
                        {!isExpanded && idx === 0 && omitted > 0 && (
                          <div className="py-0.5 pl-5 text-[10px] text-muted-foreground/60">… {omitted} more</div>
                        )}
                      </div>
                    );
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
              {message.attachments && message.attachments.length > 0 && (
                <AttachmentImagePreviews attachments={message.attachments} />
              )}
            </div>
          </div>
        );
      })}
      {(submitting || sessionRunning) && (
        <div className="flex items-center gap-1.5 rounded border border-emerald-200/70 bg-emerald-50/40 px-2.5 py-1 text-xs text-emerald-700">
          <Loader2 className="h-3 w-3 animate-spin shrink-0" />
          <span className="min-w-0 truncate">
            {lastActivityText ? compactText(lastActivityText, 120) : `${t("chat.thinking")}...`}
          </span>
        </div>
      )}
    </>
  );
}
