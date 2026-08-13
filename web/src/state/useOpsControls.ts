import { useCallback, useState } from "react";
import {
  postQuarantineRelease,
  postRebuild,
  postReplay,
} from "../api/client";
import { ApiError, type ControlResult } from "../api/types";

export interface UseOpsControlsOptions {
  baseUrl: string;
  tenantId: string;
  fetchImpl?: typeof fetch;
}

export interface UseOpsControlsResult {
  busy: boolean;
  errorMessage: string | null;
  result: ControlResult | null;
  release: (eventId: string) => Promise<void>;
  replay: (eventId?: string) => Promise<void>;
  rebuild: () => Promise<void>;
}

/** Presentation hook. A 403 is a request error, not a grant. */
export function useOpsControls(
  options: UseOpsControlsOptions,
): UseOpsControlsResult {
  const { baseUrl, tenantId, fetchImpl = fetch } = options;
  const [busy, setBusy] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [result, setResult] = useState<ControlResult | null>(null);

  const run = useCallback(
    async (work: () => Promise<ControlResult>) => {
      setBusy(true);
      setErrorMessage(null);
      try {
        const next = await work();
        setResult(next);
      } catch (cause) {
        setResult(null);
        setErrorMessage(
          cause instanceof ApiError ? cause.code : "request_failed",
        );
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  const release = useCallback(
    (eventId: string) =>
      run(() => postQuarantineRelease(baseUrl, tenantId, eventId, fetchImpl)),
    [baseUrl, fetchImpl, run, tenantId],
  );
  const replay = useCallback(
    (eventId?: string) =>
      run(() => postReplay(baseUrl, tenantId, eventId, fetchImpl)),
    [baseUrl, fetchImpl, run, tenantId],
  );
  const rebuild = useCallback(
    () => run(() => postRebuild(baseUrl, tenantId, fetchImpl)),
    [baseUrl, fetchImpl, run, tenantId],
  );

  return { busy, errorMessage, result, release, replay, rebuild };
}
