import { cleanup, render, screen } from "@testing-library/react";
import { ArrowRight } from "lucide-react";
import { afterEach, describe, expect, it } from "vitest";
import { Button } from "./Button";

afterEach(cleanup);

describe("Button", () => {
  it("keeps an icon beside the label as a direct flex item", () => {
    render(<Button>Continue <ArrowRight data-testid="button-icon" /></Button>);

    const button = screen.getByRole("button", { name: "Continue" });
    expect(button.querySelector(":scope > svg")).toBe(screen.getByTestId("button-icon"));
  });

  it("uses a readable light style for the secondary variant", () => {
    render(<Button variant="secondary">Receive package</Button>);

    expect(screen.getByRole("button", { name: "Receive package" })).toHaveClass("bg-white", "text-ink");
  });

  it("keeps inverse controls readable on dark surfaces", () => {
    render(<Button variant="inverse">Copy</Button>);
    expect(screen.getByRole("button")).toHaveClass("text-white", "bg-white/10", "focus-visible:ring-white");
  });
});
