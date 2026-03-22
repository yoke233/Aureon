/** @vitest-environment jsdom */
import { describe, it, expect, vi, beforeEach } from "vitest";

// Mock the themeApi module so tests don't make real HTTP calls
vi.mock("@/lib/themeApi", () => ({
  listUserThemes: vi.fn().mockResolvedValue([]),
  getUserTheme: vi.fn().mockResolvedValue(null),
  saveUserTheme: vi.fn().mockResolvedValue(true),
  deleteUserTheme: vi.fn().mockResolvedValue(true),
  listBundledThemes: vi.fn().mockResolvedValue([]),
  getBundledTheme: vi.fn().mockResolvedValue(null),
}));

describe("settingsStore", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.resetModules();
  });

  it("defaults to slate theme and md font size", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    const state = useSettingsStore.getState();
    expect(state.theme).toBe("slate");
    expect(state.fontSize).toBe("md");
  });

  it("setTheme updates theme and persists to localStorage", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    useSettingsStore.getState().setTheme("ocean");
    expect(useSettingsStore.getState().theme).toBe("ocean");
    const saved = JSON.parse(localStorage.getItem("ai-workflow-settings") ?? "{}");
    expect(saved.theme).toBe("ocean");
  });

  it("setFontSize updates fontSize and persists to localStorage", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    useSettingsStore.getState().setFontSize("lg");
    expect(useSettingsStore.getState().fontSize).toBe("lg");
    const saved = JSON.parse(localStorage.getItem("ai-workflow-settings") ?? "{}");
    expect(saved.fontSize).toBe("lg");
  });

  it("reads persisted values from localStorage on init", async () => {
    localStorage.setItem("ai-workflow-settings", JSON.stringify({ theme: "amber", fontSize: "sm" }));
    const { useSettingsStore } = await import("./settingsStore");
    expect(useSettingsStore.getState().theme).toBe("amber");
    expect(useSettingsStore.getState().fontSize).toBe("sm");
  });

  it("addCustomTheme saves to backend and activates", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    const { saveUserTheme } = await import("@/lib/themeApi");

    const rawJson = JSON.stringify({ name: "One Dark", type: "dark", colors: {} });
    await useSettingsStore.getState().addCustomTheme(
      {
        id: "vsc-one-dark",
        name: "One Dark",
        type: "dark",
        cssVars: { "--background": "220 13% 18%" },
        previewColors: {
          background: "#282c34",
          foreground: "#abb2bf",
          primary: "#61afef",
          accent: "#528bff",
          border: "#3e4451",
        },
      },
      rawJson,
    );

    expect(useSettingsStore.getState().theme).toBe("vsc-one-dark");
    expect(useSettingsStore.getState().userThemeCache["vsc-one-dark"]).toBeDefined();
    expect(useSettingsStore.getState().userThemeEntries).toHaveLength(1);
    expect(saveUserTheme).toHaveBeenCalledWith(
      expect.objectContaining({ id: "vsc-one-dark", name: "One Dark" }),
    );
  });

  it("removeCustomTheme calls API and falls back to slate if active", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    const { deleteUserTheme } = await import("@/lib/themeApi");

    const rawJson = JSON.stringify({ name: "Monokai", type: "dark", colors: {} });
    await useSettingsStore.getState().addCustomTheme(
      {
        id: "vsc-monokai",
        name: "Monokai",
        type: "dark",
        cssVars: { "--background": "70 8% 15%" },
        previewColors: {
          background: "#272822",
          foreground: "#f8f8f2",
          primary: "#a6e22e",
          accent: "#66d9ef",
          border: "#3e3d32",
        },
      },
      rawJson,
    );
    expect(useSettingsStore.getState().theme).toBe("vsc-monokai");

    await useSettingsStore.getState().removeCustomTheme("vsc-monokai");
    expect(useSettingsStore.getState().userThemeCache["vsc-monokai"]).toBeUndefined();
    expect(useSettingsStore.getState().theme).toBe("slate");
    expect(deleteUserTheme).toHaveBeenCalledWith("vsc-monokai");
  });

  it("restores custom theme id from localStorage on init", async () => {
    localStorage.setItem("ai-workflow-settings", JSON.stringify({ theme: "vsc-test", fontSize: "md" }));
    const { useSettingsStore } = await import("./settingsStore");
    expect(useSettingsStore.getState().theme).toBe("vsc-test");
  });

  it("loadBundledManifest delegates to themeApi", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    const { listBundledThemes } = await import("@/lib/themeApi");
    vi.mocked(listBundledThemes).mockResolvedValueOnce([
      {
        id: "twilight",
        name: "Twilight",
        type: "dark",
        folder: "twilight",
        description: "Bundled",
      },
    ]);

    await useSettingsStore.getState().loadBundledManifest();

    expect(listBundledThemes).toHaveBeenCalledTimes(1);
    expect(useSettingsStore.getState().bundledThemes).toEqual([
      expect.objectContaining({ id: "twilight", folder: "twilight" }),
    ]);
  });

  it("activateBundledTheme reads theme content through themeApi", async () => {
    const { useSettingsStore } = await import("./settingsStore");
    const { getBundledTheme } = await import("@/lib/themeApi");
    useSettingsStore.setState({
      bundledThemes: [
        {
          id: "twilight",
          name: "Twilight",
          type: "dark",
          folder: "twilight",
          description: "Bundled",
        },
      ],
    });
    vi.mocked(getBundledTheme).mockResolvedValueOnce(
      JSON.stringify({
        name: "Twilight",
        type: "dark",
        colors: {
          "editor.background": "#1e1e1e",
          "editor.foreground": "#d4d4d4",
          "button.background": "#007acc",
          "focusBorder": "#007acc",
          "panel.border": "#444444",
        },
      }),
    );

    await useSettingsStore.getState().activateBundledTheme("twilight");

    expect(getBundledTheme).toHaveBeenCalledWith("twilight");
    expect(useSettingsStore.getState().theme).toBe("twilight");
    expect(useSettingsStore.getState().bundledThemeCache.twilight).toBeDefined();
  });
});
