import { z } from "zod";

export interface ApiSuccess<T> { data: T; meta: { requestId: string } }
export interface ApiErrorResponse { error: { code: string; message: string; details?: unknown }; meta?: { requestId?: string } }

export const quotationStatusSchema = z.enum(["PENDING_CONFIRMATION", "CONFIRMED", "EXPIRED", "REJECTED"]);
export const orderStatusSchema = z.enum(["WAITING_DEPOSIT", "WAITING_PURCHASE", "PURCHASED", "IN_TRANSIT_TO_FOREIGN_WAREHOUSE", "ARRIVED_FOREIGN_WAREHOUSE", "PACKED", "IN_TRANSIT_TO_DOMESTIC_WAREHOUSE", "ARRIVED_DOMESTIC_WAREHOUSE", "WAITING_REMAINING_PAYMENT", "READY_FOR_DOMESTIC_DELIVERY", "OUT_FOR_DELIVERY", "DELIVERED", "CANCELLED"]);
export const paymentStatusSchema = z.enum(["PENDING", "SUCCEEDED", "FAILED", "CANCELLED", "REFUNDED"]);

export const quotationSchema = z.object({
  id: z.string(), customerId: z.string(), productUrl: z.string(), productName: z.string(), imageUrl: z.string().optional().default(""),
  sourcePrice: z.coerce.number(), currency: z.string(), quantity: z.number(), exchangeRate: z.number(), productAmountVnd: z.number(),
  serviceFeeVnd: z.number(), estimatedShippingFeeVnd: z.number(), totalAmountVnd: z.number(), status: quotationStatusSchema,
  createdAt: z.string(), updatedAt: z.string(),
});
export const orderItemSchema = z.object({ id: z.string(), orderId: z.string(), productName: z.string(), productUrl: z.string(), quantity: z.number(), unitPriceVnd: z.number(), totalPriceVnd: z.number(), createdAt: z.string() });
export const orderSchema = z.object({ orderId: z.string(), customerId: z.string(), quotationId: z.string(), deliveryAddress: z.string(), totalAmountVnd: z.number(), depositAmountVnd: z.number(), remainingAmountVnd: z.number(), status: orderStatusSchema, createdAt: z.string(), updatedAt: z.string(), items: z.array(orderItemSchema) });
export const trackingEventSchema = z.object({ id: z.string(), orderId: z.string(), status: orderStatusSchema, description: z.string(), source: z.string(), occurredAt: z.string(), createdAt: z.string() });
export const paymentSchema = z.object({ paymentId: z.string(), orderId: z.string(), type: z.enum(["DEPOSIT", "REMAINING_BALANCE", "REFUND"]), amountVnd: z.number(), currency: z.string(), status: paymentStatusSchema, paymentUrl: z.string(), providerReference: z.string(), createdAt: z.string(), updatedAt: z.string() });
export const warehousePackageSchema = z.object({ packageId: z.string(), orderId: z.string(), sourceTrackingNumber: z.string(), warehouseCode: z.string(), weightKg: z.number(), lengthCm: z.number(), widthCm: z.number(), heightCm: z.number(), status: z.literal("RECEIVED_AT_FOREIGN_WAREHOUSE"), receivedAt: z.string(), createdAt: z.string(), updatedAt: z.string() });
export const systemRatesSchema = z.object({ serviceFeePercent: z.number(), estimatedShippingFeeVnd: z.number(), depositPercent: z.number(), supportedCurrencies: z.array(z.string()), exchangeRates: z.record(z.string(), z.number()), effectiveAt: z.string() });

export type QuotationStatus = z.infer<typeof quotationStatusSchema>;
export type OrderStatus = z.infer<typeof orderStatusSchema>;
export type PaymentStatus = z.infer<typeof paymentStatusSchema>;
export type Quotation = z.infer<typeof quotationSchema>;
export type OrderItem = z.infer<typeof orderItemSchema>;
export type Order = z.infer<typeof orderSchema>;
export type TrackingEvent = z.infer<typeof trackingEventSchema>;
export type Payment = z.infer<typeof paymentSchema>;
export type WarehousePackage = z.infer<typeof warehousePackageSchema>;
export type SystemRates = z.infer<typeof systemRatesSchema>;
export interface HealthResponse { status: string; service?: string }
