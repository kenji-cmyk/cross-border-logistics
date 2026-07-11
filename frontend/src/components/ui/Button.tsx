import { LoaderCircle } from "lucide-react";
import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";

export function Button({ children, className, variant = "primary", loading = false, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode; variant?: "primary" | "secondary" | "ghost"; loading?: boolean }) {
  return <button className={cn("ui-button inline-flex min-h-11 items-center justify-center gap-2 rounded-2xl px-5 py-3 text-sm font-semibold transition-[transform,background-color,border-color,color,box-shadow] duration-200 ease-out hover:-translate-y-0.5 active:translate-y-0 active:scale-[.98] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-55 disabled:hover:translate-y-0", variant === "primary" && "bg-ink text-white shadow-primary", variant === "secondary" && "border border-black/10 bg-white text-ink shadow-soft hover:bg-brand-soft", variant === "ghost" && "text-muted hover:bg-black/[.04] hover:text-ink", className)} disabled={loading || props.disabled} {...props}>{loading && <LoaderCircle aria-hidden="true" className="h-4 w-4 animate-spin" />}{children}</button>;
}
