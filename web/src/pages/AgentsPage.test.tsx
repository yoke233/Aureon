// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { AgentsPage } from "./AgentsPage";

const { mockUseWorkbench } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

function createWsClientMock() {
  const handlers = new Map<string, (payload: unknown) => void>();
  return {
    subscribe: vi.fn((type: string, handler: (payload: unknown) => void) => {
      handlers.set(type, handler);
      return vi.fn();
    }),
    emit(type: string, payload: unknown) {
      const handler = handlers.get(type);
      if (handler) {
        handler(payload);
      }
    },
  };
}

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <AgentsPage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("AgentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持编辑并保存 LLM 配置", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      listDrivers: vi.fn().mockResolvedValue([
        {
          id: "codex-cli",
          launch_command: "codex",
          launch_args: ["serve"],
          capabilities_max: { fs_read: true, fs_write: true, terminal: true },
        },
      ]),
      listProfiles: vi.fn().mockResolvedValue([
        {
          id: "lead",
          name: "Lead",
          role: "lead",
          driver_id: "codex-cli",
          skills: ["planner"],
          session: { reuse: true },
        },
      ]),
      getLLMConfig: vi.fn().mockResolvedValue({
        default_config_id: "openai-prod",
        configs: [
          {
            id: "openai-prod",
            type: "openai_response",
            model: "gpt-4.1-mini",
          },
        ],
      }),
      getSandboxSupport: vi.fn().mockResolvedValue({
        os: "windows",
        arch: "amd64",
        enabled: true,
        configured_provider: "docker",
        current_provider: "docker",
        current_supported: true,
        providers: {},
      }),
      updateLLMConfig: vi.fn().mockResolvedValue({
        default_config_id: "openai-prod",
        configs: [
          {
            id: "openai-prod",
            type: "openai_response",
            model: "gpt-4.1",
          },
        ],
      }),
    };

    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    expect(await screen.findByText("代理管理")).toBeTruthy();

    fireEvent.change(await screen.findByDisplayValue("gpt-4.1-mini"), {
      target: { value: "gpt-4.1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(apiClient.updateLLMConfig).toHaveBeenCalledWith({
        default_config_id: "openai-prod",
        configs: [
          {
            id: "openai-prod",
            type: "openai_response",
            model: "gpt-4.1",
          },
        ],
      });
    });

    expect(await screen.findByDisplayValue("gpt-4.1")).toBeTruthy();
  });

  it("收到 runtime 配置重载事件后重新拉取页面数据", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      listDrivers: vi.fn().mockResolvedValue([
        {
          id: "codex-cli",
          launch_command: "codex",
          launch_args: ["serve"],
          capabilities_max: { fs_read: true, fs_write: true, terminal: true },
        },
      ]),
      listProfiles: vi.fn().mockResolvedValue([
        {
          id: "worker-a",
          name: "Worker A",
          role: "worker",
          driver_id: "codex-cli",
          skills: [],
          session: { reuse: false },
        },
      ]),
      getLLMConfig: vi.fn().mockResolvedValue({
        default_config_id: "claude-backup",
        configs: [
          {
            id: "claude-backup",
            type: "anthropic",
            model: "claude-3-7-sonnet-latest",
          },
        ],
      }),
      getSandboxSupport: vi.fn().mockResolvedValue({
        os: "linux",
        arch: "amd64",
        enabled: false,
        configured_provider: "docker",
        current_provider: "",
        current_supported: false,
        providers: {},
      }),
      updateLLMConfig: vi.fn(),
    };

    mockUseWorkbench.mockReturnValue({ apiClient, wsClient });

    renderPage();

    expect(await screen.findByText("worker-a")).toBeTruthy();

    expect(apiClient.listDrivers).toHaveBeenCalledTimes(1);
    expect(apiClient.listProfiles).toHaveBeenCalledTimes(1);
    expect(apiClient.getLLMConfig).toHaveBeenCalledTimes(1);
    expect(apiClient.getSandboxSupport).toHaveBeenCalledTimes(1);

    wsClient.emit("runtime.config_reloaded", {});

    await waitFor(() => {
      expect(apiClient.listDrivers).toHaveBeenCalledTimes(2);
      expect(apiClient.listProfiles).toHaveBeenCalledTimes(2);
      expect(apiClient.getLLMConfig).toHaveBeenCalledTimes(2);
      expect(apiClient.getSandboxSupport).toHaveBeenCalledTimes(2);
    });
  });
});
