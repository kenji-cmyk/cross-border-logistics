import { AlertTriangle, RotateCw } from "lucide-react";
import { ApiError } from "../../lib/api";
import { presentError } from "../../lib/presentation";
import { Button } from "./Button";

export function ErrorPanel({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const apiError = error instanceof ApiError ? error : new ApiError("INTERNAL_ERROR", "Unexpected error");
  const copy = presentError(apiError.code);
  return <div role="alert" tabIndex={-1} className="rounded-2xl border border-danger/15 bg-danger-soft p-5 text-danger-dark"><div className="flex gap-3"><AlertTriangle aria-hidden="true" className="mt-0.5 h-5 w-5 shrink-0" /><div><h2 className="font-semibold">{copy.title}</h2><p className="mt-1 text-sm leading-6">{copy.message}</p>{onRetry && <Button variant="secondary" onClick={onRetry} className="mt-4 border-danger/20 bg-white text-danger-dark"><RotateCw className="h-4 w-4" />{copy.action}</Button>}<details className="mt-4 text-xs"><summary className="cursor-pointer font-medium">Technical details</summary><dl className="mt-2 grid gap-1 break-all"><div><dt className="inline font-semibold">Error code: </dt><dd className="inline">{apiError.code}</dd></div>{apiError.requestId && <div><dt className="inline font-semibold">Request ID: </dt><dd className="inline">{apiError.requestId}</dd></div>}</dl></details></div></div></div>;
}

export function PageSkeleton() { return <div aria-label="Loading page" className="grid animate-pulse gap-5"><div className="h-9 w-2/5 rounded-xl bg-slate-200" /><div className="h-5 w-3/4 rounded-xl bg-slate-200" /><div className="h-64 rounded-[2rem] bg-white shadow-soft" /></div>; }
export function EmptyState({ title, message }: { title: string; message: string }) { return <div className="rounded-3xl border border-dashed border-black/15 bg-white/60 p-10 text-center"><h2 className="text-lg font-semibold">{title}</h2><p className="mt-2 text-sm text-muted">{message}</p></div>; }
