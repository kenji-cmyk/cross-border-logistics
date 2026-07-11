const CUSTOMER_KEY = "crossborder.demoCustomerId";
const RECENT_ORDER_KEY = "crossborder.recentOrderId";
const RECENT_QUOTE_KEY = "crossborder.recentQuotationId";
export function getDemoIdentity() {
  const existing = localStorage.getItem(CUSTOMER_KEY);
  if (existing) return existing;
  const suffix = typeof crypto.randomUUID === "function" ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  const value = `customer-demo-${suffix}`;
  localStorage.setItem(CUSTOMER_KEY, value);
  return value;
}
export const resetDemoIdentity = () => localStorage.removeItem(CUSTOMER_KEY);
export const rememberOrder = (id: string) => localStorage.setItem(RECENT_ORDER_KEY, id);
export const rememberQuotation = (id: string) => localStorage.setItem(RECENT_QUOTE_KEY, id);
export const recentOrder = () => localStorage.getItem(RECENT_ORDER_KEY);
export const recentQuotation = () => localStorage.getItem(RECENT_QUOTE_KEY);
