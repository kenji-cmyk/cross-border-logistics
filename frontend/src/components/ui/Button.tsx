import type { ButtonHTMLAttributes, ReactNode } from "react";
import { cn } from "../../lib/cn";
export function Button({ children, className, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { children: ReactNode }) {
  return <button className={cn("focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#2563EB] focus-visible:ring-offset-2", className)} {...props}>{children}</button>;
}
