import { Check, Copy } from "lucide-react";
import { useState } from "react";
import { Button } from "./Button";
export function CopyButton({ value, label = "Copy" }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => { await navigator.clipboard.writeText(value); setCopied(true); window.setTimeout(() => setCopied(false), 1600); };
  return <Button type="button" variant="ghost" onClick={() => void copy()} className="min-h-9 px-3 py-2 text-xs">{copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}{copied ? "Copied" : label}</Button>;
}
