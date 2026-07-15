import { Check, Circle, Clock3, Package, Radio, RefreshCw } from "lucide-react";
import { Link } from "react-router-dom";
import { AppShell, PageContainer, PageHeader } from "../app/layouts/AppShell";
import { Button } from "../components/ui/Button";
import { CopyButton } from "../components/ui/CopyButton";
import { EmptyState, ErrorPanel, PageSkeleton } from "../components/ui/Feedback";
import { Money } from "../components/ui/Money";
import { PaymentStreamNotification } from "../components/ui/PaymentStreamNotification";
import { StatusBadge } from "../components/ui/StatusBadge";
import { useOrderStream } from "../hooks/useOrderStream";
import { formatDateTime } from "../lib/format";
import { orderStatusPresentation } from "../lib/presentation";

export function TrackingPage({ orderId }: { orderId: string }) {
  const stream = useOrderStream(orderId);
  const { order, timeline, connection } = stream;
  if (order.isPending) return <AppShell><PageContainer><PageSkeleton /></PageContainer></AppShell>;
  if (order.isError) return <AppShell><PageContainer narrow><ErrorPanel error={order.error} onRetry={() => void order.refetch()} /></PageContainer></AppShell>;

  const current = order.data;
  const presentation = orderStatusPresentation[current.status];
  const events = timeline.data ?? [];
  return (
    <AppShell>
      <PageContainer>
        <PageHeader eyebrow="Live order tracking" title="Every milestone, in one place." description="Order and timeline data come directly from the system of record. Live events trigger a fresh server read." />
        <PaymentStreamNotification notification={stream.notification} onDismiss={stream.dismissNotification} />
        <section className="rounded-[2rem] bg-ink p-6 text-white shadow-primary sm:p-8">
          <div className="flex flex-col justify-between gap-6 sm:flex-row sm:items-start">
            <div>
              <p className="text-xs font-semibold uppercase tracking-[.16em] text-white/50">Current status</p>
              <h2 className="mt-3 text-2xl font-semibold">{presentation.label}</h2>
              <p className="mt-2 max-w-xl text-sm leading-6 text-white/65">{presentation.description}</p>
            </div>
            <StatusBadge label={presentation.label} tone={presentation.tone} />
          </div>
          <div className="mt-7 flex flex-col gap-4 border-t border-white/10 pt-5 text-sm sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-center gap-2"><span className="truncate font-mono text-xs text-white/70">{orderId}</span><CopyButton value={orderId} label="Copy ID" /></div>
            <span aria-live="polite" className="inline-flex items-center gap-2 text-xs font-semibold"><Radio className={`h-4 w-4 ${connection === "live" ? "text-success" : "text-warning"}`} />{connection === "live" ? "Live updates connected" : connection === "fallback" ? "Fallback polling every 15 seconds" : "Connecting to live updates"}</span>
          </div>
        </section>
        <div className="mt-8 grid gap-8 lg:grid-cols-[1.15fr_.85fr]">
          <section className="rounded-[2rem] bg-white p-6 shadow-card sm:p-8">
            <div className="flex items-center justify-between gap-4">
              <h2 className="text-xl font-semibold">Tracking timeline</h2>
              <Button variant="ghost" onClick={() => { void order.refetch(); void timeline.refetch(); }} loading={order.isFetching || timeline.isFetching}><RefreshCw className="h-4 w-4" />Refresh</Button>
            </div>
            {timeline.isError && <div className="mt-5"><ErrorPanel error={timeline.error} onRetry={() => void timeline.refetch()} /></div>}
            {!timeline.isPending && !events.length ? (
              <div className="mt-6"><EmptyState title="No milestones yet" message="The first tracking event will appear after the order is created." /></div>
            ) : (
              <ol className="mt-7 space-y-0">
                {events.map((event, index) => {
                  const item = orderStatusPresentation[event.status];
                  const isLast = index === events.length - 1;
                  return (
                    <li key={event.id} className="relative grid grid-cols-[32px_1fr] gap-4 pb-8 last:pb-0">
                      {!isLast && <span aria-hidden="true" className="absolute bottom-0 left-[15px] top-8 w-px bg-brand/20" />}
                      <span className={`z-10 grid h-8 w-8 place-items-center rounded-full ${isLast ? "bg-brand text-white" : "bg-success-soft text-success"}`}>{isLast ? <Circle className="h-3 w-3 fill-current" /> : <Check className="h-4 w-4" />}</span>
                      <div>
                        <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-semibold">{item.label}</h3><time className="text-xs tabular-nums text-muted">{formatDateTime(event.occurredAt)}</time></div>
                        <p className="mt-2 text-sm leading-6 text-muted">{item.description}</p>
                        <details className="mt-3 text-xs text-subtle"><summary className="cursor-pointer font-medium">Technical event details</summary><dl className="mt-2 grid gap-1 break-all"><div>Source: {event.source}</div><div>Status: {event.status}</div><div>Raw description: {event.description}</div><div>Event ID: {event.id}</div></dl></details>
                      </div>
                    </li>
                  );
                })}
              </ol>
            )}
          </section>
          <aside className="space-y-5">
            <div className="rounded-[2rem] bg-white p-6 shadow-soft">
              <div className="flex items-center gap-2"><Package className="h-5 w-5 text-brand" /><h2 className="font-semibold">Product summary</h2></div>
              {current.items.map((item) => <div key={item.id} className="mt-5 border-t border-black/[.07] pt-5"><h3 className="font-semibold">{item.productName}</h3><p className="mt-1 text-sm text-muted">Quantity {item.quantity}</p><Money value={item.totalPriceVnd} className="mt-3 block text-sm font-semibold" /></div>)}
            </div>
            <div className="rounded-[2rem] bg-white p-6 shadow-soft">
              <h2 className="font-semibold">Financial summary</h2>
              <dl className="mt-5 grid gap-3 text-sm">
                <div className="flex justify-between"><dt className="text-muted">Total</dt><dd className="font-semibold"><Money value={current.totalAmountVnd} /></dd></div>
                <div className="flex justify-between"><dt className="text-muted">Deposit · 70%</dt><dd className="font-semibold"><Money value={current.depositAmountVnd} /></dd></div>
                <div className="flex justify-between"><dt className="text-muted">Remaining · 30%</dt><dd className="font-semibold"><Money value={current.remainingAmountVnd} /></dd></div>
              </dl>
              {current.status === "WAITING_REMAINING_PAYMENT" && <Link to={`/orders/${orderId}/payment`} className="mt-5 flex min-h-12 items-center justify-center rounded-2xl bg-ink px-5 text-sm font-semibold text-white">Pay remaining 30%</Link>}
            </div>
            <div className="rounded-[2rem] bg-white p-6 shadow-soft"><h2 className="font-semibold">Delivery address</h2><p className="mt-3 text-sm leading-6 text-muted">{current.deliveryAddress}</p><p className="mt-5 flex items-center gap-2 text-xs text-subtle"><Clock3 className="h-4 w-4" />Last updated {formatDateTime(current.updatedAt)}</p></div>
          </aside>
        </div>
      </PageContainer>
    </AppShell>
  );
}
