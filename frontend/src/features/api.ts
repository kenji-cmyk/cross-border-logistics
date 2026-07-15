import type { z } from "zod";
import { apiRequest, ApiError } from "../lib/api";
import { orderSchema, paymentSchema, quotationSchema, systemRatesSchema, trackingEventSchema, warehousePackageSchema } from "../types/api";

async function requestParsed<T>(path: string, schema: z.ZodType<T>, init?: RequestInit, timeoutMs?: number) {
  const envelope = await apiRequest<unknown>(path, init, timeoutMs);
  const result = schema.safeParse(envelope.data);
  if (!result.success) throw new ApiError("INVALID_RESPONSE", "The gateway returned an unexpected response.", envelope.meta.requestId);
  return result.data;
}
const json = (body: unknown): RequestInit => ({ method: "POST", body: JSON.stringify(body) });

export const frontendApi = {
  getRates: () => requestParsed("/api/v1/admin/rates", systemRatesSchema),
  createQuotation: (input: { customerId: string; productUrl: string; quantity: number }) => requestParsed("/api/v1/quotations/extract", quotationSchema, json(input), 15_000),
  getQuotation: (id: string) => requestParsed(`/api/v1/quotations/${encodeURIComponent(id)}`, quotationSchema),
  createOrder: (input: { quotationId: string; deliveryAddress: string; customerId?: string }) => requestParsed("/api/v1/orders", orderSchema, json(input)),
  getOrder: (id: string) => requestParsed(`/api/v1/orders/${encodeURIComponent(id)}`, orderSchema),
  getTimeline: (id: string) => requestParsed(`/api/v1/orders/${encodeURIComponent(id)}/timeline`, trackingEventSchema.array()),
  createDeposit: (orderId: string) => requestParsed("/api/v1/payments/deposit", paymentSchema, json({ orderId })),
  createRemainingBalance: (orderId: string) => requestParsed("/api/v1/payments/remaining-balance", paymentSchema, json({ orderId })),
  getPayment: (id: string) => requestParsed(`/api/v1/payments/${encodeURIComponent(id)}`, paymentSchema),
  mockPaymentSuccess: (id: string) => requestParsed(`/api/v1/payments/${encodeURIComponent(id)}/mock-success`, paymentSchema, json({})),
  receivePackage: (input: { orderId: string; sourceTrackingNumber: string; warehouseCode: string; weightKg: number; lengthCm: number; widthCm: number; heightCm: number }) => requestParsed("/api/v1/warehouse/packages/receive", warehousePackageSchema, json(input)),
  getPackage: (id: string) => requestParsed(`/api/v1/warehouse/packages/${encodeURIComponent(id)}`, warehousePackageSchema),
};
