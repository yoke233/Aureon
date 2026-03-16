// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import type { ChatFeedEntry } from "./chatTypes";
import { MessageFeedView } from "./MessageFeedView";

function renderView(entries: ChatFeedEntry[], collapsedActivityGroups: Record<string, boolean> = {}) {
  return render(
    <I18nextProvider i18n={i18n}>
      <MessageFeedView
        entries={entries}
        submitting={false}
        sessionRunning={false}
        lastActivityText=""
        copiedMessageId={null}
        collapsedActivityGroups={collapsedActivityGroups}
        onCopyMessage={vi.fn()}
        onCreateWorkItem={vi.fn()}
        onActivityGroupToggle={vi.fn()}
      />
    </I18nextProvider>,
  );
}

describe("MessageFeedView", () => {
  beforeEach(() => {
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("工具组隐藏已完成的调用，只在标题显示已完成计数", () => {
    const entries: ChatFeedEntry[] = [
      {
        type: "tool_group",
        id: "tg-1",
        items: [
          {
            kind: "tool_call",
            data: {
              id: "tool-1",
              type: "tool_call",
              title: "读取工作区文件",
              detail: "已完成的内容",
              time: "10:00:00",
              at: "2026-03-16T10:00:00Z",
              status: "completed",
            },
          },
        ],
      },
    ];

    renderView(entries);

    // 标题栏仍可见
    expect(screen.getByText(/1.*工具调用/)).not.toBeNull();
    // completed 项的标题不渲染在展开区域
    expect(screen.queryByText("读取工作区文件")).toBeNull();
  });

  it("工具组只显示非完成的调用，第一条和最后一条之间省略", () => {
    const entries: ChatFeedEntry[] = [
      {
        type: "tool_group",
        id: "tg-1",
        items: [
          {
            kind: "tool_call",
            data: { id: "t1", type: "tool_call", title: "完成的", detail: "", time: "10:00:00", at: "2026-03-16T10:00:00Z", status: "completed" },
          },
          {
            kind: "tool_call",
            data: { id: "t2", type: "tool_call", title: "第一个运行中", detail: "detail-a", time: "10:00:01", at: "2026-03-16T10:00:01Z", status: "running" },
          },
          {
            kind: "tool_call",
            data: { id: "t3", type: "tool_call", title: "中间的", detail: "detail-b", time: "10:00:02", at: "2026-03-16T10:00:02Z", status: "running" },
          },
          {
            kind: "tool_call",
            data: { id: "t4", type: "tool_call", title: "最后一个", detail: "detail-c", time: "10:00:03", at: "2026-03-16T10:00:03Z", status: "running" },
          },
        ],
      },
    ];

    renderView(entries);

    // 第一个和最后一个显示
    expect(screen.getByText("第一个运行中")).not.toBeNull();
    expect(screen.getByText("最后一个")).not.toBeNull();
    // 中间被省略
    expect(screen.queryByText("中间的")).toBeNull();
    expect(screen.getByText(/1 more/)).not.toBeNull();
  });

  it("展开后显示全部调用（包括已完成）", () => {
    const entries: ChatFeedEntry[] = [
      {
        type: "tool_group",
        id: "tg-3",
        items: [
          {
            kind: "tool_call",
            data: {
              id: "tool-done",
              type: "tool_call",
              title: "已完成的调用",
              detail: "done detail",
              time: "10:00:00",
              at: "2026-03-16T10:00:00Z",
              status: "completed",
            },
          },
          {
            kind: "tool_call",
            data: {
              id: "tool-run",
              type: "tool_call",
              title: "运行中的调用",
              detail: "running detail",
              time: "10:00:01",
              at: "2026-03-16T10:00:01Z",
              status: "running",
            },
          },
        ],
      },
    ];

    // 默认：只显示运行中的
    const { unmount } = renderView(entries);
    expect(screen.getByText("运行中的调用")).not.toBeNull();
    expect(screen.queryByText("已完成的调用")).toBeNull();
    unmount();

    // 展开后：全部显示
    renderView(entries, { "tg-3": true });
    expect(screen.getByText("运行中的调用")).not.toBeNull();
    expect(screen.getByText("已完成的调用")).not.toBeNull();
  });
});
