// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { GitTagsPage } from "./GitTagsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function renderPage(initialEntry = "/projects/9/git-tags") {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter initialEntries={[initialEntry]}>
        <Routes>
          <Route path="/projects/:projectId/git-tags" element={<GitTagsPage />} />
        </Routes>
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("GitTagsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
    vi.stubGlobal("scrollTo", vi.fn());
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("默认加载提交记录并可基于 commit 创建标签", async () => {
    const apiClient = {
      listGitCommits: vi.fn().mockResolvedValue([
        {
          sha: "abcdef1234567890",
          short: "abcdef1",
          message: "feat: release",
          author: "Alice",
          timestamp: "2026-03-13T00:00:00Z",
        },
      ]),
      listGitTags: vi.fn().mockResolvedValue([]),
      createGitTag: vi.fn().mockResolvedValue({
        name: "v1.0.0",
        sha: "abcdef1234567890",
        pushed: true,
      }),
      pushGitTag: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    await waitFor(() => {
      expect(apiClient.listGitCommits).toHaveBeenCalledWith(9, { limit: 50 });
    });

    expect(await screen.findByText("feat: release")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "打标签" }));

    expect(screen.getByDisplayValue("abcdef1234567890")).toBeTruthy();

    fireEvent.change(screen.getByPlaceholderText("例如: v1.0.0"), {
      target: { value: "v1.0.0" },
    });
    fireEvent.change(screen.getByPlaceholderText("版本说明..."), {
      target: { value: "首个稳定版本" },
    });
    fireEvent.click(screen.getByRole("button", { name: "创建标签" }));

    await waitFor(() => {
      expect(apiClient.createGitTag).toHaveBeenCalledWith(9, {
        name: "v1.0.0",
        ref: "abcdef1234567890",
        message: "首个稳定版本",
        push: true,
      });
    });

    expect(await screen.findByText("Tag v1.0.0 已创建并推送成功 (abcdef1)")).toBeTruthy();
    expect(apiClient.listGitTags).toHaveBeenCalled();
  });

  it("切换到标签列表后支持推送单个标签并展示失败信息", async () => {
    const apiClient = {
      listGitCommits: vi.fn().mockResolvedValue([]),
      listGitTags: vi.fn().mockResolvedValue([
        {
          name: "v1.0.1",
          sha: "1234567890abcdef",
          message: "修复补丁",
          timestamp: "2026-03-14T00:00:00Z",
        },
      ]),
      createGitTag: vi.fn(),
      pushGitTag: vi.fn().mockRejectedValue(new Error("remote rejected")),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    fireEvent.click(screen.getByRole("button", { name: "标签列表" }));

    await waitFor(() => {
      expect(apiClient.listGitTags).toHaveBeenCalledWith(9);
    });

    const row = await screen.findByText("v1.0.1");
    fireEvent.click(within(row.closest("tr") as HTMLElement).getByRole("button", { name: "推送" }));

    await waitFor(() => {
      expect(apiClient.pushGitTag).toHaveBeenCalledWith(9, { name: "v1.0.1" });
    });

    expect(await screen.findByText("推送失败: remote rejected")).toBeTruthy();
  });
});
