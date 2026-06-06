import type { ApiErrorBody } from "../types/api";

const envBase = import.meta.env.VITE_API_BASE_URL as string | undefined;
const envWebSocketURL = import.meta.env.VITE_WS_URL as string | undefined;
export const API_BASE_URL = envBase?.replace(/\/$/, "") || "/api";

export function websocketURL(path = "/api/ws") {
  const token = localStorage.getItem("argus_access_token") || "";
  const workspaceId = localStorage.getItem("argus_active_workspace_id") || "";

  const appendParams = (urlStr: string) => {
    try {
      const urlObj = urlStr.startsWith("ws:") || urlStr.startsWith("wss:") 
        ? new URL(urlStr) 
        : new URL(urlStr, window.location.origin);
      if (token) urlObj.searchParams.set("token", token);
      if (workspaceId) urlObj.searchParams.set("workspace_id", workspaceId);
      return urlStr.startsWith("ws:") || urlStr.startsWith("wss:") 
        ? urlObj.toString() 
        : urlObj.pathname + urlObj.search;
    } catch {
      return urlStr;
    }
  };

  if (envWebSocketURL) {
    return appendParams(envWebSocketURL);
  }

  if (import.meta.env.DEV && API_BASE_URL === "/api") {
    // Use the Vite proxy for WebSockets in dev mode
    const host = window.location.host; // e.g. localhost:5173
    const finalUrl = `ws://${host}/api/ws`;
    return appendParams(finalUrl);
  }

  // Use the current page's host as the default for WebSocket connections.
  // This ensures it works when accessing the server via IP on the local network.
  const host = typeof window !== "undefined" ? window.location.host : "localhost:8080";
  const protocol = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";

  if (!envBase || !envBase.startsWith("http")) {
    const finalUrl = `${protocol}//${host}${path}`;
    return appendParams(finalUrl);
  }

  const apiURL = new URL(API_BASE_URL);
  apiURL.protocol = apiURL.protocol === "https:" ? "wss:" : "ws:";
  apiURL.pathname = path;
  return appendParams(apiURL.toString());
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
    headers
  });

  // Intercept 401 Unauthorized for Token Refresh
  if (response.status === 401 && path !== "/auth/login" && path !== "/auth/refresh") {
    try {
      const refreshToken = localStorage.getItem("argus_refresh_token");
      if (!refreshToken) {
        throw new Error("No refresh token available");
      }

      const refreshResponse = await fetch(`${API_BASE_URL}/auth/refresh`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({ refresh_token: refreshToken })
      });

      if (!refreshResponse.ok) {
        throw new Error("Failed to refresh token");
      }

      const tokens = await refreshResponse.json();
      localStorage.setItem("argus_access_token", tokens.access_token);
      localStorage.setItem("argus_refresh_token", tokens.refresh_token);

      // Retry the original request with the new access token
      headers.set("Authorization", `Bearer ${tokens.access_token}`);
      response = await fetch(`${API_BASE_URL}${path}`, {
        ...options,
        headers
      });
    } catch (err) {
      // Clear tokens and redirect to login
      localStorage.removeItem("argus_access_token");
      localStorage.removeItem("argus_refresh_token");
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
      const body = (await response.json()) as ApiErrorBody;
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
