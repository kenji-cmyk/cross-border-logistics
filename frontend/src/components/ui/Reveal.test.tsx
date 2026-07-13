import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { Reveal } from "./Reveal";

afterEach(cleanup);

describe("Reveal", () => {
  it("shows content immediately when IntersectionObserver is unavailable", () => {
    render(<Reveal>Visible content</Reveal>);
    expect(screen.getByText("Visible content")).toHaveClass("is-visible");
  });
});
