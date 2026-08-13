import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useOpsControls } from "./useOpsControls";
import { FORBIDDEN } from "../api/types";
import { NORTHSTAR_TENANT_ID } from "../fixtures/northstar";

function Harness(props: { fetchImpl: typeof fetch }) {
  const state = useOpsControls({
    baseUrl: "http://api.test",
    tenantId: NORTHSTAR_TENANT_ID,
    fetchImpl: props.fetchImpl,
  });
  return (
    <div>
      <span data-testid="ctrl-error">{state.errorMessage ?? ""}</span>
      <span data-testid="ctrl-status">{state.result?.status ?? ""}</span>
      <button type="button" onClick={() => void state.release("evt-1")}>
        release
      </button>
    </div>
  );
}

describe("useOpsControls", () => {
  it("posts release with cookies and records a 403 as an error", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () => {
      return new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });
    });
    render(<Harness fetchImpl={fetchImpl} />);
    fireEvent.click(screen.getByText("release"));
    await waitFor(() => {
      expect(screen.getByTestId("ctrl-error")).toHaveTextContent(FORBIDDEN);
    });
    expect(fetchImpl).toHaveBeenCalledWith(
      `http://api.test/v1/tenants/${NORTHSTAR_TENANT_ID}/ops/quarantine/release`,
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
  });
});
