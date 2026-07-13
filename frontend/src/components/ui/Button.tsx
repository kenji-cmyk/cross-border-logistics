import { LoaderCircle } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "inverse";

export function Button({ children, className, variant = "primary", loading = false, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode; variant?: ButtonVariant; loading?: boolean }) {
  return <button aria-busy={loading || undefined} className={cn("ui-button inline-flex min-h-11 items-center justify-center gap-2 rounded-2xl border border-transparent px-5 py-3 text-sm font-semibold transition-[transform,background-color,border-color,color,box-shadow,opacity] duration-200 ease-out hover:-translate-y-0.5 active:translate-y-0 active:scale-[.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-55 disabled:hover:translate-y-0 disabled:active:scale-100", variant === "primary" && "bg-ink text-white shadow-primary hover:bg-slate-800 active:bg-black", variant === "secondary" && "border-black/10 bg-white text-ink shadow-soft hover:border-brand/25 hover:bg-brand-soft active:bg-blue-100", variant === "ghost" && "text-muted hover:bg-slate-100 hover:text-ink active:bg-slate-200", variant === "inverse" && "border-white/15 bg-white/10 text-white hover:border-white/25 hover:bg-white/20 active:bg-white/25 focus-visible:ring-white focus-visible:ring-offset-ink", className)} disabled={loading || props.disabled} {...props}>{loading && <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />}{children}</button>;
}
