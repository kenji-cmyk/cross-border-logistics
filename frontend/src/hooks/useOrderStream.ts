import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { frontendApi } from "../features/api";
import { API_BASE_URL } from "../lib/api";
import type { TrackingEvent } from "../types/api";

export type StreamState = "connecting" | "live" | "fallback";
export const dedupeTimeline = (items: TrackingEvent[]) => Array.from(new Map(items.map((item) => [item.id, item])).values());
export function useOrderStream(orderId: string) {
  const client = useQueryClient();
  const [connection, setConnection] = useState<StreamState>("connecting");
  const order = useQuery({ queryKey: ["order", orderId], queryFn: () => frontendApi.getOrder(orderId), enabled: Boolean(orderId), retry: 1, refetchInterval: connection === "fallback" ? 15_000 : false });
  const timeline = useQuery({ queryKey: ["timeline", orderId], queryFn: () => frontendApi.getTimeline(orderId), enabled: Boolean(orderId), retry: 1, refetchInterval: connection === "fallback" ? 15_000 : false, select: dedupeTimeline });
  useEffect(() => {
    if (!orderId) return;
    const refresh = () => { void client.invalidateQueries({ queryKey: ["order", orderId] }); void client.invalidateQueries({ queryKey: ["timeline", orderId] }); };
    const source = new EventSource(`${API_BASE_URL.replace(/\/$/, "")}/api/v1/notifications/orders/${encodeURIComponent(orderId)}/stream`);
    source.onopen = () => setConnection("live");
    source.onerror = () => { setConnection("fallback"); refresh(); };
    source.addEventListener("order.status_changed", refresh);
    return () => source.close();
  }, [client, orderId]);
  return { order, timeline, connection };
}
