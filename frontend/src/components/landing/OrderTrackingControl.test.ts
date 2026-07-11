import { describe, expect, it } from "vitest";
import { validateOrderId } from "./OrderTrackingControl";
describe("tracking validation", () => { it("requires an order ID", () => expect(validateOrderId("   ")).toMatch(/order ID/i)); it("accepts a value", () => expect(validateOrderId("order-1")).toBeNull()); });
