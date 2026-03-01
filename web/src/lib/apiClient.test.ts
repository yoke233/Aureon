import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiError, createApiClient } from "./apiClient";

describe("apiClient", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("会在请求头注入 Bearer token 并返回 JSON", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
      getToken: () => "secret-token",
    });

    const result = await client.request<{ ok: boolean }>({
      path: "/projects",
    });

    expect(result.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledOnce();
    const call = fetchMock.mock.calls[0];
    expect(call?.[0]).toBe("http://localhost:8080/api/v1/projects");

    const requestInit = call?.[1];
    const headers = requestInit?.headers;
    expect(headers).toBeInstanceOf(Headers);
    expect((headers as Headers).get("Authorization")).toBe("Bearer secret-token");
  });

  it("当响应非 2xx 时抛出 ApiError", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ message: "bad request" }), {
          status: 400,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
      getToken: () => "",
    });

    await expect(client.request({ path: "/projects" })).rejects.toBeInstanceOf(
      ApiError,
    );
  });

  it("listPlans/listPipelines 会透传 limit 与 offset 查询参数", async () => {
    const fetchMock = vi.fn().mockImplementation(async () => {
      return new Response(JSON.stringify({ items: [], total: 0, offset: 0 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.listPlans("proj-1", { limit: 50, offset: 100 });
    await client.listPipelines("proj-1", { limit: 20, offset: 40 });

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/plans?limit=50&offset=100",
    );
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "http://localhost:8080/api/v1/projects/proj-1/pipelines?limit=20&offset=40",
    );
  });

  it("createProject 支持 github 字段并不包含多余字段", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "p1" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const client = createApiClient({
      baseUrl: "http://localhost:8080/api/v1",
    });

    await client.createProject({
      name: "proj",
      repo_path: "D:/repo/proj",
      github: {
        owner: "acme",
        repo: "repo",
      },
    });

    const requestInit = fetchMock.mock.calls[0]?.[1] as RequestInit;
    const parsedBody = JSON.parse(String(requestInit.body)) as Record<string, unknown>;
    expect(parsedBody).toEqual({
      name: "proj",
      repo_path: "D:/repo/proj",
      github: {
        owner: "acme",
        repo: "repo",
      },
    });
    expect(parsedBody).not.toHaveProperty("config");
  });
});
