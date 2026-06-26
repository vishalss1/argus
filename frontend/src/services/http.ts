import type { ApiErrorBody } from "../types/api";

const envBase = import.meta.env.VITE_API_BASE_URL as string | undefined;
const envWebSocketURL = import.meta.env.VITE_WS_URL as string | undefined;
export const API_BASE_URL = envBase?.replace(/\/$/, "") || "/api";

export function websocketURL(path = "/api/ws") {
  if (envWebSocketURL) {
    return envWebSocketURL;
  }

  if (import.meta.env.DEV && API_BASE_URL === "/api") {
    const host = window.location.host;
    return `ws://${host}/api/ws`;
  }

  const host = typeof window !== "undefined" ? window.location.host : "localhost:8080";
  const protocol = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";

  if (!envBase || !envBase.startsWith("http")) {
    return `${protocol}//${host}${path}`;
  }

  const apiURL = new URL(API_BASE_URL);
  apiURL.protocol = apiURL.protocol === "https:" ? "wss:" : "ws:";
  apiURL.pathname = path;
  return apiURL.toString();
}

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

interface RequestOptions extends RequestInit {
  raw?: boolean;
}

let activeRefreshPromise: Promise<{ access_token: string }> | null = null;

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const hasBody = options.body !== undefined;

  if (hasBody && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  // Inject Authorization Bearer token
  const accessToken = localStorage.getItem("argus_access_token");
  if (accessToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  // Inject Workspace ID
  const activeWorkspaceID = localStorage.getItem("argus_active_workspace_id");
  if (activeWorkspaceID && !headers.has("X-Workspace-ID")) {
    headers.set("X-Workspace-ID", activeWorkspaceID);
  }

  let response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: "include"
  });

  // Intercept 401 Unauthorized for Token Refresh
  if (response.status === 401 && path !== "/auth/login" && path !== "/auth/refresh") {
    try {
      if (!activeRefreshPromise) {
        activeRefreshPromise = (async () => {
          try {
            const refreshResponse = await fetch(`${API_BASE_URL}/auth/refresh`, {
              method: "POST",
              credentials: "include"
            });

            if (!refreshResponse.ok) {
              throw new Error("Failed to refresh token");
            }

            const tokens = await refreshResponse.json();
            localStorage.setItem("argus_access_token", tokens.access_token);
            return tokens;
          } finally {
            activeRefreshPromise = null;
          }
        })();
      }

      const tokens = await activeRefreshPromise;

      // Retry the original request with the new access token
      headers.set("Authorization", `Bearer ${tokens.access_token}`);
      response = await fetch(`${API_BASE_URL}${path}`, {
        ...options,
        headers,
        credentials: "include"
      });
    } catch (err) {
      // Clear tokens and redirect to login
      localStorage.removeItem("argus_access_token");
      localStorage.removeItem("argus_active_workspace_id");
      
      // Only trigger redirect in browser environment
      if (typeof window !== "undefined" && window.location.pathname !== "/login" && window.location.pathname !== "/register") {
        window.location.href = "/login";
      }
      throw new ApiError("Session expired. Please log in again.", 401);
    }
  }

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    try {
      const clonedResponse = response.clone();
      const body = (await clonedResponse.json()) as ApiErrorBody;
      if (body.error) message = body.error;
    } catch {
      const text = await response.text().catch(() => "");
      if (text) message = text;
    }
    throw new ApiError(message, response.status);
  }

  if (options.raw) {
    return (await response.text()) as T;
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

export async function requestBlob(path: string, options: RequestOptions = {}): Promise<Blob> {
  const headers = new Headers(options.headers);
  const hasBody = options.body !== undefined;

  if (hasBody && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const accessToken = localStorage.getItem("argus_access_token");
  if (accessToken && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${accessToken}`);
  }

  const activeWorkspaceID = localStorage.getItem("argus_active_workspace_id");
  if (activeWorkspaceID && !headers.has("X-Workspace-ID")) {
    headers.set("X-Workspace-ID", activeWorkspaceID);
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
    credentials: "include"
  });

  if (!response.ok) {
    let message = `Request failed with status ${response.status}`;
    const text = await response.text().catch(() => "");
    if (text) message = text;
    throw new ApiError(message, response.status);
  }

  return response.blob();
}
