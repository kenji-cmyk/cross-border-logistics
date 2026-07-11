import type { ApiErrorResponse, ApiSuccess } from "../types/api";

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
const DEFAULT_TIMEOUT_MS = 8_000;

export class ApiError extends Error {
  constructor(public readonly code: string, message: string, public readonly requestId?: string, public readonly status?: number) {
    super(message); this.name = "ApiError";
  }
}

const requestId = () => typeof crypto?.randomUUID === "function" ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
const isRecord = (value: unknown): value is Record<string, unknown> => typeof value === "object" && value !== null;

export function parseSuccessEnvelope<T>(value: unknown): ApiSuccess<T> {
  if (!isRecord(value) || !("data" in value) || !isRecord(value.meta) || typeof value.meta.requestId !== "string") throw new ApiError("INVALID_RESPONSE", "The gateway returned an unexpected response.");
  return value as unknown as ApiSuccess<T>;
}

function parseError(value: unknown, status: number): ApiError {
  const candidate = value as Partial<ApiErrorResponse>;
  if (isRecord(candidate?.error) && typeof candidate.error.code === "string" && typeof candidate.error.message === "string") return new ApiError(candidate.error.code, candidate.error.message, candidate.meta?.requestId, status);
  return new ApiError("GATEWAY_ERROR", "The request could not be completed.", undefined, status);
}

export async function apiRequest<T>(path: string, init: RequestInit = {}, timeoutMs = DEFAULT_TIMEOUT_MS): Promise<ApiSuccess<T>> {
  if (!path.startsWith("/") || path.startsWith("/internal")) throw new ApiError("INVALID_PATH", "Only public gateway paths are allowed.");
  const controller = new AbortController();
  const timer = window.setTimeout(() => controller.abort(), timeoutMs);
  const externalSignal = init.signal;
  const abort = () => controller.abort();
  externalSignal?.addEventListener("abort", abort, { once: true });
  try {
    const response = await fetch(`${API_BASE_URL.replace(/\/$/, "")}${path}`, {
      ...init, signal: controller.signal,
      headers: { "Content-Type": "application/json", "X-Request-ID": requestId(), ...init.headers },
    });
    const contentType = response.headers.get("content-type") ?? "";
    const payload: unknown = contentType.includes("application/json") ? await response.json() : null;
    if (!response.ok) throw parseError(payload, response.status);
    return parseSuccessEnvelope<T>(payload);
  } catch (error) {
    if (error instanceof ApiError) throw error;
    if (controller.signal.aborted) throw new ApiError("REQUEST_TIMEOUT", "The gateway took too long to respond.");
    throw new ApiError("NETWORK_ERROR", "The gateway is currently unavailable.");
  } finally {
    window.clearTimeout(timer); externalSignal?.removeEventListener("abort", abort);
  }
}
