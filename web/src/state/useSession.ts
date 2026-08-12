import { useCallback, useEffect, useState } from "react";
import { fetchSession, logoutSession } from "../api/session";
import { ApiError, type SessionView } from "../api/types";

export type SessionStatus =
  | "loading"
  | "unauthenticated"
  | "authenticated"
  | "error";

export interface UseSessionOptions {
  baseUrl: string;
  fetchImpl?: typeof fetch;
}

export interface UseSessionResult {
  status: SessionStatus;
  session: SessionView | null;
  errorMessage: string | null;
  logout: () => Promise<void>;
}

export function useSession(options: UseSessionOptions): UseSessionResult {
  const { baseUrl, fetchImpl = fetch } = options;
  const [status, setStatus] = useState<SessionStatus>("loading");
  const [session, setSession] = useState<SessionView | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const view = await fetchSession(baseUrl, fetchImpl);
        if (cancelled) {
          return;
        }
        setSession(view);
        setErrorMessage(null);
        setStatus("authenticated");
      } catch (cause) {
        if (cancelled) {
          return;
        }
        if (cause instanceof ApiError && cause.status === 401) {
          setSession(null);
          setErrorMessage(null);
          setStatus("unauthenticated");
          return;
        }
        setSession(null);
        setErrorMessage(
          cause instanceof ApiError ? cause.code : "request_failed",
        );
        setStatus("error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [baseUrl, fetchImpl]);

  const logout = useCallback(async () => {
    await logoutSession(baseUrl, fetchImpl);
    setSession(null);
    setStatus("unauthenticated");
  }, [baseUrl, fetchImpl]);

  return { status, session, errorMessage, logout };
}
