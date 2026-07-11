import { beforeEach, describe, expect, it } from "vitest";
import { dedupeTimeline } from "../hooks/useOrderStream";
import { validateQuotationInput } from "../pages/NewQuotationPage";
import { validatePackage } from "../pages/WarehouseReceivePage";
import type { TrackingEvent } from "../types/api";
import { formatVnd } from "./format";
import { orderStatusPresentation, presentError } from "./presentation";
import { getDemoIdentity, resetDemoIdentity } from "./storage";

describe("frontend domain helpers", () => {
  beforeEach(() => localStorage.clear());
  it("formats VND explicitly", () => expect(formatVnd(1_485_000)).toBe("1.485.000 ₫"));
  it("maps raw order status to English copy", () => expect(orderStatusPresentation.WAITING_PURCHASE.label).toBe("Awaiting Purchase"));
  it("presents recoverable API errors", () => expect(presentError("NETWORK_ERROR").action).toMatch(/retry|reconnect/i));
  it("keeps demo identity stable until reset", () => { const first = getDemoIdentity(); expect(getDemoIdentity()).toBe(first); resetDemoIdentity(); expect(getDemoIdentity()).not.toBe(first); });
  it("accepts the supported sample quotation URL", () => expect(validateQuotationInput("https://shop.example/item/keyboard", 1)).toEqual({}));
  it("rejects insecure quotation URLs", () => expect(validateQuotationInput("http://shop.example/item", 1)).toHaveProperty("url"));
  it("validates warehouse measurements", () => expect(validatePackage({ orderId: "46ab7a1a-bab7-4a46-b9f9-d7572a284895", sourceTrackingNumber: "CN-1", warehouseCode: "CN-GZ-01", weightKg: "1.4", lengthCm: "30", widthCm: "20", heightCm: "15" })).toEqual({}));
  it("rejects measurements outside backend limits", () => expect(validatePackage({ orderId: "46ab7a1a-bab7-4a46-b9f9-d7572a284895", sourceTrackingNumber: "CN-1", warehouseCode: "CN-GZ-01", weightKg: "0", lengthCm: "501", widthCm: "20", heightCm: "15" })).toMatchObject({ weightKg: expect.any(String), lengthCm: expect.any(String) }));
  it("deduplicates visible timeline events by backend ID", () => { const event = { id: "event-1", orderId: "order-1", status: "WAITING_DEPOSIT", description: "raw", source: "order-service", occurredAt: new Date().toISOString(), createdAt: new Date().toISOString() } satisfies TrackingEvent; expect(dedupeTimeline([event, event])).toHaveLength(1); });
});
