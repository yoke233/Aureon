// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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

  it("工具调用默认展开并以五行预览展示详细内容", () => {
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
              detail: [
                "line-1",
                "line-2",
                "line-3",
                "line-4",
                "line-5",
                "line-6",
                "line-7",
              ].join("\n"),
              time: "10:00:00",
              at: "2026-03-16T10:00:00Z",
              status: "completed",
            },
          },
        ],
      },
    ];

    renderView(entries);

    expect(screen.getByText("读取工作区文件")).not.toBeNull();
    expect(screen.getByRole("button", { name: "展开" })).not.toBeNull();
    expect(screen.getByTestId("tool-call-detail-tool-1").className).toContain("line-clamp-5");
  });

  it("工具调用详情可以展开和收起", () => {
    const entries: ChatFeedEntry[] = [
      {
        type: "tool_group",
        id: "tg-1",
        items: [
          {
            kind: "tool_call",
            data: {
              id: "tool-2",
              type: "tool_call",
              title: "执行命令",
              detail: new Array(10).fill("stdout output").join("\n"),
              time: "10:00:00",
              at: "2026-03-16T10:00:00Z",
              status: "running",
            },
          },
        ],
      },
    ];

    renderView(entries);

    const detail = screen.getByTestId("tool-call-detail-tool-2");
    fireEvent.click(screen.getByRole("button", { name: "展开" }));
    expect(detail.className.includes("line-clamp-5")).toBe(false);

    fireEvent.click(screen.getByRole("button", { name: "收起" }));
    expect(detail.className).toContain("line-clamp-5");
  });

  it("显式折叠的工具组不会默认展开", () => {
    const entries: ChatFeedEntry[] = [
      {
        type: "tool_group",
        id: "tg-3",
        items: [
          {
            kind: "tool_call",
            data: {
              id: "tool-3",
              type: "tool_call",
              title: "不会显示的详情",
              detail: "hidden detail",
              time: "10:00:00",
              at: "2026-03-16T10:00:00Z",
            },
          },
        ],
      },
    ];

    renderView(entries, { "tg-3": true });

    expect(screen.queryByText("不会显示的详情")).toBeNull();
  });
});
