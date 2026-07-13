import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowUpRight,
  CheckCircle2,
  CreditCard,
  Radio,
  ShieldCheck,
} from "lucide-react";
import { useEffect, useRef } from "react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { AppShell, PageContainer, PageHeader } from "../app/layouts/AppShell";
import { Button } from "../components/ui/Button";
import { CopyButton } from "../components/ui/CopyButton";
import { ErrorPanel, PageSkeleton } from "../components/ui/Feedback";
import { Money } from "../components/ui/Money";
import { StatusBadge } from "../components/ui/StatusBadge";
import { frontendApi } from "../features/api";
import { useOrderStream } from "../hooks/useOrderStream";
import {
  orderStatusPresentation,
  paymentStatusPresentation,
} from "../lib/presentation";

const DEMO_MODE =
  import.meta.env.VITE_DEMO_MODE === "true" || import.meta.env.DEV;
export function DepositPaymentPage() {
  const { orderId = "" } = useParams();
  const [params, setParams] = useSearchParams();
  const paymentId = params.get("paymentId");
  const autoCreateAttempted = useRef(false);
  const client = useQueryClient();
  const stream = useOrderStream(orderId);
  const payment = useQuery({
    queryKey: ["payment", paymentId],
    queryFn: () => frontendApi.getPayment(paymentId!),
    enabled: Boolean(paymentId),
    refetchInterval: (query) =>
      query.state.data?.status === "PENDING" ? 5_000 : false,
  });
  const create = useMutation({
    mutationFn: () => frontendApi.createDeposit(orderId),
    onSuccess: (result) => {
      setParams({ paymentId: result.paymentId }, { replace: true });
      client.setQueryData(["payment", result.paymentId], result);
    },
  });
  const succeed = useMutation({
    mutationFn: () => frontendApi.mockPaymentSuccess(paymentId!),
    onSuccess: (result) => {
      client.setQueryData(["payment", result.paymentId], result);
      void client.invalidateQueries({ queryKey: ["order", orderId] });
    },
  });
  useEffect(() => {
    if (!autoCreateAttempted.current && !paymentId && stream.order.data?.status === "WAITING_DEPOSIT") {
      autoCreateAttempted.current = true;
      create.mutate();
    }
  }, [paymentId, stream.order.data?.status]);
  if (stream.order.isPending || (!paymentId && create.isPending))
    return (
      <AppShell>
        <PageContainer>
          <PageSkeleton />
        </PageContainer>
      </AppShell>
    );
  if (stream.order.isError)
    return (
      <AppShell>
        <PageContainer narrow>
          <ErrorPanel
            error={stream.order.error}
            onRetry={() => void stream.order.refetch()}
          />
        </PageContainer>
      </AppShell>
    );
  const order = stream.order.data;
  const currentPayment = payment.data ?? create.data;
  const orderStatus = orderStatusPresentation[order.status];
  const paymentStatus = currentPayment
    ? paymentStatusPresentation[currentPayment.status]
    : null;
  return (
    <AppShell>
      <PageContainer>
        <PageHeader
          eyebrow="Order · Step 4 of 4"
          title="Secure your deposit."
          description="CrossBorder uses a hosted payment flow. We never collect card, banking, password, or OTP information in this application."
        />
        <div className="grid gap-8 lg:grid-cols-[1.1fr_.9fr]">
          <section className="rounded-[2rem] bg-white p-6 shadow-card sm:p-8">
            <div className="flex flex-wrap items-center justify-between gap-4">
              <div>
                <p className="text-xs font-semibold uppercase tracking-wider text-muted">
                  Order ID
                </p>
                <div className="mt-1 flex items-center gap-1">
                  <p className="break-all font-mono text-sm">{order.orderId}</p>
                  <CopyButton value={order.orderId} label="Copy ID" />
                </div>
              </div>
              <StatusBadge label={orderStatus.label} tone={orderStatus.tone} />
            </div>
            <div className="mt-7 grid gap-4 rounded-3xl bg-canvas p-5 sm:grid-cols-3">
              <div>
                <p className="text-xs text-muted">Order total</p>
                <Money
                  value={order.totalAmountVnd}
                  className="mt-1 block font-semibold"
                />
              </div>
              <div>
                <p className="text-xs text-muted">Deposit due</p>
                <Money
                  value={order.depositAmountVnd}
                  className="mt-1 block font-bold text-brand"
                />
              </div>
              <div>
                <p className="text-xs text-muted">Remaining later</p>
                <Money
                  value={order.remainingAmountVnd}
                  className="mt-1 block font-semibold"
                />
              </div>
            </div>
            <div className="mt-6 flex gap-3 rounded-2xl border border-success/15 bg-success-soft p-4 text-sm text-success-dark">
              <ShieldCheck className="h-5 w-5 shrink-0" />
              <div>
                <p className="font-semibold">Hosted-payment safety</p>
                <p className="mt-1 leading-6">
                  Payment details are entered only on the provider's hosted
                  page. Verify the destination before continuing.
                </p>
              </div>
            </div>
            {(create.isError || payment.isError || succeed.isError) && (
              <div className="mt-6">
                <ErrorPanel
                  error={create.error ?? payment.error ?? succeed.error}
                  onRetry={() => {
                    create.reset();
                    succeed.reset();
                    void payment.refetch();
                  }}
                />
              </div>
            )}
            {currentPayment ? (
              <div className="mt-7">
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wider text-muted">
                      Deposit payment
                    </p>
                    <p className="mt-1 break-all font-mono text-xs text-muted">
                      {currentPayment.paymentId}
                    </p>
                  </div>
                  {paymentStatus && (
                    <StatusBadge
                      label={paymentStatus.label}
                      tone={paymentStatus.tone}
                    />
                  )}
                </div>
                <div className="mt-5">
                  <a
                    href={currentPayment.paymentUrl}
                    target="_blank"
                    rel="noreferrer noopener"
                    className="inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-2xl bg-ink px-5 text-sm font-semibold text-white shadow-primary"
                  >
                    Open payment portal <ArrowUpRight className="h-4 w-4" />
                  </a>
                </div>
                {DEMO_MODE && currentPayment.status === "PENDING" && (
                  <div className="mt-5 rounded-2xl border border-dashed border-brand/30 bg-brand-soft p-4">
                    <p className="text-xs font-semibold uppercase tracking-wider text-brand">
                      Demo tool
                    </p>
                    <p className="mt-1 text-sm text-muted">
                      Use the backend's explicit demo endpoint to simulate a
                      successful provider payment.
                    </p>
                    <Button
                      onClick={() => succeed.mutate()}
                      loading={succeed.isPending}
                      className="mt-4 w-full sm:w-auto"
                    >
                      <CreditCard className="h-4 w-4" />
                      Simulate successful payment
                    </Button>
                  </div>
                )}
              </div>
            ) : (
              <Button
                onClick={() => create.mutate()}
                loading={create.isPending}
                className="mt-6 w-full"
              >
                Create deposit payment
              </Button>
            )}
          </section>
          <aside className="space-y-5">
            <div
              aria-live="polite"
              className="rounded-[2rem] border border-black/[.07] bg-white p-6 shadow-soft"
            >
              <div className="flex items-center gap-3">
                <span className="grid h-10 w-10 place-items-center rounded-full bg-brand-soft text-brand">
                  {order.status === "WAITING_DEPOSIT" ? (
                    <Radio className="h-5 w-5" />
                  ) : (
                    <CheckCircle2 className="h-5 w-5" />
                  )}
                </span>
                <div>
                  <p className="text-sm font-semibold">Order connection</p>
                  <p className="text-xs text-muted">
                    {stream.connection === "live"
                      ? "Live updates connected"
                      : stream.connection === "fallback"
                        ? "Fallback polling active"
                        : "Connecting to live updates"}
                  </p>
                </div>
              </div>
              <p className="mt-5 text-sm leading-6 text-muted">
                {orderStatus.description}
              </p>
            </div>
            {order.status !== "WAITING_DEPOSIT" && (
              <Link
                to={`/tracking/${orderId}`}
                className="flex min-h-12 items-center justify-center rounded-2xl bg-success px-5 text-sm font-semibold text-white"
              >
                View live order tracking
              </Link>
            )}
          </aside>
        </div>
      </PageContainer>
    </AppShell>
  );
}
