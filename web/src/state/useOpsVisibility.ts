import { useEffect, useState } from "react";
import { fetchOps } from "../api/client";
import { ApiError, type OpsSnapshot } from "../api/types";

export interface UseOpsVisibilityOptions {
  baseUrl: string;
  tenantId: string;
  fetchImpl?: typeof fetch;
}

export interface UseOpsVisibilityResult {
  snapshot: OpsSnapshot | null;
  errorMessage: string | null;
}

/** Independent of inventory; a 403 on one surface must not hide the other. */
export function useOpsVisibility(
  options: UseOpsVisibilityOptions,
): UseOpsVisibilityResult {
  const { baseUrl, tenantId, fetchImpl = fetch } = options;
  const [snapshot, setSnapshot] = useState<OpsSnapshot | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSnapshot(null);
    setErrorMessage(null);
    void fetchOps(baseUrl, tenantId, fetchImpl)
      .then((body) => {
        if (cancelled) {
          return;
        }
        setSnapshot(body);
        setErrorMessage(null);
      })
      .catch((cause) => {
        if (cancelled) {
          return;
        }
        setSnapshot(null);
        setErrorMessage(
          cause instanceof ApiError ? cause.code : "request_failed",
        );
      });
    return () => {
      cancelled = true;
    };
  }, [baseUrl, tenantId, fetchImpl]);

  return { snapshot, errorMessage };
}
