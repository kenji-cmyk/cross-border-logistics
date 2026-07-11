export const formatVnd = (value: number) => `${new Intl.NumberFormat("vi-VN").format(value)} ₫`;
export const formatSourceMoney = (value: number, currency: string) => `${new Intl.NumberFormat("en-US", { maximumFractionDigits: 6 }).format(value)} ${currency}`;
export const formatDateTime = (value: string) => new Intl.DateTimeFormat("en-GB", { dateStyle: "medium", timeStyle: "short" }).format(new Date(value));
export const formatPercent = (value: number) => `${value}%`;
export const hostnameFromUrl = (value: string) => { try { return new URL(value).hostname; } catch { return "Source product"; } };
