# MoMo sandbox payment

Set `PAYMENT_PROVIDER=momo` and provide `MOMO_PARTNER_CODE`, `MOMO_ACCESS_KEY`, `MOMO_SECRET_KEY`, `MOMO_API_BASE_URL`, `MOMO_IPN_URL`, and `MOMO_REDIRECT_URL`. The IPN URL must be a public HTTPS URL ending in `/api/v1/payments/momo/ipn`; use a trusted HTTPS tunnel for local testing.

The browser redirect is informational only. Payment Service verifies MoMo's HMAC signature and stored partner, payment, request, and amount fields before changing state. Pending payments and refunds are queried again by the reconciliation worker. Never commit sandbox or production secrets.

Sandbox acceptance: create an Order, open its hosted deposit URL, complete MoMo payment, wait for the IPN and Kafka Order transition, create the remaining payment, then request a full refund from the same browser tab. The Order ownership token exists only in `sessionStorage`; closing the tab intentionally removes the self-service refund capability.
