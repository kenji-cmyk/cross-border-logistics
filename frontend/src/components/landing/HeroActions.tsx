export function HeroActions() {
  return <div className="mt-9 flex flex-col items-center gap-3 sm:flex-row sm:mt-12 animate-fade-rise-delay-2">
    <a href="/quote" className="w-full rounded-full bg-[#0B1220] px-10 py-4 text-center text-base font-medium text-white shadow-[0_14px_40px_rgba(11,18,32,0.18)] transition-all duration-200 hover:scale-[1.03] hover:shadow-[0_18px_46px_rgba(11,18,32,0.24)] active:scale-[.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB] focus-visible:ring-offset-2 sm:w-auto">Start a Quotation <span aria-hidden="true">↗</span></a>
    <a href="#tracking" className="w-full rounded-full border border-black/10 bg-white/70 px-10 py-4 text-center text-base font-medium text-[#0B1220] backdrop-blur-lg transition-all duration-200 hover:scale-[1.02] hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB] sm:w-auto">Track an Order</a>
  </div>;
}
