import { formatVnd } from "../../lib/format";
export function Money({ value, className = "" }: { value: number; className?: string }) { return <span className={`tabular-nums ${className}`}>{formatVnd(value)}</span>; }
