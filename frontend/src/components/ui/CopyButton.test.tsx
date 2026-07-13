import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CopyButton } from "./CopyButton";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

describe("CopyButton", () => {
  it("reports success after the Clipboard API copies the value", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    render(<CopyButton value="order-123" label="Copy ID" />);
    fireEvent.click(screen.getByRole("button", { name: /copy id/i }));
    await screen.findByRole("button", { name: /copied/i });
    expect(writeText).toHaveBeenCalledWith("order-123");
  });

  it("falls back to DOM copy when Clipboard API rejects", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: vi.fn().mockRejectedValue(new Error("denied")) } });
    const execCommand = vi.fn().mockReturnValue(true);
    Object.defineProperty(document, "execCommand", { configurable: true, value: execCommand });
    render(<CopyButton value="order-456" />);
    fireEvent.click(screen.getByRole("button", { name: /^copy$/i }));
    await screen.findByRole("button", { name: /copied/i });
    expect(execCommand).toHaveBeenCalledWith("copy");
    expect(document.querySelector("textarea")).toBeNull();
  });

  it("shows an error when neither copy method succeeds", async () => {
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    Object.defineProperty(document, "execCommand", { configurable: true, value: vi.fn().mockReturnValue(false) });
    render(<CopyButton value="order-789" />);
    fireEvent.click(screen.getByRole("button", { name: /^copy$/i }));
    await waitFor(() => expect(screen.getByRole("button", { name: /copy failed/i })).toBeInTheDocument());
  });

  it("uses the inverse button treatment on dark surfaces", () => {
    render(<CopyButton value="order-1" tone="inverse" />);
    expect(screen.getByRole("button")).toHaveClass("text-white", "bg-white/10");
  });
});
