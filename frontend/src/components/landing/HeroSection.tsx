import { useBackendHealth } from "../../hooks/useBackendHealth";
import { StatusDot } from "../ui/StatusDot";
import { HeroActions } from "./HeroActions";
import { OrderTrackingControl } from "./OrderTrackingControl";
import { RatesPreview } from "./RatesPreview";
import { TrustIndicators } from "./TrustIndicators";

export function HeroSection() {
  const health = useBackendHealth();
  return <main id="home" className="relative z-10">
    <section aria-labelledby="hero-heading" style={{ paddingTop: "calc(8rem - 75px)" }} className="mx-auto flex max-w-7xl flex-col items-center justify-center px-5 pb-24 text-center sm:px-6 lg:px-8 lg:pb-36">
      <div className="inline-flex animate-fade-rise items-center gap-2 rounded-full border border-black/10 bg-white/70 px-4 py-2 text-xs font-medium text-[#667085] backdrop-blur-md sm:text-sm"><StatusDot />Cross-border purchasing, without the blind spots</div>
      <h1 id="hero-heading" className="mt-7 max-w-6xl animate-fade-rise font-display text-5xl font-normal text-[#0B1220] sm:text-7xl md:text-8xl lg:text-[6.5rem]" style={{ lineHeight: .94, letterSpacing: "-.035em" }}>From <em className="font-normal italic text-[#667085]">anywhere in the world,</em><br />delivered <em className="font-normal italic text-[#667085]">with clarity.</em></h1>
      <p className="mt-7 max-w-2xl animate-fade-rise-delay font-sans text-base leading-relaxed text-[#667085] sm:mt-8 sm:text-lg">Request transparent quotations, secure your deposit, and follow every milestone from purchase to foreign warehouse arrival through one connected logistics experience.</p>
      <HeroActions />
      <OrderTrackingControl />
      <TrustIndicators />
      <div className="mt-5 inline-flex items-center gap-2 text-[11px] font-medium text-[#667085]" aria-live="polite"><StatusDot tone={health === "online" ? "green" : "neutral"} />{health === "checking" ? "Checking gateway" : health === "online" ? "Gateway online" : "Demo unavailable"}</div>
      <RatesPreview />
    </section>
  </main>;
}
