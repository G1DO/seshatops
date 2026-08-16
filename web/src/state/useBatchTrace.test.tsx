import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useBatchTrace } from "./useBatchTrace";
import { FORBIDDEN } from "../api/types";
import {
  NORTHSTAR_BATCH_ID,
  NORTHSTAR_TENANT_ID,
  sampleBatchTrace,
} from "../fixtures/northstar";

function Harness(props: { fetchImpl: typeof fetch; batchId?: string }) {
  const state = useBatchTrace({
    baseUrl: "http://api.test",
    tenantId: NORTHSTAR_TENANT_ID,
    batchId: props.batchId ?? NORTHSTAR_BATCH_ID,
    fetchImpl: props.fetchImpl,
  });
  return (
    <div>
      <span data-testid="lineage-error">{state.errorMessage ?? ""}</span>
      <span data-testid="lineage-supplier">
        {state.snapshot ? state.snapshot.supplier.id : ""}
      </span>
    </div>
  );
}

describe("useBatchTrace", () => {
  it("loads a batch trace independently of inventory", async () => {
    const snapshot = sampleBatchTrace();
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      expect(String(input)).toContain(
        `/v1/tenants/${NORTHSTAR_TENANT_ID}/ops/lineage/batches/${NORTHSTAR_BATCH_ID}`,
      );
      expect(String(input)).not.toContain("/inventory");
      return new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("lineage-supplier")).toHaveTextContent(
        snapshot.supplier.id,
      );
    });
    expect(screen.getByTestId("lineage-error")).toHaveTextContent("");
  });

  it("surfaces forbidden and not_found without treating them as snapshots", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("lineage-error")).toHaveTextContent(FORBIDDEN);
    });
    expect(screen.getByTestId("lineage-supplier")).toHaveTextContent("");
  });
});
