const TOKEN_STORAGE_KEY = "ai-workflow-api-token";

export const readStoredApiToken = (): string | null => {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(TOKEN_STORAGE_KEY);
  if (!raw) {
    return null;
  }
  const token = raw.trim();
  return token.length > 0 ? token : null;
};

export const persistStoredApiToken = (token: string): void => {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.setItem(TOKEN_STORAGE_KEY, token);
};

export const clearStoredApiToken = (): void => {
  if (typeof window === "undefined") {
    return;
  }
  window.localStorage.removeItem(TOKEN_STORAGE_KEY);
};
