import { ArrowRight, Box, CircleDollarSign, Globe2, ShieldCheck } from "lucide-react";
import { Link } from "react-router-dom";
import { BackgroundVideo } from "../components/landing/BackgroundVideo";
import { HeroSection } from "../components/landing/HeroSection";
import { LogisticsRouteGraphic } from "../components/landing/LogisticsRouteGraphic";
import { Navbar } from "../components/landing/Navbar";
import { Reveal } from "../components/ui/Reveal";

const steps = [
  ["01", "Request a quotation", "Share a supported product link. Product and fee details are returned with a clear VND total."],
  ["02", "Secure the deposit", "Confirm your delivery address and continue to the hosted deposit experience."],
  ["03", "Follow every milestone", "Live status signals refresh the authoritative order and tracking timeline automatically."],
];
const assurances = [
  [ShieldCheck, "Hosted payment", "Sensitive payment details stay with the provider."],
  [CircleDollarSign, "Backend totals", "Authoritative money comes from service responses."],
  [Globe2, "Supported sources", "Product-source limits are explained honestly."],
  [Box, "Event-driven tracking", "SSE signals trigger fresh order reads."],
] as const;

export function LandingPage() {
  return <div className="relative min-h-screen w-full overflow-hidden bg-canvas">
    <BackgroundVideo /><LogisticsRouteGraphic /><Navbar /><HeroSection />
    <section id="how-it-works" className="relative z-10 mx-auto max-w-7xl scroll-mt-24 px-5 py-24 sm:px-6 lg:px-8">
      <Reveal>
        <p className="text-xs font-semibold uppercase tracking-[.2em] text-brand">How it works</p>
        <div className="mt-4 grid gap-8 lg:grid-cols-[.8fr_1.2fr]"><h2 className="font-display text-5xl leading-none tracking-tight sm:text-6xl">Clarity from product link to warehouse.</h2><p className="max-w-xl text-base leading-7 text-muted lg:justify-self-end">One connected experience carries your identifiers forward, preserves your progress after reload, and makes backend-owned financial and logistics status easy to understand.</p></div>
      </Reveal>
      <div className="mt-12 grid gap-5 md:grid-cols-3">{steps.map(([number, title, copy], index) => <Reveal key={number} as="article" delay={index * 70} className="motion-interactive-surface rounded-[2rem] border border-black/[.07] bg-white p-6 shadow-soft"><span className="text-xs font-semibold text-brand">{number}</span><h3 className="mt-12 text-xl font-semibold">{title}</h3><p className="mt-3 text-sm leading-6 text-muted">{copy}</p></Reveal>)}</div>
    </section>
    <section id="about" className="relative z-10 mx-auto max-w-7xl scroll-mt-24 px-5 pb-24 sm:px-6 lg:px-8">
      <Reveal className="overflow-hidden rounded-[2.5rem] bg-ink p-7 text-white shadow-primary sm:p-12">
        <div className="grid gap-10 lg:grid-cols-[1fr_1fr]"><div><p className="text-xs font-semibold uppercase tracking-[.2em] text-blue-300">Trust and safety</p><h2 className="mt-4 font-display text-5xl leading-none">Transparent where it matters.</h2><p className="mt-5 max-w-lg text-sm leading-7 text-white/60">No raw payment credentials, no hidden browser calculations, and no invented delivery promises. Each interface reflects what the current demo backend can actually do.</p></div><div className="grid gap-3 sm:grid-cols-2">{assurances.map(([Icon, title, copy], index) => <Reveal key={title} delay={80 + index * 55} className="rounded-3xl border border-white/10 bg-white/[.06] p-5"><Icon className="h-5 w-5 text-blue-300" /><h3 className="mt-4 font-semibold">{title}</h3><p className="mt-2 text-xs leading-5 text-white/55">{copy}</p></Reveal>)}</div></div>
      </Reveal>
    </section>
    <Reveal as="section"><footer className="relative z-10 border-t border-black/[.06] bg-white/50"><div className="mx-auto flex max-w-7xl flex-col gap-6 px-5 py-10 sm:flex-row sm:items-center sm:justify-between sm:px-6 lg:px-8"><span className="font-display text-3xl">CrossBorder</span><nav className="flex flex-wrap gap-5 text-sm font-medium text-muted"><Link className="motion-nav-link" to="/quote">Quotation</Link><Link className="motion-nav-link" to="/rates">Rates</Link><Link className="motion-nav-link" to="/warehouse/receive">Warehouse</Link><Link className="motion-nav-link" to="/admin/rates">Admin rates</Link></nav><Link to="/quote" className="motion-cta inline-flex items-center gap-2 font-semibold">Start now <ArrowRight className="h-4 w-4" /></Link></div></footer></Reveal>
  </div>;
}
