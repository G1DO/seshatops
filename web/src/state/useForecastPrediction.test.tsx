import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useForecastPrediction } from "./useForecastPrediction";
import { FORBIDDEN } from "../api/types";
import {
  NORTHSTAR_ITEM_ID,
  NORTHSTAR_TENANT_ID,
  sampleForecastPrediction,
} from "../fixtures/northstar";

function Harness(props: { fetchImpl: typeof fetch }) {
  const state = useForecastPrediction({
    baseUrl: "http://api.test",
    tenantId: NORTHSTAR_TENANT_ID,
    resourceId: NORTHSTAR_ITEM_ID,
    fetchImpl: props.fetchImpl,
  });
  return (
    <div>
      <span data-testid="forecast-error">{state.errorMessage ?? ""}</span>
      <span data-testid="forecast-status">
        {state.snapshot?.freshness.status ?? ""}
      </span>
    </div>
  );
}

describe("useForecastPrediction", () => {
  it("loads a prediction independently of inventory", async () => {
    const prediction = sampleForecastPrediction();
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      expect(String(input)).toContain(
        `/v1/tenants/${NORTHSTAR_TENANT_ID}/forecast/predictions/${NORTHSTAR_ITEM_ID}`,
      );
      expect(String(input)).not.toContain("/inventory");
      return new Response(JSON.stringify(prediction), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });

    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("forecast-status")).toHaveTextContent("fresh");
    });
    expect(screen.getByTestId("forecast-error")).toHaveTextContent("");
  });

  it("maps an absent prediction to unavailable", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: "not_found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      });

    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("forecast-error")).toHaveTextContent(
        "unavailable",
      );
    });
    expect(screen.getByTestId("forecast-status")).toHaveTextContent("");
  });

  it("surfaces authorization errors without treating them as a prediction", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });

    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("forecast-error")).toHaveTextContent(FORBIDDEN);
    });
    expect(screen.getByTestId("forecast-status")).toHaveTextContent("");
  });
});
