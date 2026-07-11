import { describe, expect, it } from "vitest";
import { ApiError, parseSuccessEnvelope } from "./api";
describe("API envelope parsing", () => {
  it("returns a typed success envelope", () => expect(parseSuccessEnvelope<{ ok: boolean }>({ data: { ok: true }, meta: { requestId: "req-1" } }).data.ok).toBe(true));
  it("rejects malformed envelopes", () => expect(() => parseSuccessEnvelope({ data: {} })).toThrow(ApiError));
});
