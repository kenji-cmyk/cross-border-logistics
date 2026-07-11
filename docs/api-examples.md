# API reference and examples

All public responses use `{ "data": ..., "meta": { "requestId": ... } }`; errors use `{ "error": { "code", "message", "details" }, "meta": ... }`. Send `X-Request-ID` to preserve a caller ID, or let the service generate one.

## Public endpoint inventory

| Method | Path |
|---|---|
| GET | `/health` |
| POST / GET | `/api/v1/quotations` / `/api/v1/quotations/{quotationId}` |
| POST / GET | `/api/v1/orders` / `/api/v1/orders/{orderId}` |
| GET | `/api/v1/orders/{orderId}/timeline` |
| POST | `/api/v1/payments/deposits` |
| GET / POST | `/api/v1/payments/{paymentId}` / `/api/v1/payments/{paymentId}/mock-success` |
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

Create a quotation and save its ID:

```bash
QUOTATION=$(curl -sS -X POST "$BASE_URL/api/v1/quotations" -H 'Content-Type: application/json' -d "{\"customerId\":\"customer-$RUN_ID\",\"productUrl\":\"https://example.com/product/123?demo=$RUN_ID\",\"productName\":\"Wireless Keyboard\",\"sourcePrice\":50,\"currency\":\"USD\",\"quantity\":1}")
echo "$QUOTATION" | jq
QUOTATION_ID=$(echo "$QUOTATION" | jq -r '.data.id')
curl -sS "$BASE_URL/api/v1/quotations/$QUOTATION_ID" | jq
```

Expected quotation status is `PENDING_CONFIRMATION`; total is `1485000` VND.

```bash
ORDER=$(curl -sS -X POST "$BASE_URL/api/v1/orders" -H 'Content-Type: application/json' -d "{\"quotationId\":\"$QUOTATION_ID\",\"customerId\":\"customer-$RUN_ID\",\"deliveryAddress\":\"Thu Duc City, Ho Chi Minh City\"}")
echo "$ORDER" | jq
ORDER_ID=$(echo "$ORDER" | jq -r '.data.orderId')

PAYMENT=$(curl -sS -X POST "$BASE_URL/api/v1/payments/deposits" -H 'Content-Type: application/json' -d "{\"orderId\":\"$ORDER_ID\"}")
echo "$PAYMENT" | jq
PAYMENT_ID=$(echo "$PAYMENT" | jq -r '.data.paymentId')
curl -sS "$BASE_URL/api/v1/payments/$PAYMENT_ID" | jq
curl -sS -X POST "$BASE_URL/api/v1/payments/$PAYMENT_ID/mock-success" | jq
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
