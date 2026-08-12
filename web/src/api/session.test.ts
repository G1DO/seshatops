import { describe, expect, it, vi } from "vitest";
import { fetchSession, loginUrl, logoutSession } from "./session";
import { ApiError, UNAUTHENTICATED } from "./types";

const sampleSession = {
  principal_id: "operator-northstar",
  subject: "operator-northstar",
  issuer: "https://idp.test",
  authenticated_at: "2026-08-12T12:00:00.000Z",
  expires_at: "2026-08-12T13:00:00.000Z",
  correlation_id: "corr-1",
};

describe("session client", () => {
  it("builds a same-origin login URL when the API base is empty", () => {
    expect(loginUrl("")).toBe("/auth/login");
    expect(loginUrl("http://api.test")).toBe("http://api.test/auth/login");
  });

  it("fetches presentation session fields with cookies", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify(sampleSession), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await expect(fetchSession("", fetchImpl)).resolves.toEqual(sampleSession);
    expect(fetchImpl).toHaveBeenCalledWith(
      "/auth/session",
      expect.objectContaining({
        method: "GET",
        credentials: "include",
      }),
    );
  });

  it("maps 401 to unauthenticated", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: UNAUTHENTICATED }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    await expect(fetchSession("", fetchImpl)).rejects.toMatchObject({
      name: "ApiError",
      code: UNAUTHENTICATED,
      status: 401,
    } satisfies Partial<ApiError>);
  });

  it("posts logout with cookies", async () => {
    const fetchImpl = vi.fn<typeof fetch>(
      async () => new Response(null, { status: 204 }),
    );
    await expect(logoutSession("", fetchImpl)).resolves.toBeUndefined();
    expect(fetchImpl).toHaveBeenCalledWith(
      "/auth/logout",
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
  });
});
