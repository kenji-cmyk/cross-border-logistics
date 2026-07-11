import { useCallback, useEffect, useState } from "react";
import { API_BASE_URL, apiRequest } from "../lib/api";

type Order = { orderId: string; status: string; totalAmountVnd: number };
type TimelineItem = { id: string; status: string; description: string; occurredAt: string };

export function TrackingPage({ orderId }: { orderId: string }) {
  const [order, setOrder] = useState<Order | null>(null);
  const [timeline, setTimeline] = useState<TimelineItem[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const refresh = useCallback(async () => {
    try {
      const [current, events] = await Promise.all([
        apiRequest<Order>(`/api/v1/orders/${encodeURIComponent(orderId)}`),
        apiRequest<TimelineItem[]>(`/api/v1/orders/${encodeURIComponent(orderId)}/timeline`),
      ]);
      setOrder(current.data); setTimeline(events.data); setError(null);
    } catch { setError("Không thể tải trạng thái đơn hàng. Hệ thống sẽ tự thử lại."); }
  }, [orderId]);

  useEffect(() => {
    void refresh();
    const source = new EventSource(`${API_BASE_URL.replace(/\/$/, "")}/api/v1/notifications/orders/${encodeURIComponent(orderId)}/stream`);
    source.onopen = () => setConnected(true);
    source.onerror = () => { setConnected(false); void refresh(); };
    source.addEventListener("order.status_changed", () => void refresh());
    const poll = window.setInterval(() => void refresh(), 15_000);
    return () => { source.close(); window.clearInterval(poll); };
  }, [orderId, refresh]);

  return <main className="min-h-screen bg-[#F7F9FC] px-6 py-12 text-[#0B1220]">
    <section className="mx-auto max-w-2xl rounded-3xl border border-black/10 bg-white p-8 shadow-xl">
      <a href="/" className="text-sm text-blue-700">← Trang chủ</a>
      <div className="mt-5 flex items-center justify-between gap-4"><h1 className="text-2xl font-semibold">Theo dõi đơn hàng</h1><span className="text-xs text-[#667085]">{connected ? "● Cập nhật trực tiếp" : "○ Đang dùng chế độ dự phòng"}</span></div>
      <p className="mt-2 break-all text-sm text-[#667085]">{orderId}</p>
      {order && <div className="mt-6 rounded-2xl bg-blue-50 p-5"><p className="text-xs uppercase tracking-wider text-blue-700">Trạng thái hiện tại</p><p className="mt-1 text-lg font-semibold">{order.status}</p></div>}
      {error && <p className="mt-4 text-sm text-red-700">{error}</p>}
      <ol className="mt-7 space-y-4">{timeline.map(item => <li key={item.id} className="border-l-2 border-blue-500 pl-4"><p className="font-medium">{item.description}</p><p className="mt-1 text-xs text-[#667085]">{new Date(item.occurredAt).toLocaleString("vi-VN")}</p></li>)}</ol>
    </section>
  </main>;
}
