import { useCallback, useEffect, useRef, useState } from "react";
import { fetchSnapshot, streamUrl } from "../api/client";
import {
  openProjectionStream,
  type EventSourceFactory,
} from "../api/sse";
import { ApiError } from "../api/types";
import type { ConnectionState } from "./connection";
import {
  applySnapshot,
  applySseUpdate,
  emptyProjectionView,
  type ProjectionViewState,
} from "./projectionStore";

export interface UseInventoryProjectionOptions {
  baseUrl: string;
  tenantId: string;
  /** Injected for tests. */
  fetchImpl?: typeof fetch;
  createEventSource?: EventSourceFactory;
  /** Delay before REST catch-up after disconnect (ms). */
  reconnectDelayMs?: number;
}

export interface UseInventoryProjectionResult {
  connection: ConnectionState;
  projection: ProjectionViewState;
  errorMessage: string | null;
}

export function useInventoryProjection(
  options: UseInventoryProjectionOptions,
): UseInventoryProjectionResult {
  const {
    baseUrl,
    tenantId,
    fetchImpl = fetch,
    createEventSource,
    reconnectDelayMs = 250,
  } = options;

  const [connection, setConnection] = useState<ConnectionState>("loading");
  const [projection, setProjection] = useState<ProjectionViewState>(
    emptyProjectionView,
  );
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const closeStreamRef = useRef<(() => void) | null>(null);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const generationRef = useRef(0);

  const clearReconnectTimer = () => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  };

  const closeStream = () => {
    if (closeStreamRef.current) {
      closeStreamRef.current();
      closeStreamRef.current = null;
    }
  };

  const loadAndConnect = useCallback(
    async (mode: "initial" | "reconnect") => {
      const generation = ++generationRef.current;
      clearReconnectTimer();
      closeStream();

      if (mode === "initial") {
        setConnection("loading");
      } else {
        setConnection("stale");
      }
      setErrorMessage(null);

      try {
        const snapshot = await fetchSnapshot(baseUrl, tenantId, fetchImpl);
        if (generation !== generationRef.current) {
          return;
        }
        setProjection((prev) => applySnapshot(prev, snapshot));
        setErrorMessage(null);
        // Stay loading/stale until EventSource onopen — do not claim live early.

        const url = streamUrl(baseUrl, tenantId);
        closeStreamRef.current = openProjectionStream(
          url,
          {
            onOpen: () => {
              if (generation !== generationRef.current) {
                return;
              }
              setConnection("live");
            },
            onUpdate: (update) => {
              if (generation !== generationRef.current) {
                return;
              }
              setProjection((prev) => applySseUpdate(prev, update));
              setConnection("live");
            },
            onDisconnected: () => {
              if (generation !== generationRef.current) {
                return;
              }
              setConnection("disconnected");
              clearReconnectTimer();
              reconnectTimerRef.current = setTimeout(() => {
                void loadAndConnect("reconnect");
              }, reconnectDelayMs);
            },
          },
          createEventSource,
        );
      } catch (cause) {
        if (generation !== generationRef.current) {
          return;
        }
        const message =
          cause instanceof ApiError
            ? cause.code
            : "request_failed";
        setErrorMessage(message);
        setConnection("error");
      }
    },
    [baseUrl, tenantId, fetchImpl, createEventSource, reconnectDelayMs],
  );

  useEffect(() => {
    void loadAndConnect("initial");
    return () => {
      generationRef.current += 1;
      clearReconnectTimer();
      closeStream();
    };
  }, [loadAndConnect]);

  return { connection, projection, errorMessage };
}
