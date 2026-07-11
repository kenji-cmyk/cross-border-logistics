import { render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { RatesPreview } from "./RatesPreview";
describe("rates fallback", () => {
  afterEach(() => vi.restoreAllMocks());
  it("hides when the backend is unavailable", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("offline"));
    const { container } = render(<RatesPreview />);
    await waitFor(() => expect(container.querySelector("aside")).not.toBeInTheDocument());
  });
});
