import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OperationsView } from "./OperationsView";
import { NORTHSTAR_ITEM_ID, NORTHSTAR_TENANT_ID } from "../fixtures/northstar";

describe("OperationsView", () => {
  it("renders loading without claiming live currency", () => {
    render(
      <OperationsView
        connection="loading"
        projection={{
          items: [],
          checksum: "",
          observed_at: "",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
      />,
    );
    const status = screen.getByTestId("connection-status");
    expect(status).toHaveAttribute("data-connection", "loading");
    expect(status).toHaveAttribute("data-authoritative-live", "false");
  });

  it("renders success inventory and freshness metadata", () => {
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [
            {
              item_id: NORTHSTAR_ITEM_ID,
              quantity_on_hand: 8,
              aggregate_version: 1,
              previous_quantity_on_hand: 10,
            },
          ],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: "evt-1",
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
      />,
    );
    expect(screen.getByTestId("connection-status")).toHaveAttribute(
      "data-authoritative-live",
      "true",
    );
    expect(screen.getByTestId(`before-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "10",
    );
    expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "8",
    );
    expect(screen.getByTestId("checksum")).toHaveTextContent("abc");
    expect(screen.getByTestId("observed-at")).toHaveTextContent(
      "2026-08-12T07:00:00Z",
    );
    expect(screen.getByTestId("last-applied-event")).toHaveTextContent("evt-1");
  });

  it("renders explicit error, stale, and disconnected states as non-live", () => {
    const { rerender } = render(
      <OperationsView
        connection="error"
        projection={{
          items: [],
          checksum: "",
          observed_at: "",
          last_applied_event_id: null,
        }}
        errorMessage="snapshot_failed"
        tenantId={NORTHSTAR_TENANT_ID}
      />,
    );
    expect(screen.getByTestId("error-banner")).toHaveTextContent(
      "snapshot_failed",
    );
    expect(screen.getByTestId("connection-status")).toHaveAttribute(
      "data-authoritative-live",
      "false",
    );

    rerender(
      <OperationsView
        connection="stale"
        projection={{
          items: [
            {
              item_id: NORTHSTAR_ITEM_ID,
              quantity_on_hand: 8,
              aggregate_version: 1,
              previous_quantity_on_hand: 10,
            },
          ],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
      />,
    );
    let status = screen.getByTestId("connection-status");
    expect(status).toHaveAttribute("data-connection", "stale");
    expect(status).toHaveAttribute("data-authoritative-live", "false");
    expect(status).toHaveTextContent(/not guaranteed current/i);

    rerender(
      <OperationsView
        connection="disconnected"
        projection={{
          items: [
            {
              item_id: NORTHSTAR_ITEM_ID,
              quantity_on_hand: 8,
              aggregate_version: 1,
              previous_quantity_on_hand: 10,
            },
          ],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: "evt-1",
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
      />,
    );
    status = screen.getByTestId("connection-status");
    expect(status).toHaveAttribute("data-connection", "disconnected");
    expect(status).toHaveAttribute("data-authoritative-live", "false");
    expect(status).toHaveTextContent(/not guaranteed current/i);
  });
});
