import { useState, type FormEvent } from "react";
export const validateOrderId = (value: string) => value.trim() ? null : "Enter an order ID to view its timeline.";

export function OrderTrackingControl() {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);
  const submit = (event: FormEvent) => {
    event.preventDefault(); const validation = validateOrderId(value); setError(validation);
    if (!validation) window.location.assign(`/tracking/${encodeURIComponent(value.trim())}`);
  };
  return <form id="tracking" onSubmit={submit} noValidate className="mt-7 w-full max-w-xl animate-fade-rise-delay-2 scroll-mt-24">
    <div className="flex flex-col gap-2 rounded-2xl border border-black/10 bg-white/70 p-2 shadow-[0_18px_60px_rgba(15,23,42,0.08)] backdrop-blur-xl sm:flex-row">
      <label htmlFor="order-id" className="sr-only">Order ID</label>
      <input id="order-id" value={value} onChange={(e) => { setValue(e.target.value); if (error) setError(null); }} aria-describedby={error ? "tracking-error" : "tracking-hint"} aria-invalid={Boolean(error)} placeholder="Enter your order ID" className="min-w-0 flex-1 rounded-xl bg-transparent px-4 py-3 text-sm text-[#0B1220] outline-none placeholder:text-[#98A2B3] focus:ring-2 focus:ring-[#2563EB]/30" />
      <button type="submit" className="rounded-xl bg-[#0B1220] px-6 py-3 text-sm font-medium text-white transition-transform hover:scale-[1.02] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB] focus-visible:ring-offset-2">View Timeline <span aria-hidden="true">→</span></button>
    </div>
    <p id="tracking-hint" className="sr-only">Enter the order identifier from your confirmation.</p>
    {error && <p id="tracking-error" role="alert" className="mt-2 text-sm text-red-700">{error}</p>}
    <span aria-live="polite" className="sr-only" />
  </form>;
}
