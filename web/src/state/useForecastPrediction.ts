import { useEffect, useState } from "react";
import { fetchForecastPrediction } from "../api/client";
import {
  ApiError,
  type ForecastPredictionSnapshot,
} from "../api/types";

export interface UseForecastPredictionOptions {
  baseUrl: string;
  tenantId: string;
  resourceId: string;
  fetchImpl?: typeof fetch;
}

export interface UseForecastPredictionResult {
  snapshot: ForecastPredictionSnapshot | null;
  errorMessage: string | null;
}

/** Independent of inventory/ops; Go owns authorization and freshness. */
export function useForecastPrediction(
  options: UseForecastPredictionOptions,
): UseForecastPredictionResult {
  const { baseUrl, tenantId, resourceId, fetchImpl = fetch } = options;
  const [snapshot, setSnapshot] = useState<ForecastPredictionSnapshot | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setSnapshot(null);
    setErrorMessage(null);
    if (resourceId === "") {
      return () => {
        cancelled = true;
      };
    }
    void fetchForecastPrediction(baseUrl, tenantId, resourceId, fetchImpl)
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
          cause instanceof ApiError && cause.code === "not_found"
            ? "unavailable"
            : cause instanceof ApiError
              ? cause.code
              : "request_failed",
        );
      });
    return () => {
      cancelled = true;
    };
  }, [baseUrl, tenantId, resourceId, fetchImpl]);

  return { snapshot, errorMessage };
}
