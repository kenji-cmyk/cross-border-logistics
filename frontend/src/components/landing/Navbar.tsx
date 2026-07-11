import { useState } from "react";

const links = [["Home", "#home"], ["How It Works", "#how-it-works"], ["Tracking", "#tracking"], ["Rates", "#rates"], ["About", "#about"]];
export function Navbar() {
  const [open, setOpen] = useState(false);
  return <header className="relative z-20 mx-auto max-w-7xl px-5 py-5 lg:px-8">
    <nav aria-label="Primary navigation" className="relative flex items-center justify-between rounded-[2rem] border border-black/10 bg-white/65 px-5 py-3 shadow-[0_12px_40px_rgba(15,23,42,0.06)] backdrop-blur-xl">
      <a href="#home" className="flex items-center gap-3 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB]">
        <span className="font-display text-2xl tracking-tight text-[#0B1220] sm:text-3xl">CrossBorder<sup className="ml-0.5 align-super font-sans text-[.35em] not-italic">®</sup></span>
        <span className="hidden border-l border-black/10 pl-3 font-sans text-[9px] font-medium uppercase leading-tight tracking-[.2em] text-[#667085] sm:block">Shopping<br />&amp; Logistics</span>
      </a>
      <div className="hidden items-center gap-7 lg:flex">{links.map(([label, href], i) => <a key={label} href={href} className={`text-sm font-medium transition-colors ${i === 0 ? "text-[#0B1220]" : "text-[#667085] hover:text-[#0B1220]"}`}>{label}</a>)}</div>
      <div className="flex items-center gap-2">
        <a href="/quote" className="hidden rounded-full bg-[#0B1220] px-6 py-2.5 text-sm font-medium text-white transition-transform duration-200 hover:scale-[1.03] active:scale-[.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB] focus-visible:ring-offset-2 sm:block">Get a Quote</a>
        <button type="button" aria-expanded={open} aria-controls="mobile-menu" aria-label="Toggle navigation menu" onClick={() => setOpen(!open)} className="grid h-10 w-10 place-items-center rounded-full border border-black/10 bg-white/70 lg:hidden"><span aria-hidden="true" className="text-lg">{open ? "×" : "≡"}</span></button>
      </div>
      {open && <div id="mobile-menu" className="absolute left-3 right-3 top-[calc(100%+.6rem)] grid gap-1 rounded-2xl border border-black/10 bg-white/95 p-3 shadow-xl backdrop-blur-xl lg:hidden">{links.map(([label, href]) => <a key={label} href={href} onClick={() => setOpen(false)} className="rounded-xl px-4 py-3 text-sm font-medium text-[#667085] hover:bg-[#EFF6FF] hover:text-[#0B1220]">{label}</a>)}<a href="/quote" className="rounded-xl bg-[#0B1220] px-4 py-3 text-center text-sm font-medium text-white sm:hidden">Get a Quote</a></div>}
    </nav>
  </header>;
}
