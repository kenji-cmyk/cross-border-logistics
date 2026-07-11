import { describe, expect, it } from "vitest";
import { calculateVideoOpacity } from "./useVideoFadeLoop";
describe("video opacity", () => {
  it("fades in and out", () => { expect(calculateVideoOpacity(.25, 10)).toBe(.5); expect(calculateVideoOpacity(5, 10)).toBe(1); expect(calculateVideoOpacity(9.75, 10)).toBe(.5); });
  it("handles invalid durations", () => expect(calculateVideoOpacity(0, Number.NaN)).toBe(0));
});
