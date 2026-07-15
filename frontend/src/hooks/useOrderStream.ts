import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { frontendApi } from "../features/api";
import { API_BASE_URL } from "../lib/api";
import type { TrackingEvent } from "../types/api";

export type StreamState = "connecting" | "live" | "fallback";
export interface PaymentStreamNotification { id: string; title: string; message: string }
export const dedupeTimeline = (items: TrackingEvent[]) => Array.from(new Map(items.map((item) => [item.id, item])).values());
export const paymentNotificationForStatus = (eventId: string | undefined, status: string | undefined): PaymentStreamNotification | null => {
  if (status === "WAITING_PURCHASE") return { id: eventId ?? status, title: "Deposit received", message: "SePay confirmed the 70% deposit. Your order is now waiting for purchase." };
  if (status === "READY_FOR_DOMESTIC_DELIVERY") return { id: eventId ?? status, title: "Balance received", message: "SePay confirmed the remaining 30%. Your order is ready for domestic delivery." };
  return null;
};
export function useOrderStream(orderId: string) {
  const client = useQueryClient();
  const [connection, setConnection] = useState<StreamState>("connecting");
  const [notification, setNotification] = useState<PaymentStreamNotification | null>(null);
  const order = useQuery({ queryKey: ["order", orderId], queryFn: () => frontendApi.getOrder(orderId), enabled: Boolean(orderId), retry: 1, refetchInterval: connection === "fallback" ? 15_000 : false });
  const timeline = useQuery({ queryKey: ["timeline", orderId], queryFn: () => frontendApi.getTimeline(orderId), enabled: Boolean(orderId), retry: 1, refetchInterval: connection === "fallback" ? 15_000 : false, select: dedupeTimeline });
  useEffect(() => {
    if (!orderId) return;
    const refresh = () => { void client.invalidateQueries({ queryKey: ["order", orderId] }); void client.invalidateQueries({ queryKey: ["timeline", orderId] }); };
    const receiveStatus = (event: Event) => {
      try {
        const envelope = JSON.parse((event as MessageEvent<string>).data) as { eventId?: string; data?: { currentStatus?: string } };
        const nextNotification = paymentNotificationForStatus(envelope.eventId, envelope.data?.currentStatus);
        if (nextNotification) setNotification(nextNotification);
      } catch {
        // A malformed live event still triggers a source-of-truth refresh.
      }
      refresh();
    };
    const source = new EventSource(`${API_BASE_URL.replace(/\/$/, "")}/api/v1/notifications/orders/${encodeURIComponent(orderId)}/stream`);
    source.onopen = () => setConnection("live");
    source.onerror = () => { setConnection("fallback"); refresh(); };
    source.addEventListener("order.status_changed", receiveStatus);
    return () => source.close();
  }, [client, orderId]);
  return { order, timeline, connection, notification, dismissNotification: () => setNotification(null) };
}
