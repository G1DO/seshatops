import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { OperationsView } from "../ui/OperationsView";
import { useInventoryProjection } from "../state/useInventoryProjection";
import {
  NORTHSTAR_ITEM_ID,
  NORTHSTAR_TENANT_ID,
  sampleSnapshotAfter,
  sampleSnapshotBefore,
  sampleSseUpdate,
} from "../fixtures/northstar";
import {
  MockEventSource,
  mockEventSourceFactory,
} from "./mockEventSource";

function Harness(props: {
  fetchImpl: typeof fetch;
  reconnectDelayMs?: number;
}) {
  const state = useInventoryProjection({
    baseUrl: "http://api.test",
    tenantId: NORTHSTAR_TENANT_ID,
    fetchImpl: props.fetchImpl,
    createEventSource: mockEventSourceFactory,
    reconnectDelayMs: props.reconnectDelayMs ?? 20,
  });
  return (
    <OperationsView
      connection={state.connection}
      projection={state.projection}
      errorMessage={state.errorMessage}
      tenantId={NORTHSTAR_TENANT_ID}
    />
  );
}

describe("inventory projection integration", () => {
  afterEach(() => {
    MockEventSource.reset();
    vi.useRealTimers();
  });

  it("loads the initial projection exclusively through REST", async () => {
    const snapshot = sampleSnapshotBefore();
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input);
      expect(url).toContain(
        `/v1/tenants/${NORTHSTAR_TENANT_ID}/inventory`,
      );
      expect(url).not.toContain("/stream");
      return new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });

    render(<Harness fetchImpl={fetchImpl} />);

    expect(screen.getByTestId("connection-status")).toHaveAttribute(
      "data-connection",
      "loading",
    );

    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "live",
      );
    });

    expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "10",
    );
    expect(screen.getByTestId("checksum")).toHaveTextContent(snapshot.checksum);
    expect(screen.getByTestId("observed-at")).toHaveTextContent(
      snapshot.observed_at,
    );
    expect(MockEventSource.instances).toHaveLength(1);
    expect(MockEventSource.latest().url).toContain("/inventory/stream");
  });

  it("applies an SSE committed update as before/after presentation state", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify(sampleSnapshotBefore()), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "live",
      );
    });

    act(() => {
      MockEventSource.latest().emitUpdate(sampleSseUpdate());
    });

    await waitFor(() => {
      expect(screen.getByTestId(`before-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
        "10",
      );
      expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
        "8",
      );
    });
    expect(screen.getByTestId("last-applied-event")).toHaveTextContent(
      sampleSseUpdate().last_applied_event_id,
    );
  });

  it("reconnects via REST catch-up after SSE disconnect", async () => {
    let snapshotCalls = 0;
    const fetchImpl = vi.fn<typeof fetch>(async () => {
      snapshotCalls += 1;
      const body =
        snapshotCalls === 1 ? sampleSnapshotBefore() : sampleSnapshotAfter();
      return new Response(JSON.stringify(body), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });

    render(<Harness fetchImpl={fetchImpl} reconnectDelayMs={10} />);
    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "live",
      );
    });

    act(() => {
      MockEventSource.latest().emitUpdate(sampleSseUpdate());
    });
    await waitFor(() => {
      expect(screen.getByTestId("last-applied-event")).toHaveTextContent(
        sampleSseUpdate().last_applied_event_id,
      );
    });

    act(() => {
      MockEventSource.latest().triggerError();
    });

    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "disconnected",
      );
    });

    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "live",
      );
      expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
        "8",
      );
    });

    expect(snapshotCalls).toBeGreaterThanOrEqual(2);
    expect(MockEventSource.instances.length).toBeGreaterThanOrEqual(2);
    expect(screen.getByTestId(`before-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "10",
    );
    expect(screen.getByTestId("last-applied-event")).toHaveTextContent("—");
  });

  it("shows an explicit error when the initial REST load fails", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify({ error: "snapshot_failed" }), {
        status: 500,
        headers: { "Content-Type": "application/json" },
      }),
    );

    render(<Harness fetchImpl={fetchImpl} />);

    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "error",
      );
    });
    expect(screen.getByTestId("error-banner")).toHaveTextContent(
      "snapshot_failed",
    );
    expect(MockEventSource.instances).toHaveLength(0);
  });

  it("keeps duplicate SSE updates from inventing divergent business state", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify(sampleSnapshotBefore()), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("connection-status")).toHaveAttribute(
        "data-connection",
        "live",
      );
    });

    act(() => {
      const source = MockEventSource.latest();
      source.emitUpdate(sampleSseUpdate());
      source.emitUpdate(sampleSseUpdate());
    });

    await waitFor(() => {
      expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
        "8",
      );
    });
    expect(screen.getByTestId(`before-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "10",
    );
    expect(screen.getAllByTestId(`item-${NORTHSTAR_ITEM_ID}`)).toHaveLength(1);
  });
});
