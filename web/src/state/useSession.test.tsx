import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useSession } from "./useSession";
import { UNAUTHENTICATED } from "../api/types";

function Harness(props: { fetchImpl: typeof fetch }) {
  const state = useSession({ baseUrl: "", fetchImpl: props.fetchImpl });
  return (
    <div>
      <span data-testid="status">{state.status}</span>
      <span data-testid="principal">{state.session?.principal_id ?? ""}</span>
    </div>
  );
}

describe("useSession", () => {
  it("exposes an authenticated principal from GET /auth/session", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(
        JSON.stringify({
          principal_id: "operator-northstar",
          subject: "operator-northstar",
          issuer: "https://idp.test",
          authenticated_at: "2026-08-12T12:00:00.000Z",
          expires_at: "2026-08-12T13:00:00.000Z",
          correlation_id: "corr-1",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("status")).toHaveTextContent("authenticated");
    });
    expect(screen.getByTestId("principal")).toHaveTextContent(
      "operator-northstar",
    );
  });

  it("treats 401 as unauthenticated rather than a projection error", async () => {
    const fetchImpl = vi.fn<typeof fetch>(
      async () =>
        new Response(JSON.stringify({ error: UNAUTHENTICATED }), {
          status: 401,
          headers: { "Content-Type": "application/json" },
        }),
    );
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated");
    });
    expect(screen.getByTestId("principal")).toHaveTextContent("");
  });

  it("treats session probe failures as an error, not signed-out", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () => {
      throw new TypeError("network");
    });
    render(<Harness fetchImpl={fetchImpl} />);
    await waitFor(() => {
      expect(screen.getByTestId("status")).toHaveTextContent("error");
    });
    expect(screen.getByTestId("principal")).toHaveTextContent("");
  });
});
