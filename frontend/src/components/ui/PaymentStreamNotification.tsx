import { CheckCircle2, X } from "lucide-react";
import type { PaymentStreamNotification as Notification } from "../../hooks/useOrderStream";

export function PaymentStreamNotification({ notification, onDismiss }: { notification: Notification | null; onDismiss: () => void }) {
  if (!notification) return null;
  return (
    <div role="status" aria-live="polite" className="mb-7 flex items-start gap-3 rounded-2xl border border-success/20 bg-success-soft p-4 text-success-dark shadow-soft">
      <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0" />
      <div className="min-w-0 flex-1">
        <p className="font-semibold">{notification.title}</p>
        <p className="mt-1 text-sm leading-6">{notification.message}</p>
      </div>
      <button type="button" onClick={onDismiss} aria-label="Dismiss payment notification" className="grid h-9 w-9 shrink-0 place-items-center rounded-full hover:bg-white/60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-success">
        <X className="h-4 w-4" />
      </button>
    </div>
  );
}
