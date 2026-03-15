import type { RefObject, ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Bot, Loader2, User } from "lucide-react";
import { cn } from "@/lib/utils";
import type { AgentProfile, ThreadMessage } from "@/types/apiV2";

interface ThreadMessageListProps {
  messages: ThreadMessage[];
  profileByID: Map<string, AgentProfile>;
  thinkingAgentIDs: Set<string>;
  sending: boolean;
  messagesEndRef: RefObject<HTMLDivElement>;
  renderMessageContent: (msg: ThreadMessage) => ReactNode;
  focusAgentProfile: (profileID: string) => void;
  readTargetAgentID: (metadata: Record<string, unknown> | undefined) => string | null;
  readAutoRoutedTo: (metadata: Record<string, unknown> | undefined) => string[];
  readWorkItemTrackID: (metadata: Record<string, unknown> | undefined) => number | null;
  formatRelativeTime: (value: string) => string;
}

export function ThreadMessageList({
  messages,
  profileByID,
  thinkingAgentIDs,
  sending,
  messagesEndRef,
  renderMessageContent,
  focusAgentProfile,
  readTargetAgentID,
  readAutoRoutedTo,
  readWorkItemTrackID,
  formatRelativeTime,
}: ThreadMessageListProps) {
  const { t } = useTranslation();

  if (messages.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-3 text-muted-foreground">
        <Bot className="h-10 w-10 text-muted-foreground/30" />
        <p className="text-sm">{t("threads.noMessages", "No messages yet. Start the conversation.")}</p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-4">
      {messages.map((msg) => {
        const isAgent = msg.role === "agent";
        const isSystem = msg.role === "system";
        const targetAgent = readTargetAgentID(msg.metadata);
        const autoRoutedTo = readAutoRoutedTo(msg.metadata);
        const workItemTrackID = readWorkItemTrackID(msg.metadata);
        const profile = isAgent ? profileByID.get(msg.sender_id) : undefined;

        if (isSystem) {
          return (
            <div key={msg.id} className="flex justify-center">
              <div className="flex items-center gap-2 rounded-full border border-border/40 bg-muted/40 px-4 py-1.5 text-xs text-muted-foreground">
                {workItemTrackID ? (
                  <span className="rounded-full bg-background px-2 py-0.5 text-[10px] font-medium text-foreground/70">
                    Track #{workItemTrackID}
                  </span>
                ) : null}
                <Bot className="h-3 w-3" />
                <span>{msg.content}</span>
              </div>
            </div>
          );
        }

        return (
          <div key={msg.id} className={cn("flex gap-3", !isAgent && "flex-row-reverse")}>
            <div
              className={cn(
                "flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-xs font-bold",
                isAgent ? "bg-emerald-100 text-emerald-700" : "bg-blue-100 text-blue-700",
              )}
            >
              {isAgent ? <Bot className="h-4 w-4" /> : <User className="h-4 w-4" />}
            </div>
            <div className="group/msg max-w-[75%] min-w-0">
              <div
                className={cn(
                  "mb-1 flex items-center gap-1.5 text-[11px] text-muted-foreground",
                  !isAgent && "flex-row-reverse",
                )}
              >
                <span className="font-medium text-foreground/70">
                  {isAgent ? (profile?.name ?? msg.sender_id) : (msg.sender_id || "You")}
                </span>
                {targetAgent ? (
                  <span className="rounded bg-blue-50 px-1 py-px text-[10px] text-blue-600">
                    @{targetAgent}
                  </span>
                ) : null}
                {workItemTrackID ? (
                  <span className="rounded bg-amber-50 px-1 py-px text-[10px] text-amber-700">
                    Track #{workItemTrackID}
                  </span>
                ) : null}
                <span>{formatRelativeTime(msg.created_at)}</span>
              </div>
              <div
                className={cn(
                  "rounded-2xl px-4 py-2.5 text-sm leading-relaxed",
                  isAgent ? "rounded-tl-md bg-muted/80 text-foreground" : "rounded-tr-md bg-blue-600 text-white",
                )}
              >
                <p className="whitespace-pre-wrap break-words">{renderMessageContent(msg)}</p>
              </div>
              {!isAgent && autoRoutedTo.length > 0 && (
                <div className="mt-1 flex flex-wrap items-center justify-end gap-1 text-[10px]">
                  <span className="text-muted-foreground/60">Auto</span>
                  <span className="text-muted-foreground/40">→</span>
                  {autoRoutedTo.map((agentID) => {
                    const agentProfile = profileByID.get(agentID);
                    return (
                      <button
                        key={agentID}
                        type="button"
                        className="inline-flex items-center gap-1 rounded-full border border-emerald-200 bg-emerald-50 px-1.5 py-0.5 font-medium text-emerald-700 transition-colors hover:bg-emerald-100"
                        onClick={() => focusAgentProfile(agentID)}
                      >
                        <Bot className="h-2.5 w-2.5" />
                        {agentProfile?.name ?? agentID}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          </div>
        );
      })}

      {thinkingAgentIDs.size > 0 && (
        <div className="flex flex-col gap-2">
          {[...thinkingAgentIDs].map((agentID) => {
            const profile = profileByID.get(agentID);
            return (
              <div key={agentID} className="flex items-center gap-3">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-emerald-700">
                  <Bot className="h-4 w-4" />
                </div>
                <div className="flex items-center gap-2 rounded-2xl rounded-tl-md bg-muted/60 px-4 py-2.5">
                  <span className="text-xs font-medium text-muted-foreground">
                    {profile?.name ?? agentID}
                  </span>
                  <span className="inline-flex items-center gap-0.5">
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50" style={{ animationDelay: "0ms" }} />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50" style={{ animationDelay: "150ms" }} />
                    <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-muted-foreground/50" style={{ animationDelay: "300ms" }} />
                  </span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {sending && thinkingAgentIDs.size === 0 && (
        <div className="flex items-center gap-2 px-11 text-xs text-muted-foreground">
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          <span>{t("threads.sending", "Sending")}...</span>
        </div>
      )}

      <div ref={messagesEndRef} />
    </div>
  );
}
