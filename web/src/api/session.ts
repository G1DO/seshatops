import { ApiError, UNAUTHENTICATED, type ErrorBody, type SessionView } from "./types";

function errorCodeFromBody(body: unknown): string | undefined {
  if (body === null || typeof body !== "object") {
    return undefined;
  }
  const err = (body as ErrorBody).error;
  return typeof err === "string" && err.length > 0 ? err : undefined;
}

function isSessionView(value: unknown): value is SessionView {
  if (value === null || typeof value !== "object") {
    return false;
  }
  const row = value as Record<string, unknown>;
  return (
    typeof row.principal_id === "string" &&
    typeof row.subject === "string" &&
    typeof row.issuer === "string" &&
    typeof row.authenticated_at === "string" &&
    typeof row.expires_at === "string" &&
    typeof row.correlation_id === "string"
  );
}

function rootUrl(baseUrl: string): string {
  return baseUrl.replace(/\/$/, "");
}

/** Same-origin `/auth/login` when baseUrl is empty (Vite proxy). */
export function loginUrl(baseUrl: string): string {
  const root = rootUrl(baseUrl);
  return `${root}/auth/login`;
}

export async function fetchSession(
  baseUrl: string,
  fetchImpl: typeof fetch = fetch,
): Promise<SessionView> {
  const url = `${rootUrl(baseUrl)}/auth/session`;
  let response: Response;
  try {
    response = await fetchImpl(url, {
      method: "GET",
      headers: { Accept: "application/json" },
      credentials: "include",
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  let body: unknown;
  try {
    body = await response.json();
  } catch {
    throw new ApiError(response.status, "malformed_response");
  }

  if (response.status === 401) {
    throw new ApiError(401, errorCodeFromBody(body) ?? UNAUTHENTICATED);
  }
  if (!response.ok) {
    throw new ApiError(
      response.status,
      errorCodeFromBody(body) ?? "request_failed",
    );
  }
  if (!isSessionView(body)) {
    throw new ApiError(response.status, "malformed_response");
  }
  return body;
}

export async function logoutSession(
  baseUrl: string,
  fetchImpl: typeof fetch = fetch,
): Promise<void> {
  const url = `${rootUrl(baseUrl)}/auth/logout`;
  let response: Response;
  try {
    response = await fetchImpl(url, {
      method: "POST",
      credentials: "include",
    });
  } catch {
    throw new ApiError(0, "network_error");
  }
  if (response.status === 204) {
    return;
  }
  throw new ApiError(response.status, "request_failed");
}
