# API reference and examples

All public responses use `{ "data": ..., "meta": { "requestId": ... } }`; errors use `{ "error": { "code", "message", "details" }, "meta": ... }`. Send `X-Request-ID` to preserve a caller ID, or let the service generate one.

## Public endpoint inventory

| Method | Path |
|---|---|
| GET | `/health` |
| POST / GET | `/api/v1/quotations/extract` / `/api/v1/quotations/{quotationId}` |
| POST / GET | `/api/v1/orders` / `/api/v1/orders/{orderId}` |
| GET | `/api/v1/orders/{orderId}/timeline` |
| POST | `/api/v1/payments/deposit` |
| GET / POST | `/api/v1/payments/{paymentId}` / `/api/v1/payments/callback` |
| GET (SSE) | `/api/v1/notifications/orders/{orderId}/stream` |
| POST | `/api/v1/warehouse/packages/receive` |
| GET | `/api/v1/warehouse/packages/{packageId}` |
| GET | `/api/v1/admin/rates` |

Internal-only routes are `/internal/quotations/{quotationId}`, `/internal/orders/{orderId}/payment-summary`, and `/internal/orders/{orderId}/warehouse-summary`. Nginx does not expose them.

## Full manual flow

Set the gateway and create unique demo data:

```bash
BASE_URL=${BASE_URL:-http://localhost}
RUN_ID=$(date +%s)
```

Extract a supported product and save its quotation ID:

```bash
QUOTATION=$(curl -sS -X POST "$BASE_URL/api/v1/quotations/extract" -H 'Content-Type: application/json' -d "{\"customerId\":\"customer-$RUN_ID\",\"productUrl\":\"https://shop.example/item/123?name=Wireless%20Keyboard&price=50&currency=USD&demo=$RUN_ID\",\"quantity\":1}")
echo "$QUOTATION" | jq
QUOTATION_ID=$(echo "$QUOTATION" | jq -r '.data.id')
curl -sS "$BASE_URL/api/v1/quotations/$QUOTATION_ID" | jq
```

Expected quotation status is `PENDING_CONFIRMATION`; total is `1485000` VND.

### Product extraction modes

`PRODUCT_EXTRACTOR_MODE=demo` is the default. It performs no outbound HTTP
request and preserves the deterministic `shop.example` and `example.com`
query-parameter fixture shown above. This is the mode used by the local demo
and sequence tests.

`PRODUCT_EXTRACTOR_MODE=hybrid` keeps those demo domains deterministic and may
also fetch configured public product pages. Set `PRODUCT_ALLOWED_HOSTS` to a
comma-separated list of exact hosts, such as
`store.example,www.store.example`. An explicit wildcard such as
`*.store.example` enables subdomains but does not enable the base host itself.
Unsupported hosts return `PRODUCT_EXTRACTION_UNAVAILABLE`; the service never
fabricates product metadata for them.

The HTTP extractor supports configured public HTTPS product pages that expose
usable JSON-LD `Product` data, OpenGraph product metadata, or basic HTML
metadata. It accepts JSON-LD products in `@graph`, `Offer`/`AggregateOffer`
objects and arrays, and string/array/`ImageObject` images. It does not convert
currencies; the existing quotation rate provider and calculation flow remain
responsible for that.

Outbound requests enforce the configured total timeout, response-size and
redirect limits. Every destination and redirect must remain on an allowed host
whose DNS answers are all public; credentials, non-HTTPS URLs, private,
loopback, link-local, multicast, and metadata-service destinations are
rejected. Only HTML-compatible responses are parsed. JavaScript-only,
login-protected, CAPTCHA-protected, and anti-bot-protected pages may return
`PRODUCT_EXTRACTION_UNAVAILABLE`; no browser or bypass mechanism is used.

Optional settings and backward-compatible defaults are documented in
`.env.example`. A hybrid deployment must configure at least one allowed host.

```bash
ORDER=$(curl -sS -X POST "$BASE_URL/api/v1/orders" -H 'Content-Type: application/json' -d "{\"quotationId\":\"$QUOTATION_ID\",\"deliveryAddress\":\"Thu Duc City, Ho Chi Minh City\"}")
echo "$ORDER" | jq
ORDER_ID=$(echo "$ORDER" | jq -r '.data.orderId')

PAYMENT=$(curl -sS -X POST "$BASE_URL/api/v1/payments/deposit" -H 'Content-Type: application/json' -d "{\"orderId\":\"$ORDER_ID\"}")
echo "$PAYMENT" | jq
PAYMENT_ID=$(echo "$PAYMENT" | jq -r '.data.paymentId')
curl -sS "$BASE_URL/api/v1/payments/$PAYMENT_ID" | jq
# `make demo` signs and replays the provider callback using PAYMENT_WEBHOOK_SECRET.
```

Expected statuses are Order `WAITING_DEPOSIT`, Payment `PENDING`, then Payment `SUCCEEDED`. Poll rather than relying on a fixed sleep:

```bash
until [[ $(curl -sS "$BASE_URL/api/v1/orders/$ORDER_ID" | jq -r '.data.status') == WAITING_PURCHASE ]]; do sleep 2; done

PACKAGE=$(curl -sS -X POST "$BASE_URL/api/v1/warehouse/packages/receive" -H 'Content-Type: application/json' -d "{\"orderId\":\"$ORDER_ID\",\"sourceTrackingNumber\":\"CN-DEMO-$RUN_ID\",\"warehouseCode\":\"CN-GZ-01\",\"weightKg\":1.4,\"lengthCm\":30,\"widthCm\":20,\"heightCm\":15}")
echo "$PACKAGE" | jq
PACKAGE_ID=$(echo "$PACKAGE" | jq -r '.data.packageId')
curl -sS "$BASE_URL/api/v1/warehouse/packages/$PACKAGE_ID" | jq

until [[ $(curl -sS "$BASE_URL/api/v1/orders/$ORDER_ID" | jq -r '.data.status') == ARRIVED_FOREIGN_WAREHOUSE ]]; do sleep 2; done
curl -sS "$BASE_URL/api/v1/orders/$ORDER_ID/timeline" | jq
curl -sS "$BASE_URL/api/v1/admin/rates" | jq
```

Admin defaults are service fee `5`, shipping fee `120000` VND, and deposit `70`. Common errors are `VALIDATION_ERROR` (400), `NOT_FOUND` (404), `CONFLICT` or `INVALID_STATE` (409), `DEPENDENCY_ERROR` (502), and `INTERNAL_ERROR` (500).
