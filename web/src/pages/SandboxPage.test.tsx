// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "@/i18n";
import { SandboxPage } from "./SandboxPage";

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
        <SandboxPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("SandboxPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("加载沙盒信息后支持切换开关、provider 并保存", async () => {
    const apiClient = {
      getSandboxSupport: vi.fn().mockResolvedValue({
        os: "linux",
        arch: "amd64",
        enabled: false,
        configured_provider: "home_dir",
        current_provider: "home_dir",
        current_supported: true,
        providers: {
          docker: { supported: true, implemented: true, reason: "已连接 docker" },
          home_dir: { supported: true, implemented: true, reason: "基础隔离" },
        },
      }),
      updateSandboxSupport: vi.fn().mockResolvedValue({
        os: "linux",
        arch: "amd64",
        enabled: true,
        configured_provider: "docker",
        current_provider: "docker",
        current_supported: true,
        providers: {
          docker: { supported: true, implemented: true, reason: "已连接 docker" },
          home_dir: { supported: true, implemented: true, reason: "基础隔离" },
        },
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("linux")).toBeTruthy();
    expect(screen.getAllByText("基础隔离").length).toBeGreaterThan(0);

    const saveButton = screen.getByRole("button", { name: "保存配置" });
    expect(saveButton.hasAttribute("disabled")).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: "启用沙盒" }));
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "docker" } });

    expect(screen.getAllByText("已连接 docker").length).toBeGreaterThan(0);
    expect(saveButton.hasAttribute("disabled")).toBe(false);

    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(apiClient.updateSandboxSupport).toHaveBeenCalledWith({
        enabled: true,
        provider: "docker",
      });
    });

    expect(screen.getAllByText("docker").length).toBeGreaterThan(0);
  });

  it("加载失败时展示错误", async () => {
    const apiClient = {
      getSandboxSupport: vi.fn().mockRejectedValue(new Error("sandbox unavailable")),
      updateSandboxSupport: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient });

    renderPage();

    expect(await screen.findByText("sandbox unavailable")).toBeTruthy();
  });
});
