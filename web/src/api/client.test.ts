import { describe, expect, it, vi } from "vitest";
import { fetchOps, fetchSnapshot, postQuarantineRelease } from "./client";
import { ApiError, FORBIDDEN, UNAUTHENTICATED } from "./types";
import {
  NORTHSTAR_TENANT_ID,
  sampleOpsSnapshot,
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

describe("fetchOps", () => {
  it("returns a validated ops snapshot on success and sends cookies", async () => {
    const snapshot = sampleOpsSnapshot();
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(
      fetchOps("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).resolves.toEqual(snapshot);
    expect(fetchImpl).toHaveBeenCalledWith(
      `http://example.test/v1/tenants/${NORTHSTAR_TENANT_ID}/ops`,
      expect.objectContaining({
        method: "GET",
        credentials: "include",
      }),
    );
  });

  it("maps unauthenticated and forbidden without treating them as snapshots", async () => {
    const unauth: typeof fetch = async () =>
      new Response(JSON.stringify({ error: UNAUTHENTICATED }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      });
    await expect(
      fetchOps("http://example.test", NORTHSTAR_TENANT_ID, unauth),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: UNAUTHENTICATED,
      status: 401,
    } satisfies Partial<ApiError>);

    const forbidden: typeof fetch = async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      });
    await expect(
      fetchOps("http://example.test", NORTHSTAR_TENANT_ID, forbidden),
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
      fetchOps("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).rejects.toMatchObject({ code: "malformed_response" });
  });
});

describe("postQuarantineRelease", () => {
  it("POSTs cookies and maps forbidden without treating it as success", async () => {
    const fetchImpl = vi.fn<typeof fetch>(async () =>
      new Response(JSON.stringify({ error: FORBIDDEN }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      }),
    );
    await expect(
      postQuarantineRelease(
        "http://example.test",
        NORTHSTAR_TENANT_ID,
        "018f5d78-6e64-4f5f-bd16-8e9f7c4a2001",
        fetchImpl,
      ),
    ).rejects.toMatchObject({
      name: "ApiError",
      code: FORBIDDEN,
      status: 403,
    } satisfies Partial<ApiError>);
    expect(fetchImpl).toHaveBeenCalledWith(
      `http://example.test/v1/tenants/${NORTHSTAR_TENANT_ID}/ops/quarantine/release`,
      expect.objectContaining({
        method: "POST",
        credentials: "include",
      }),
    );
  });
});
