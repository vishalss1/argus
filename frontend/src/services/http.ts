import type { ApiErrorBody } from "../types/api";

const envBase = import.meta.env.VITE_API_BASE_URL as string | undefined;
const envWebSocketURL = import.meta.env.VITE_WS_URL as string | undefined;
export const API_BASE_URL = envBase?.replace(/\/$/, "") || "/api";

export function websocketURL(path = "/api/ws") {
  if (envWebSocketURL) return envWebSocketURL;
  if (import.meta.env.DEV && API_BASE_URL === "/api") {
    // Use the Vite proxy for WebSockets in dev mode
    const host = window.location.host; // e.g. localhost:5173
    return `ws://${host}/api/ws`;
  }

  // Use the current page's host as the default for WebSocket connections.
  // This ensures it works when accessing the server via IP on the local network.
  const host = typeof window !== "undefined" ? window.location.host : "localhost:8080";
  const protocol = typeof window !== "undefined" && window.location.protocol === "https:" ? "wss:" : "ws:";

  if (!envBase || !envBase.startsWith("http")) {
    return `${protocol}//${host}${path}`;
  }

  const apiURL = new URL(API_BASE_URL);
  apiURL.protocol = apiURL.protocol === "https:" ? "wss:" : "ws:";
  apiURL.pathname = path;
  apiURL.search = "";
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

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const hasBody = options.body !== undefined;

  if (hasBody && !(options.body instanceof FormData) && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers
  });

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
