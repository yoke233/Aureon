import { useMemo } from "react";
import type { ChatMessageView, ChatActivityView, ChatFeedItem, ChatFeedEntry } from "./chatTypes";

/**
 * Computes the merged, sorted chat feed from messages + activities,
 * groups tool calls, and handles pagination.
 */
export function useChatFeed(
  currentMessages: ChatMessageView[],
  currentActivities: ChatActivityView[],
  feedVisibleCount: number,
) {
  const chatFeed = useMemo<ChatFeedItem[]>(() => {
    const items: ChatFeedItem[] = [];
    for (const msg of currentMessages) {
      items.push({ kind: "message", data: msg });
    }

    // Build a set of existing assistant message contents for dedup
    const existingContents = new Set(
      currentMessages
        .filter((m) => m.role === "assistant")
        .map((m) => m.content.trim()),
    );

    for (const act of currentActivities) {
      if (act.type === "agent_thought") {
        items.push({ kind: "thought", data: act });
      } else if (act.type === "tool_call") {
        items.push({ kind: "tool_call", data: act });
      } else if (act.type === "agent_message") {
        const content = (act.detail || act.title).trim();
        // Skip if an identical assistant message already exists from the messages API
        if (existingContents.has(content)) {
          continue;
        }
        items.push({
          kind: "message",
          data: {
            id: act.id,
            role: "assistant",
            content,
            time: act.time,
            at: act.at,
          },
        });
      }
    }
    items.sort((a, b) => {
      const aAt = a.kind === "message" ? a.data.at : a.data.at;
      const bAt = b.kind === "message" ? b.data.at : b.data.at;
      return new Date(aAt).getTime() - new Date(bAt).getTime();
    });
    return items;
  }, [currentMessages, currentActivities]);

  const chatFeedEntries = useMemo<ChatFeedEntry[]>(() => {
    const entries: ChatFeedEntry[] = [];
    let toolBuffer: (ChatFeedItem & { kind: "tool_call" })[] = [];
    let groupCounter = 0;

    const flushTools = () => {
      if (toolBuffer.length > 0) {
        entries.push({ type: "tool_group", id: `tg-${groupCounter++}`, items: [...toolBuffer] });
        toolBuffer = [];
      }
    };

    for (const item of chatFeed) {
      if (item.kind === "tool_call") {
        toolBuffer.push(item);
      } else {
        flushTools();
        if (item.kind === "message") {
          entries.push({ type: "message", item });
        } else if (item.kind === "thought") {
          entries.push({ type: "thought", item });
        }
      }
    }
    flushTools();
    return entries;
  }, [chatFeed]);

  const visibleFeedEntries = useMemo(() => {
    const start = Math.max(0, chatFeedEntries.length - feedVisibleCount);
    return chatFeedEntries.slice(start);
  }, [chatFeedEntries, feedVisibleCount]);

  const hasMoreFeedEntries = feedVisibleCount < chatFeedEntries.length;

  return { chatFeed, chatFeedEntries, visibleFeedEntries, hasMoreFeedEntries };
}
