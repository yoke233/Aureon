// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { FeatureManifestPage } from "./FeatureManifestPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage(initialEntry = "/projects/9/manifest") {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/projects/:projectId/manifest" element={<FeatureManifestPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("FeatureManifestPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
    vi.stubGlobal("confirm", vi.fn(() => true));
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("支持加载功能清单、新增条目、更新状态并删除", async () => {
    const summaryResponses = [
      { project_id: 9, pass: 1, fail: 0, pending: 1, skipped: 0, total: 2 },
      { project_id: 9, pass: 2, fail: 0, pending: 0, skipped: 0, total: 2 },
      { project_id: 9, pass: 2, fail: 0, pending: 0, skipped: 0, total: 2 },
      { project_id: 9, pass: 1, fail: 0, pending: 0, skipped: 0, total: 1 },
    ];
    const entryResponses = [
      [
        {
          id: 1,
          project_id: 9,
          key: "auth.login.success",
          description: "用户登录成功",
          status: "pass",
          tags: ["auth"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
        {
          id: 2,
          project_id: 9,
          key: "auth.login.fail",
          description: "用户登录失败",
          status: "pending",
          tags: ["auth", "edge"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
      ],
      [
        {
          id: 1,
          project_id: 9,
          key: "auth.login.success",
          description: "用户登录成功",
          status: "pass",
          tags: ["auth"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
        {
          id: 2,
          project_id: 9,
          key: "auth.login.fail",
          description: "用户登录失败",
          status: "pending",
          tags: ["auth", "edge"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
        {
          id: 3,
          project_id: 9,
          key: "checkout.submit",
          description: "提交订单",
          status: "pending",
          tags: ["checkout", "critical path"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
      ],
      [
        {
          id: 1,
          project_id: 9,
          key: "auth.login.success",
          description: "用户登录成功",
          status: "pass",
          tags: ["auth"],
          created_at: "2026-03-15T00:00:00Z",
          updated_at: "2026-03-15T00:00:00Z",
        },
      ],
    ];

    const apiClient = {
      getManifestSummary: vi.fn().mockImplementation(() => Promise.resolve(summaryResponses.shift() ?? summaryResponses.at(-1))),
      listManifestEntries: vi.fn().mockImplementation(() => Promise.resolve(entryResponses.shift() ?? entryResponses.at(-1) ?? [])),
      createManifestEntry: vi.fn().mockResolvedValue({}),
      updateManifestEntryStatus: vi.fn().mockResolvedValue({}),
      deleteManifestEntry: vi.fn().mockResolvedValue({}),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("功能清单")).toBeTruthy();
    expect(await screen.findByText("auth.login.success")).toBeTruthy();
    expect(screen.getByText("用户登录失败")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "添加功能" }));
    fireEvent.change(screen.getByPlaceholderText("如 auth.login.success"), {
      target: { value: "checkout.submit" },
    });
    fireEvent.change(screen.getByPlaceholderText("描述端到端场景"), {
      target: { value: "提交订单" },
    });
    fireEvent.change(screen.getByPlaceholderText("逗号分隔"), {
      target: { value: "checkout, critical path" },
    });
    fireEvent.click(screen.getByRole("button", { name: "创建" }));

    await waitFor(() => {
      expect(apiClient.createManifestEntry).toHaveBeenCalledWith(9, {
        key: "checkout.submit",
        description: "提交订单",
        tags: ["checkout", "critical path"],
      });
    });

    expect(await screen.findByText("checkout.submit")).toBeTruthy();

    const authFailRow = screen.getByText("auth.login.fail").closest("tr");
    expect(authFailRow).toBeTruthy();
    fireEvent.change(within(authFailRow as HTMLElement).getByRole("combobox"), {
      target: { value: "pass" },
    });

    await waitFor(() => {
      expect(apiClient.updateManifestEntryStatus).toHaveBeenCalledWith(2, "pass");
    });

    fireEvent.click(within(authFailRow as HTMLElement).getByTitle("删除"));

    await waitFor(() => {
      expect(apiClient.deleteManifestEntry).toHaveBeenCalledWith(2);
    });

    await waitFor(() => {
      expect(screen.queryByText("auth.login.fail")).toBeNull();
    });
  });

  it("新增功能失败时展示错误", async () => {
    const apiClient = {
      getManifestSummary: vi.fn().mockResolvedValue({ project_id: 9, pass: 0, fail: 0, pending: 0, skipped: 0, total: 0 }),
      listManifestEntries: vi.fn().mockResolvedValue([]),
      createManifestEntry: vi.fn().mockRejectedValue(new Error("duplicate key")),
      updateManifestEntryStatus: vi.fn(),
      deleteManifestEntry: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("暂无功能条目，添加第一个。")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "添加功能" }));
    fireEvent.change(screen.getByPlaceholderText("如 auth.login.success"), {
      target: { value: "auth.login.success" },
    });
    fireEvent.click(screen.getByRole("button", { name: "创建" }));

    expect(await screen.findByText("duplicate key")).toBeTruthy();
  });
});
