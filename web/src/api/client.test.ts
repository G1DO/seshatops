import { describe, expect, it, vi } from "vitest";
import { fetchSnapshot } from "./client";
import { ApiError, FORBIDDEN, UNAUTHENTICATED } from "./types";
import {
  NORTHSTAR_TENANT_ID,
  sampleSnapshotBefore,
} from "../fixtures/northstar";

describe("fetchSnapshot", () => {
  it("returns a validated snapshot on success and sends cookies", async () => {
    const snapshot = sampleSnapshotBefore();
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(
      fetchSnapshot("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).resolves.toEqual(snapshot);
    expect(fetchImpl).toHaveBeenCalledWith(
      `http://example.test/v1/tenants/${NORTHSTAR_TENANT_ID}/inventory`,
      expect.objectContaining({
        method: "GET",
        credentials: "include",
      }),
    );
  });

  it("maps API error bodies", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: "invalid_tenant_id" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      });

    await expect(
      fetchSnapshot("http://example.test", "bad", fetchImpl),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: "invalid_tenant_id",
      status: 400,
    } satisfies Partial<ApiError>);
  });

  it("maps unauthenticated without treating it as a snapshot failure", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: UNAUTHENTICATED }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });

    await expect(
      fetchSnapshot("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: UNAUTHENTICATED,
      status: 401,
    } satisfies Partial<ApiError>);
  });

  it("maps forbidden without treating it as a snapshot", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });

    await expect(
      fetchSnapshot("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: FORBIDDEN,
      status: 403,
    } satisfies Partial<ApiError>);
  });

  it("rejects malformed success bodies", async () => {
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify({ tenant_id: "x" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });

    await expect(
      fetchSnapshot("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).rejects.toMatchObject({ code: "malformed_response" });
  });
});
