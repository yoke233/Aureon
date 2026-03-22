import { readStoredApiToken } from "./authToken";
import { ApiError, createHttpTransport } from "./httpTransport";

const API_BASE_URL =
  import.meta.env.VITE_API_BASE_URL ||
  "/api";

const apiTransport = createHttpTransport({
  baseUrl: API_BASE_URL,
  getToken: readStoredApiToken,
});

const assetTransport = createHttpTransport({
  baseUrl: "/",
});

export interface UserThemeListItem {
  id: string;
  name: string;
  type: "dark" | "light";
  folder: string;
  created_at: string;
}

export interface BundledThemeManifestEntry {
  id: string;
  name: string;
  type: "dark" | "light";
  folder: string;
  description: string;
}

export interface SaveThemeRequest {
  id: string;
  name: string;
  type: "dark" | "light";
  data: unknown;
}

const isNotFoundError = (error: unknown): boolean =>
  error instanceof ApiError && error.status === 404;

export async function listUserThemes(): Promise<UserThemeListItem[]> {
  try {
    const data = await apiTransport.request<UserThemeListItem[]>({
      path: "/themes",
    });
    return Array.isArray(data) ? data : [];
  } catch {
    return [];
  }
}

export async function getUserTheme(id: string): Promise<string | null> {
  try {
    const data = await apiTransport.request<string>({
      path: `/themes/${id}`,
      responseType: "text",
    });
    return typeof data === "string" ? data : null;
  } catch (error) {
    if (isNotFoundError(error)) {
      return null;
    }
    return null;
  }
}

export async function saveUserTheme(req: SaveThemeRequest): Promise<boolean> {
  try {
    await apiTransport.request<void, SaveThemeRequest>({
      path: "/themes",
      method: "POST",
      body: req,
      responseType: "void",
    });
    return true;
  } catch {
    return false;
  }
}

export async function deleteUserTheme(id: string): Promise<boolean> {
  try {
    await apiTransport.request<void>({
      path: `/themes/${id}`,
      method: "DELETE",
      responseType: "void",
    });
    return true;
  } catch (error) {
    return isNotFoundError(error);
  }
}

export async function listBundledThemes(): Promise<BundledThemeManifestEntry[]> {
  try {
    const data = await assetTransport.request<{ themes?: BundledThemeManifestEntry[] }>({
      path: "/themes/manifest.json",
      responseType: "json",
    });
    return Array.isArray(data?.themes) ? data.themes : [];
  } catch {
    return [];
  }
}

export async function getBundledTheme(folder: string): Promise<string | null> {
  try {
    const data = await assetTransport.request<string>({
      path: `/themes/${folder}/theme.json`,
      responseType: "text",
    });
    return typeof data === "string" ? data : null;
  } catch {
    return null;
  }
}
