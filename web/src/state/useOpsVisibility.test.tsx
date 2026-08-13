import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useOpsVisibility } from "./useOpsVisibility";
import { FORBIDDEN } from "../api/types";
import {
  NORTHSTAR_TENANT_ID,
  sampleOpsSnapshot,
} from "../fixtures/northstar";

function Harness(props: { fetchImpl: typeof fetch }) {
  const state = useOpsVisibility({
    baseUrl: "http://api.test",
    tenantId: NORTHSTAR_TENANT_ID,
    fetchImpl: props.fetchImpl,
  });
  return (
    <div>
      <span data-testid="ops-error">{state.errorMessage ?? ""}</span>
      <span data-testid="ops-pending">
        {state.snapshot ? String(state.snapshot.backlog.pending) : ""}
      </span>
    </div>
  );
}

describe("useOpsVisibility", () => {
  it("loads an ops snapshot independently of inventory", async () => {
    const snapshot = sampleOpsSnapshot();
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      expect(String(input)).toContain(
        `/v1/tenants/${NORTHSTAR_TENANT_ID}/ops`,
      );
      expect(String(input)).not.toContain("/inventory");
      return new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    });
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("ops-pending")).toHaveTextContent("1");
    });
    expect(screen.getByTestId("ops-error")).toHaveTextContent("");
  });

  it("surfaces forbidden without treating it as a snapshot", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("ops-error")).toHaveTextContent(FORBIDDEN);
    });
    expect(screen.getByTestId("ops-pending")).toHaveTextContent("");
  });
});
