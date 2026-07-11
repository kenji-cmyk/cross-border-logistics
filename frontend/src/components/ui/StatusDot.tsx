import { cn } from "../../lib/cn";
export function StatusDot({ tone = "blue" }: { tone?: "blue" | "green" | "neutral" }) {
  return <span aria-hidden="true" className={cn("h-1.5 w-1.5 rounded-full", tone === "blue" && "bg-[#2563EB]", tone === "green" && "bg-[#12B76A]", tone === "neutral" && "bg-[#98A2B3]")} />;
}
