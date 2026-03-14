// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { ThreadsPage } from "./ThreadsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <ThreadsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("ThreadsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载讨论列表并支持搜索和创建", async () => {
    const apiClient = {
      listThreads: vi.fn().mockResolvedValue([
        {
          id: 7,
          title: "支付问题排查",
          status: "active",
          owner_id: "alice",
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
        {
          id: 8,
          title: "发布复盘",
          status: "closed",
          owner_id: "bob",
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
      ]),
      createThread: vi.fn().mockResolvedValue({
        id: 9,
        title: "新的讨论",
        status: "active",
        owner_id: "charlie",
        created_at: "2026-03-15T00:00:00Z",
        updated_at: "2026-03-15T00:00:00Z",
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("支付问题排查")).toBeTruthy();
    expect(screen.getByText("发布复盘")).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("Search threads..."), {
      target: { value: "发布" },
    });

    await waitFor(() => {
      expect(screen.getByText("发布复盘")).toBeTruthy();
      expect(screen.queryByText("支付问题排查")).toBeNull();
    });

    fireEvent.change(screen.getByPlaceholderText("Search threads..."), {
      target: { value: "" },
    });

    fireEvent.click(screen.getByRole("button", { name: "New Thread" }));
    fireEvent.change(screen.getByPlaceholderText("Thread title..."), {
      target: { value: "新的讨论" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => {
      expect(apiClient.createThread).toHaveBeenCalledWith({ title: "新的讨论" });
    });

    expect(await screen.findByText("新的讨论")).toBeTruthy();
  });

  it("创建讨论失败时展示错误", async () => {
    const apiClient = {
      listThreads: vi.fn().mockResolvedValue([]),
      createThread: vi.fn().mockRejectedValue(new Error("create failed")),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("No threads yet")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "New Thread" }));
    fireEvent.change(screen.getByPlaceholderText("Thread title..."), {
      target: { value: "失败的讨论" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(await screen.findByText("create failed")).toBeTruthy();
  });
});
