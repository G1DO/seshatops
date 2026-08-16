import { useEffect, useState } from "react";
import { fetchBatchTrace } from "../api/client";
import { ApiError, type BatchTraceSnapshot } from "../api/types";

export interface UseBatchTraceOptions {
  baseUrl: string;
  tenantId: string;
  batchId: string;
  fetchImpl?: typeof fetch;
}

export interface UseBatchTraceResult {
  snapshot: BatchTraceSnapshot | null;
  errorMessage: string | null;
}

/** Independent of inventory/ops; a 403 on one surface must not hide the others. */
export function useBatchTrace(options: UseBatchTraceOptions): UseBatchTraceResult {
  const { baseUrl, tenantId, batchId, fetchImpl = fetch } = options;
  const [snapshot, setSnapshot] = useState<BatchTraceSnapshot | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSnapshot(null);
    setErrorMessage(null);
    if (batchId === "") {
      return () => {
        cancelled = true;
      };
    }
    void fetchBatchTrace(baseUrl, tenantId, batchId, fetchImpl)
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
  }, [baseUrl, tenantId, batchId, fetchImpl]);

  return { snapshot, errorMessage };
}
