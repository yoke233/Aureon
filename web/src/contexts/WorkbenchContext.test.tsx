// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WorkbenchProvider, useWorkbench } from "./WorkbenchContext";

const { connectMock, disconnectMock, isDesktopMock, fetchDesktopBootstrapMock } = vi.hoisted(() => ({
  connectMock: vi.fn(),
  disconnectMock: vi.fn(),
  isDesktopMock: vi.fn(() => false),
  fetchDesktopBootstrapMock: vi.fn(),
}));

vi.mock("@/lib/wsClient", () => ({
  createWsClient: () => ({
    connect: connectMock,
    disconnect: disconnectMock,
  }),
}));

vi.mock("@/lib/desktopBridge", () => ({
  isDesktop: isDesktopMock,
  fetchDesktopBootstrap: fetchDesktopBootstrapMock,
}));

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function Probe() {
  const workbench = useWorkbench();
  return (
    <div>
      <div data-testid="auth-status">{workbench.authStatus}</div>
      <div data-testid="auth-error">{workbench.authError ?? ""}</div>
      <div data-testid="projects">{workbench.projects.map((project) => project.name).join(",")}</div>
      <button onClick={() => workbench.logout()}>logout</button>
      <button onClick={() => workbench.login("manual-secret")}>login</button>
    </div>
  );
}

function renderProvider() {
  return render(
    <WorkbenchProvider>
      <Probe />
    </WorkbenchProvider>,
  );
}

describe("WorkbenchProvider", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    window.history.replaceState({}, "", "/");
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  it("无鉴权部署下启动后 ready，logout 会重新 bootstrap 而不是停在 error", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ auth_required: false }))
      .mockResolvedValueOnce(jsonResponse([{ id: 1, name: "Alpha" }]))
      .mockResolvedValueOnce(jsonResponse({ auth_required: false }))
      .mockResolvedValueOnce(jsonResponse([{ id: 1, name: "Alpha" }]));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });
    expect(screen.getByTestId("projects").textContent).toBe("Alpha");
    await waitFor(() => {
      expect(connectMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.click(screen.getByRole("button", { name: "logout" }));

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });

    expect(screen.getByTestId("auth-error").textContent).toBe("");
    expect(fetchMock).toHaveBeenCalledTimes(4);
    expect(connectMock).toHaveBeenCalledTimes(2);
    expect(disconnectMock).toHaveBeenCalledTimes(1);
  });

  it("auth/status 失败时会回退到 storage token 并继续按鉴权模式启动", async () => {
    const fetchMock = vi.mocked(fetch);
    localStorage.setItem("ai-workflow-api-token", "stored-secret");
    fetchMock
      .mockRejectedValueOnce(new Error("network down"))
      .mockResolvedValueOnce(jsonResponse([{ id: 7, name: "Gamma" }]));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });

    const headers = fetchMock.mock.calls[1]?.[1]?.headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer stored-secret");
    expect(screen.getByTestId("projects").textContent).toBe("Gamma");
  });

  it("auth/status 失败且没有 token 时，仍按需要鉴权处理并报缺少 token", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockRejectedValueOnce(new Error("network down"));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("error");
    });

    expect(screen.getByTestId("auth-error").textContent).toContain("缺少访问 token");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("auth_required=false 且 URL 带 token 时，会清理 URL 但不会持久化 token，也不会带鉴权头", async () => {
    const fetchMock = vi.mocked(fetch);
    window.history.replaceState({}, "", "/?token=query-secret");
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ auth_required: false }))
      .mockResolvedValueOnce(jsonResponse([{ id: 3, name: "Anon" }]));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });

    expect(window.location.search).toBe("");
    expect(localStorage.getItem("ai-workflow-api-token")).toBeNull();
    const headers = fetchMock.mock.calls[1]?.[1]?.headers as Headers;
    expect(headers.get("Authorization")).toBeNull();
  });

  it("query token 校验失败时也会先清理 URL，不把 token 留在地址栏", async () => {
    const fetchMock = vi.mocked(fetch);
    window.history.replaceState({}, "", "/?token=query-secret");
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ auth_required: true }))
      .mockResolvedValueOnce(
        jsonResponse({ message: "bad token" }, 401),
      );

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("error");
    });

    expect(screen.getByTestId("auth-error").textContent).toContain("Token 校验失败");
    expect(window.location.search).toBe("");
    expect(localStorage.getItem("ai-workflow-api-token")).toBeNull();
    expect(String(fetchMock.mock.calls[1]?.[0])).not.toContain("token=query-secret");
  });

  it("query token 校验成功后会持久化 token，并带 Authorization 拉项目", async () => {
    const fetchMock = vi.mocked(fetch);
    window.history.replaceState({}, "", "/?token=query-secret");
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ auth_required: true }))
      .mockResolvedValueOnce(jsonResponse([{ id: 9, name: "Beta" }]));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });

    expect(window.location.search).toBe("");
    expect(localStorage.getItem("ai-workflow-api-token")).toBe("query-secret");
    const headers = fetchMock.mock.calls[1]?.[1]?.headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer query-secret");
  });

  it("auth required 且缺少 token 时会直接报错，不请求项目列表", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce(jsonResponse({ auth_required: true }));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("error");
    });

    expect(screen.getByTestId("auth-error").textContent).toContain("缺少访问 token");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("login 会从缺少 token 错误态恢复，并带 Authorization 重新拉项目", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ auth_required: true }))
      .mockResolvedValueOnce(jsonResponse({ auth_required: true }))
      .mockResolvedValueOnce(jsonResponse([{ id: 11, name: "Delta" }]));

    renderProvider();

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("error");
    });

    fireEvent.click(screen.getByRole("button", { name: "login" }));

    await waitFor(() => {
      expect(screen.getByTestId("auth-status").textContent).toBe("ready");
    });

    expect(localStorage.getItem("ai-workflow-api-token")).toBe("manual-secret");
    const headers = fetchMock.mock.calls[2]?.[1]?.headers as Headers;
    expect(headers.get("Authorization")).toBe("Bearer manual-secret");
    expect(screen.getByTestId("projects").textContent).toBe("Delta");
  });
});
