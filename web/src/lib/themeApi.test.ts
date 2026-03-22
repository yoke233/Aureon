/** @vitest-environment jsdom */
import { beforeEach, describe, expect, it, vi } from "vitest";

describe("themeApi", () => {
  beforeEach(() => {
    vi.resetModules();
    localStorage.clear();
    vi.stubGlobal("fetch", vi.fn());
  });

  it("uses shared token storage for API theme requests", async () => {
    localStorage.setItem("ai-workflow-api-token", "secret-token");
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    const { listUserThemes } = await import("./themeApi");
    await listUserThemes();

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toMatch(/\/api\/themes$/);
    expect(init?.headers).toBeInstanceOf(Headers);
    expect((init?.headers as Headers).get("Authorization")).toBe("Bearer secret-token");
  });

  it("uses asset transport for bundled theme files without auth header", async () => {
    const fetchMock = vi.mocked(fetch);
    fetchMock.mockResolvedValueOnce(
      new Response('{"name":"Ocean","type":"dark","colors":{"editor.background":"#000000"}}', {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    const { getBundledTheme } = await import("./themeApi");
    const result = await getBundledTheme("ocean");

    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toMatch(/\/themes\/ocean\/theme\.json$/);
    expect((init?.headers as Headers).get("Authorization")).toBeNull();
    expect(result).toContain("\"Ocean\"");
  });
});
