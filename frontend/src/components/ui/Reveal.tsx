import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from "react";
import { useReducedMotion } from "../../hooks/useReducedMotion";
import { cn } from "../../lib/cn";

export function Reveal({ children, className, delay = 0, as: Tag = "div" }: { children: ReactNode; className?: string; delay?: number; as?: "div" | "section" | "article" }) {
  const ref = useRef<HTMLElement>(null);
  const reduced = useReducedMotion();
  const [visible, setVisible] = useState(reduced || typeof IntersectionObserver === "undefined");
  useEffect(() => {
    if (reduced || typeof IntersectionObserver === "undefined") { setVisible(true); return; }
    const element = ref.current;
    if (!element) return;
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) { setVisible(true); observer.disconnect(); }
    }, { rootMargin: "0px 0px -8%", threshold: .08 });
    observer.observe(element);
    return () => observer.disconnect();
  }, [reduced]);
  return <Tag ref={ref as never} className={cn("motion-reveal", visible && "is-visible", className)} style={{ "--reveal-delay": `${delay}ms` } as CSSProperties}>{children}</Tag>;
}
