// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { I18nextProvider } from "react-i18next";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import i18n from "../i18n";
import { MobileHomePage } from "./MobileHomePage";

const { mockUseWorkbench, mockNavigate } = vi.hoisted(() => ({
  mockUseWorkbench: vi.fn(),
  mockNavigate: vi.fn(),
}));

vi.mock("@/contexts/WorkbenchContext", () => ({
  useWorkbench: mockUseWorkbench,
}));

vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual<typeof import("react-router-dom")>("react-router-dom");
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

function createWsClientMock() {
  const handlers: Record<string, (payload?: unknown) => void> = {};
  return {
    subscribe: vi.fn((event: string, handler: (payload?: unknown) => void) => {
      handlers[event] = handler;
      return vi.fn();
    }),
    send: vi.fn(),
    handlers,
  };
}

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <MobileHomePage />
      </MemoryRouter>
    </I18nextProvider>,
  );
}

describe("MobileHomePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    void i18n.changeLanguage("zh-CN");
  });

  afterEach(() => {
    cleanup();
  });

  it("支持打开已有会话并在配置热更新后刷新 driver/profile 列表", async () => {
    const wsClient = createWsClientMock();
    const apiClient = {
      listChatSessions: vi.fn().mockResolvedValue([
        {
          session_id: "session-1",
          title: "主流程回归",
          project_id: 1,
          project_name: "Alpha",
          profile_id: "lead-1",
          profile_name: "Lead",
          driver_id: "codex-cli",
          status: "alive",
          message_count: 3,
          updated_at: "2026-03-14T12:00:00Z",
          created_at: "2026-03-14T11:00:00Z",
        },
      ]),
      listProfiles: vi.fn().mockResolvedValue([
        { id: "lead-1", role: "lead", name: "Lead", driver_id: "codex-cli" },
      ]),
      listDrivers: vi.fn().mockResolvedValue([
        { id: "codex-cli", name: "Codex CLI" },
      ]),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      wsClient,
      projects: [{ id: 1, name: "Alpha" }],
      selectedProjectId: 1,
      setSelectedProjectId: vi.fn(),
    });

    renderPage();

    fireEvent.click((await screen.findByText("主流程回归")).closest("button") as HTMLButtonElement);

    expect(mockNavigate).toHaveBeenCalledWith("/chat?session=session-1");
    expect(wsClient.subscribe).toHaveBeenCalledWith("runtime.config_reloaded", expect.any(Function));

    wsClient.handlers["runtime.config_reloaded"]?.({});

    await waitFor(() => {
      expect(apiClient.listProfiles).toHaveBeenCalledTimes(2);
      expect(apiClient.listDrivers).toHaveBeenCalledTimes(2);
    });
  });

  it("支持选择项目、上传附件并发送新消息", async () => {
    const wsClient = createWsClientMock();
    const setSelectedProjectId = vi.fn();
    const apiClient = {
      listChatSessions: vi.fn().mockResolvedValue([]),
      listProfiles: vi.fn().mockResolvedValue([
        { id: "lead-1", role: "lead", name: "Lead", driver_id: "codex-cli" },
      ]),
      listDrivers: vi.fn().mockResolvedValue([
        { id: "codex-cli", name: "Codex CLI" },
      ]),
    };

    mockUseWorkbench.mockReturnValue({
      apiClient,
      wsClient,
      projects: [
        { id: 1, name: "Alpha" },
        { id: 2, name: "Beta" },
      ],
      selectedProjectId: 1,
      setSelectedProjectId,
    });

    renderPage();

    const selectors = await screen.findAllByRole("combobox");
    fireEvent.change(selectors[0], { target: { value: "2" } });

    expect(setSelectedProjectId).toHaveBeenCalledWith(2);

    const file = new File(["hi"], "spec.md", { type: "text/markdown" });
    Object.defineProperty(file, "arrayBuffer", {
      configurable: true,
      value: vi.fn().mockResolvedValue(new Uint8Array([104, 105]).buffer),
    });

    const fileInput = document.querySelector("input[type='file']") as HTMLInputElement;
    fireEvent.change(fileInput, { target: { files: [file] } });

    const textarea = screen.getByPlaceholderText("输入消息，开始与 Lead 对话...");
    fireEvent.change(textarea, { target: { value: "请整理这次发布计划" } });
    fireEvent.keyDown(textarea, { key: "Enter", code: "Enter" });

    await waitFor(() => {
      expect(wsClient.send).toHaveBeenCalledTimes(1);
    });

    const payload = wsClient.send.mock.calls[0][0];
    expect(payload.type).toBe("chat.send");
    expect(payload.data.message).toBe("请整理这次发布计划");
    expect(payload.data.project_id).toBe(2);
    expect(payload.data.project_name).toBe("Beta");
    expect(payload.data.profile_id).toBe("lead-1");
    expect(payload.data.driver_id).toBe("codex-cli");
    expect(payload.data.attachments).toHaveLength(1);
    expect(payload.data.attachments[0]).toMatchObject({
      name: "spec.md",
      mime_type: "text/markdown",
      data: "aGk=",
    });
    expect(mockNavigate).toHaveBeenCalledWith("/chat");
  });
});
