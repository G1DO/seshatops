import { describe, expect, it } from "vitest";
import { fetchSnapshot } from "./client";
import { ApiError } from "./types";
import {
  NORTHSTAR_TENANT_ID,
  sampleSnapshotBefore,
} from "../fixtures/northstar";

describe("fetchSnapshot", () => {
  it("returns a validated snapshot on success", async () => {
    const snapshot = sampleSnapshotBefore();
    const fetchImpl: typeof fetch = async () =>
      new Response(JSON.stringify(snapshot), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });

    await expect(
      fetchSnapshot("http://example.test", NORTHSTAR_TENANT_ID, fetchImpl),
    ).resolves.toEqual(snapshot);
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
