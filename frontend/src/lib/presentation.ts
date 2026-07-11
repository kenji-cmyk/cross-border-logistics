import type { OrderStatus, PaymentStatus, QuotationStatus } from "../types/api";

export type Tone = "info" | "success" | "warning" | "danger" | "neutral";
export const orderStatusPresentation: Record<OrderStatus, { label: string; description: string; tone: Tone }> = {
  WAITING_DEPOSIT: { label: "Awaiting Deposit", description: "Your order has been created and is awaiting the deposit payment.", tone: "warning" },
  WAITING_PURCHASE: { label: "Awaiting Purchase", description: "Your deposit has been received. The order is now awaiting purchase.", tone: "info" },
  PURCHASED: { label: "Product Purchased", description: "Your product has been purchased.", tone: "info" },
  IN_TRANSIT_TO_FOREIGN_WAREHOUSE: { label: "In Transit to Foreign Warehouse", description: "Your product is travelling to the foreign warehouse.", tone: "info" },
  ARRIVED_FOREIGN_WAREHOUSE: { label: "Arrived at Foreign Warehouse", description: "Your package has been received at the foreign warehouse.", tone: "success" },
  PACKED: { label: "Package Prepared", description: "Your package has been prepared for its next journey.", tone: "info" },
  IN_TRANSIT_TO_DOMESTIC_WAREHOUSE: { label: "In Transit to Domestic Warehouse", description: "Your package is travelling to the domestic warehouse.", tone: "info" },
  ARRIVED_DOMESTIC_WAREHOUSE: { label: "Arrived at Domestic Warehouse", description: "Your package has arrived at the domestic warehouse.", tone: "success" },
  WAITING_REMAINING_PAYMENT: { label: "Awaiting Remaining Payment", description: "The remaining balance is due.", tone: "warning" },
  READY_FOR_DOMESTIC_DELIVERY: { label: "Ready for Domestic Delivery", description: "Your package is ready for final delivery.", tone: "info" },
  OUT_FOR_DELIVERY: { label: "Out for Delivery", description: "Your package is on its way to you.", tone: "info" },
  DELIVERED: { label: "Delivered", description: "Your package has been delivered.", tone: "success" },
  CANCELLED: { label: "Cancelled", description: "This order has been cancelled.", tone: "danger" },
};
export const paymentStatusPresentation: Record<PaymentStatus, { label: string; tone: Tone }> = {
  PENDING: { label: "Payment Pending", tone: "warning" }, SUCCEEDED: { label: "Payment Successful", tone: "success" }, FAILED: { label: "Payment Failed", tone: "danger" }, CANCELLED: { label: "Payment Cancelled", tone: "neutral" }, REFUNDED: { label: "Payment Refunded", tone: "info" },
};
export const quotationStatusPresentation: Record<QuotationStatus, { label: string; tone: Tone }> = {
  PENDING_CONFIRMATION: { label: "Awaiting Confirmation", tone: "warning" }, CONFIRMED: { label: "Confirmed", tone: "success" }, EXPIRED: { label: "Expired", tone: "neutral" }, REJECTED: { label: "Rejected", tone: "danger" },
};
export const packageStatusPresentation = { RECEIVED_AT_FOREIGN_WAREHOUSE: { label: "Received at Foreign Warehouse", tone: "success" as Tone } };

const errorMap: Record<string, { title: string; message: string; action: string }> = {
  VALIDATION_ERROR: { title: "Check the information", message: "Some details are missing or invalid. Review the highlighted fields and try again.", action: "Review your details" },
  RESTRICTED_ITEM: { title: "This item is restricted", message: "This product cannot be supported by the assisted-shopping service.", action: "Choose another product" },
  UNSAFE_PRODUCT_URL: { title: "This link is not safe", message: "Use a valid HTTPS link from a source supported by this demo.", action: "Check the product link" },
  PRODUCT_EXTRACTION_UNAVAILABLE: { title: "Product details unavailable", message: "We could not read this product source. Try the sample product or another supported source.", action: "Try another link" },
  EXCHANGE_RATE_UNAVAILABLE: { title: "Rates are temporarily unavailable", message: "We could not retrieve the exchange rate needed for this quotation.", action: "Try again shortly" },
  QUOTATION_ALREADY_CONFIRMED: { title: "Quotation already used", message: "This quotation has already been reserved for an order.", action: "Return to your order" },
  NOT_FOUND: { title: "We could not find that record", message: "Check the identifier or return to the previous step.", action: "Check the ID" },
  CONFLICT: { title: "This action is already complete", message: "The record may already exist or have been used by another step.", action: "Refresh and continue" },
  INVALID_STATE: { title: "This step is not available yet", message: "The order is not in the required state for this action.", action: "Check order tracking" },
  DEPENDENCY_ERROR: { title: "A service is temporarily unavailable", message: "A connected service did not respond. Your saved progress is safe.", action: "Try again" },
  INVALID_WEBHOOK_SIGNATURE: { title: "Payment confirmation rejected", message: "The provider confirmation could not be verified.", action: "Contact support" },
  PACKAGE_ALREADY_EXISTS: { title: "Package already received", message: "This source tracking number is already registered.", action: "Use the existing package" },
  REQUEST_TIMEOUT: { title: "The request took too long", message: "The system did not respond in time. No progress has been discarded.", action: "Try again" },
  NETWORK_ERROR: { title: "Unable to reach the system", message: "Check your connection while we keep your current page intact.", action: "Reconnect and retry" },
  GATEWAY_ERROR: { title: "Gateway unavailable", message: "The request could not pass through the gateway.", action: "Try again" },
  INTERNAL_ERROR: { title: "Something went wrong", message: "An unexpected system error occurred.", action: "Try again or contact support" },
  INVALID_RESPONSE: { title: "Unexpected system response", message: "The server returned data in an unexpected format.", action: "Try again" },
};
export const presentError = (code: string) => errorMap[code] ?? errorMap.INTERNAL_ERROR;
