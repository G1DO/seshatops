import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { OperationsView } from "./OperationsView";
import {
  NORTHSTAR_ITEM_ID,
  NORTHSTAR_TENANT_ID,
  sampleBatchTrace,
  sampleForecastPrediction,
} from "../fixtures/northstar";

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

  it("renders ops visibility independently of inventory", () => {
    render(
      <OperationsView
        connection="error"
        projection={{
          items: [],
          checksum: "",
          observed_at: "",
          last_applied_event_id: null,
        }}
        errorMessage="forbidden"
        tenantId={NORTHSTAR_TENANT_ID}
        ops={{
          tenant_id: NORTHSTAR_TENANT_ID,
          observed_at: "2026-08-13T06:00:00.000Z",
          projection: { checksum: "abc", item_count: 1 },
          backlog: {
            pending: 2,
            publishing: 0,
            published: 0,
            quarantined: 0,
            oldest_unpublished: "2026-08-13T05:59:00.000Z",
            quarantines: [],
          },
          processing: {
            applied: 1,
            duplicate_noop: 0,
            quarantined_conflict: 0,
            quarantined_gap: 1,
            quarantined_stale: 0,
            quarantined_invalid: 0,
            quarantined_mismatch: 0,
            quarantined_transition: 0,
            failures_retrying: 0,
            failures_quarantined: 0,
            oldest_gap: "2026-08-13T05:58:00.000Z",
            oldest_failure: null,
            failures: [],
            gaps: [],
          },
        }}
        opsError={null}
      />,
    );
    expect(screen.getByTestId("error-banner")).toHaveTextContent("forbidden");
    expect(screen.getByTestId("ops-pending")).toHaveTextContent("2");
    expect(screen.getByTestId("ops-gaps")).toHaveTextContent("1");
    expect(screen.getByTestId("ops-oldest-unpublished")).toHaveTextContent(
      "2026-08-13T05:59:00.000Z",
    );
  });

  it("renders ops forbidden without hiding inventory", () => {
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
        ops={null}
        opsError="forbidden"
      />,
    );
    expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "8",
    );
    expect(screen.getByTestId("ops-error")).toHaveTextContent("forbidden");
    expect(screen.queryByTestId("ops-pending")).toBeNull();
  });

  it("renders batch lineage and provenance", () => {
    const trace = sampleBatchTrace();
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        lineage={trace}
        lineageError={null}
        lineageBatchId={trace.batch.id}
      />,
    );
    expect(screen.getByTestId("lineage-supplier")).toHaveTextContent(
      trace.supplier.id,
    );
    expect(screen.getByTestId("lineage-lot")).toHaveTextContent(trace.lot.id);
    expect(screen.getByTestId("lineage-batch")).toHaveTextContent(trace.batch.id);
    expect(screen.getByTestId("lineage-shipment")).toHaveTextContent(
      trace.shipment.id,
    );
    expect(screen.getByTestId("lineage-order")).toHaveTextContent(
      trace.shipment.order_id,
    );
    expect(screen.getByTestId("lineage-supplier-event")).toHaveTextContent(
      trace.supplier.source_event_id,
    );
  });

  it("renders lineage forbidden without hiding inventory", () => {
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
        lineage={null}
        lineageError="forbidden"
      />,
    );
    expect(screen.getByTestId(`after-${NORTHSTAR_ITEM_ID}`)).toHaveTextContent(
      "8",
    );
    expect(screen.getByTestId("lineage-error")).toHaveTextContent("forbidden");
    expect(screen.queryByTestId("lineage-supplier")).toBeNull();
  });

  it("renders empty lineage without inventing a chain", () => {
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "",
          observed_at: "",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        lineage={null}
        lineageError={null}
      />,
    );
    expect(screen.getByTestId("lineage-empty")).toBeTruthy();
    expect(screen.queryByTestId("lineage-supplier")).toBeNull();
  });

  it("renders forecast risk, uncertainty, and lineage", () => {
    const prediction = sampleForecastPrediction();
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        forecast={prediction}
        forecastError={null}
      />,
    );
    expect(screen.getByTestId("forecast-risk")).toHaveTextContent("0.25");
    expect(screen.getByTestId("forecast-uncertainty")).toHaveTextContent(
      "0.1–0.4",
    );
    expect(screen.getByTestId("forecast-freshness")).toHaveTextContent("fresh");
    expect(screen.getByTestId("forecast-feature-snapshot")).toHaveTextContent(
      prediction.lineage.feature_snapshot_id,
    );
    expect(screen.getByTestId("forecast-predictor")).toHaveTextContent(
      prediction.lineage.predictor,
    );
    expect(screen.getByTestId("forecast-correlation-id")).toHaveTextContent(
      prediction.correlation_id,
    );
  });

  it("does not present stale or abstained results as fresh confident forecasts", () => {
    const stale = {
      ...sampleForecastPrediction(),
      freshness: { status: "stale" as const, fresh_at: "2026-08-01T07:00:00Z" },
    };
    const { rerender } = render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        forecast={stale}
        forecastError={null}
      />,
    );
    expect(screen.getByTestId("forecast-freshness")).toHaveTextContent("stale");
    expect(screen.getByText("Stored stockout risk")).toBeTruthy();
    expect(screen.queryByText("Stockout risk")).toBeNull();

    rerender(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        forecast={{
          ...sampleForecastPrediction(),
          status: "abstained",
          stockout_risk: null,
          uncertainty: null,
          abstention_reason: "insufficient_training_support",
        }}
        forecastError={null}
      />,
    );
    expect(screen.getByTestId("forecast-status")).toHaveTextContent("abstained");
    expect(screen.getByTestId("forecast-abstention-reason")).toHaveTextContent(
      "insufficient_training_support",
    );
    expect(screen.queryByTestId("forecast-risk")).toBeNull();
  });

  it("renders forecast unavailable and error states explicitly", () => {
    const { rerender } = render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        forecast={null}
        forecastError="unavailable"
      />,
    );
    expect(screen.getByTestId("forecast-unavailable")).toHaveTextContent(
      /no current stockout assessment/i,
    );

    rerender(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        forecast={null}
        forecastError="forbidden"
      />,
    );
    expect(screen.getByTestId("forecast-error")).toHaveTextContent("forbidden");
  });

  it("shows control 403 as a request error, not a grant", () => {
    const onRelease = vi.fn();
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        onRelease={onRelease}
        controlError="forbidden"
      />,
    );
    expect(screen.getByTestId("ops-controls")).toBeTruthy();
    expect(screen.getByTestId("ops-control-error")).toHaveTextContent(
      "forbidden",
    );
    const release = screen.getByTestId("ops-release") as HTMLButtonElement;
    expect(release.disabled).toBe(true);
  });

  it("renders sign-in when unauthenticated and does not show inventory", () => {
    const onSignIn = vi.fn();
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
        unauthenticated
        onSignIn={onSignIn}
      />,
    );
    expect(screen.getByTestId("sign-in-panel")).toHaveTextContent(
      /does not grant tenant or action authorization/i,
    );
    expect(screen.queryByTestId("freshness-meta")).toBeNull();
    screen.getByTestId("sign-in").click();
    expect(onSignIn).toHaveBeenCalledTimes(1);
  });

  it("presents session principal and logout without treating them as authorization", () => {
    const onLogout = vi.fn();
    render(
      <OperationsView
        connection="live"
        projection={{
          items: [],
          checksum: "abc",
          observed_at: "2026-08-12T07:00:00Z",
          last_applied_event_id: null,
        }}
        errorMessage={null}
        tenantId={NORTHSTAR_TENANT_ID}
        session={{
          principal_id: "operator-northstar",
          subject: "operator-northstar",
          issuer: "https://idp.test",
          authenticated_at: "2026-08-12T12:00:00.000Z",
          expires_at: "2026-08-12T13:00:00.000Z",
          correlation_id: "corr-1",
        }}
        onLogout={onLogout}
      />,
    );
    expect(screen.getByTestId("session-principal")).toHaveTextContent(
      "operator-northstar",
    );
    expect(screen.getByTestId("session-expires")).toHaveTextContent(
      "2026-08-12T13:00:00.000Z",
    );
    screen.getByTestId("sign-out").click();
    expect(onLogout).toHaveBeenCalledTimes(1);
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
