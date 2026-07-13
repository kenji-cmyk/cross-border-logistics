import { Check, Copy, TriangleAlert } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "./Button";

export async function copyText(value: string) {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value);
      return true;
    }
  } catch {
    // Permission and insecure-context failures fall through to the DOM fallback.
  }
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.left = "-9999px";
  document.body.appendChild(textarea);
  textarea.select();
  try {
    return typeof document.execCommand === "function" && document.execCommand("copy");
  } catch {
    return false;
  } finally {
    textarea.remove();
  }
}

export function CopyButton({ value, label = "Copy", tone = "default" }: { value: string; label?: string; tone?: "default" | "inverse" }) {
  const [status, setStatus] = useState<"idle" | "success" | "error">("idle");
  const resetTimer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(resetTimer.current), []);
  const copy = async () => {
    const copied = await copyText(value);
    setStatus(copied ? "success" : "error");
    window.clearTimeout(resetTimer.current);
    resetTimer.current = window.setTimeout(() => setStatus("idle"), 1800);
  };
  const copyLabel = status === "success" ? "Copied" : status === "error" ? "Copy failed" : label;
  return <span className="inline-flex" aria-live="polite"><Button type="button" variant={tone === "inverse" ? "inverse" : "ghost"} onClick={() => void copy()} className="copy-button min-h-9 px-3 py-2 text-xs">{status === "success" ? <Check className="h-4 w-4" /> : status === "error" ? <TriangleAlert className="h-4 w-4" /> : <Copy className="h-4 w-4" />}{copyLabel}</Button></span>;
}
