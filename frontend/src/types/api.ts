export interface ApiSuccess<T> { data: T; meta: { requestId: string } }
export interface ApiErrorResponse { error: { code: string; message: string; details?: unknown }; meta?: { requestId?: string } }
export interface SystemRates { serviceFeePercent: number; estimatedShippingFeeVnd: number; depositPercent: number; supportedCurrencies: string[]; exchangeRates: Record<string, number>; effectiveAt: string }
export interface HealthResponse { status: string; service?: string }
