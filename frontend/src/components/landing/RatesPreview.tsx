import { useEffect, useState } from "react";
import { apiRequest } from "../../lib/api";
import type { SystemRates } from "../../types/api";

export function RatesPreview() {
  const [state, setState] = useState<{ loading: boolean; rates?: SystemRates }>({ loading: true });
  useEffect(() => {
    const controller = new AbortController();
    apiRequest<SystemRates>("/api/v1/admin/rates", { signal: controller.signal }).then(({ data }) => setState({ loading: false, rates: data })).catch(() => setState({ loading: false }));
    return () => controller.abort();
  }, []);
  if (!state.loading && !state.rates) return null;
  const facts = state.rates ? [
    ["Deposit", `${state.rates.depositPercent}%`], ["Service fee", `${state.rates.serviceFeePercent}%`], ["Base shipping", new Intl.NumberFormat("en-US").format(state.rates.estimatedShippingFeeVnd) + " VND"]
  ] : [["Deposit", ""], ["Service fee", ""], ["Base shipping", ""]];
  return <aside id="rates" aria-label="Current operational rates" className="mx-auto mt-5 flex max-w-2xl flex-wrap justify-center divide-x divide-black/10 rounded-2xl border border-black/[.08] bg-white/55 px-2 py-2 backdrop-blur-lg">
    {facts.map(([label,value]) => <div key={label} className="min-w-[120px] px-4 py-1.5 text-left"><div className="text-[10px] font-medium uppercase tracking-[.15em] text-[#98A2B3]">{label}</div>{state.loading ? <div data-testid="rate-skeleton" className="mt-1 h-4 w-16 animate-pulse rounded bg-slate-200" /> : <div className="mt-0.5 text-xs font-semibold text-[#0B1220]">{value}</div>}</div>)}
  </aside>;
}
