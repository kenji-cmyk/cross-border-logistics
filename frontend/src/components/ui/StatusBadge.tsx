import { cn } from "../../lib/cn";
import type { Tone } from "../../lib/presentation";
export function StatusBadge({ label, tone = "neutral" }: { label: string; tone?: Tone }) {
  return <span className={cn("inline-flex min-h-7 items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold", tone === "success" && "bg-success-soft text-success-dark", tone === "warning" && "bg-warning-soft text-warning-dark", tone === "danger" && "bg-danger-soft text-danger-dark", tone === "info" && "bg-brand-soft text-brand", tone === "neutral" && "bg-slate-100 text-muted")}><span aria-hidden="true" className="h-1.5 w-1.5 rounded-full bg-current" />{label}</span>;
}
