export type Primitive = string | number | boolean;
export type HttpMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
export type HttpResponseType = "auto" | "json" | "text" | "void";
export type HttpBodyMode = "json" | "raw";

export interface HttpRequestOptions<TBody = unknown> {
  path: string;
  method?: HttpMethod;
  query?: Record<string, Primitive | null | undefined>;
  body?: TBody;
  headers?: HeadersInit;
  signal?: AbortSignal;
  responseType?: HttpResponseType;
  bodyMode?: HttpBodyMode;
  omitAuth?: boolean;
}

export interface HttpTransportOptions {
  baseUrl: string;
  getToken?: () => string | null | undefined;
  fetchImpl?: typeof fetch;
  defaultHeaders?: HeadersInit;
}

export interface HttpTransport {
  request<TResponse, TBody = unknown>(
    options: HttpRequestOptions<TBody>,
  ): Promise<TResponse>;
  buildUrl(
    path: string,
    query?: Record<string, Primitive | null | undefined>,
  ): string;
}

export class ApiError extends Error {
  status: number;
  data: unknown;

  constructor(status: number, message: string, data: unknown) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.data = data;
  }
}

export const normalizeBaseUrl = (baseUrl: string): string => {
  const trimmed = baseUrl.replace(/\/+$/, "");
  if (/^https?:\/\//.test(trimmed)) {
    return trimmed;
  }

  if (typeof window !== "undefined" && window.location?.origin) {
    return new URL(trimmed, window.location.origin)
      .toString()
      .replace(/\/+$/, "");
  }

  return new URL(trimmed, "http://localhost").toString().replace(/\/+$/, "");
};

export const buildUrl = (
  baseUrl: string,
  path: string,
  query?: Record<string, Primitive | null | undefined>,
): string => {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const url = new URL(`${baseUrl}${normalizedPath}`);
  if (query) {
    Object.entries(query).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        url.searchParams.set(key, String(value));
      }
    });
  }
  return url.toString();
};

const readResponseData = async (
  response: Response,
  responseType: HttpResponseType,
): Promise<unknown> => {
  if (responseType === "void") {
    return undefined;
  }

  const text = await response.text();
  if (!text) {
    return undefined;
  }

  if (responseType === "text") {
    return text;
  }

  if (responseType === "json") {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }

  const contentType = response.headers.get("content-type") ?? "";
  if (contentType.toLowerCase().includes("application/json")) {
    try {
      return JSON.parse(text);
    } catch {
      return text;
    }
  }
  return text;
};

const extractErrorMessage = (status: number, data: unknown): string => {
  if (data && typeof data === "object") {
    const maybeMessage = (data as { message?: unknown }).message;
    if (typeof maybeMessage === "string" && maybeMessage.trim().length > 0) {
      return maybeMessage;
    }
    const maybeError = (data as { error?: unknown }).error;
    if (typeof maybeError === "string" && maybeError.trim().length > 0) {
      return maybeError;
    }
  }
  return `Request failed with status ${status}`;
};

export const createHttpTransport = (
  options: HttpTransportOptions,
): HttpTransport => {
  const fetchImpl = options.fetchImpl ?? fetch;
  const normalizedBaseUrl = normalizeBaseUrl(options.baseUrl);

  const request = async <TResponse, TBody = unknown>(
    requestOptions: HttpRequestOptions<TBody>,
  ): Promise<TResponse> => {
    const headers = new Headers(options.defaultHeaders);
    const requestHeaders = new Headers(requestOptions.headers);
    requestHeaders.forEach((value, key) => {
      headers.set(key, value);
    });

    if (!requestOptions.omitAuth) {
      const token = options.getToken?.();
      if (token) {
        headers.set("Authorization", `Bearer ${token}`);
      }
    }

    let body: BodyInit | undefined;
    if (requestOptions.body !== undefined) {
      if (requestOptions.bodyMode === "raw") {
        body = requestOptions.body as BodyInit;
      } else {
        if (!headers.has("Content-Type")) {
          headers.set("Content-Type", "application/json");
        }
        body = JSON.stringify(requestOptions.body);
      }
    }

    const response = await fetchImpl(
      buildUrl(normalizedBaseUrl, requestOptions.path, requestOptions.query),
      {
        method: requestOptions.method ?? "GET",
        headers,
        body,
        signal: requestOptions.signal,
      },
    );

    const resolvedResponseType = response.ok
      ? (requestOptions.responseType ?? "auto")
      : "auto";
    const data = await readResponseData(response, resolvedResponseType);

    if (!response.ok) {
      throw new ApiError(
        response.status,
        extractErrorMessage(response.status, data),
        data,
      );
    }

    return data as TResponse;
  };

  return {
    request,
    buildUrl: (
      path: string,
      query?: Record<string, Primitive | null | undefined>,
    ) => buildUrl(normalizedBaseUrl, path, query),
  };
};
