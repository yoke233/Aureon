// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import SystemEventBanner from "./SystemEventBanner";

function createWsClientMock() {
  const handlers: Record<string, (payload?: unknown) => void> = {};
  return {
    subscribe: vi.fn((event: string, handler: (payload?: unknown) => void) => {
      handlers[event] = handler;
      return vi.fn();
    }),
    onStatusChange: vi.fn(() => vi.fn()),
    handlers,
  };
}

function renderBanner(wsClient: ReturnType<typeof createWsClientMock>) {
  return render(
    <I18nextProvider i18n={i18n}>
      <SystemEventBanner wsClient={wsClient as never} />
    </I18nextProvider>,
  );
}

describe("SystemEventBanner", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it("展示预检步骤并在通过后自动隐藏", async () => {
    vi.useFakeTimers();
    const wsClient = createWsClientMock();

    renderBanner(wsClient);
    await act(async () => {});

    act(() => {
      wsClient.handlers.system_event?.({
        event: "preflight_start",
        data: {},
      });
    });
    expect(screen.getByText("正在运行预检质量门...")).toBeTruthy();

    act(() => {
      wsClient.handlers.system_event?.({
        event: "preflight_step",
        data: {
          message: "检查 Git 状态",
          name: "Git fetch",
          status: "PASS",
          duration: "120ms",
        },
      });
    });

    expect(screen.getByText("检查 Git 状态")).toBeTruthy();
    expect(screen.getByText("Git fetch")).toBeTruthy();
    expect(screen.getByText("120ms")).toBeTruthy();

    act(() => {
      wsClient.handlers.system_event?.({
        event: "preflight_pass",
        data: {},
      });
    });

    expect(screen.getByText("预检通过")).toBeTruthy();

    await vi.advanceTimersByTimeAsync(8000);

    expect(screen.queryByText("预检通过")).toBeNull();
  });

  it("展示工作区警告并支持手动关闭", async () => {
    const wsClient = createWsClientMock();

    renderBanner(wsClient);
    await act(async () => {});

    act(() => {
      wsClient.handlers["workspace.warning"]?.({
        warnings: ["git fetch 失败", "remote 未配置"],
      });
    });

    expect(screen.getByText("工作区准备警告")).toBeTruthy();
    expect(screen.getByText("git fetch 失败")).toBeTruthy();
    expect(screen.getByText("remote 未配置")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "×" }));

    expect(screen.queryByText("工作区准备警告")).toBeNull();
  });
});
