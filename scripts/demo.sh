#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost}
DEMO_TIMEOUT_SECONDS=${DEMO_TIMEOUT_SECONDS:-45}
SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)

for command in curl jq openssl; do
  command -v "$command" >/dev/null 2>&1 || { echo "$command is required." >&2; exit 1; }
done

api_request() {
  local method=$1 url=$2 body=${3:-} response status
  if [[ -n "$body" ]]; then
    response=$(curl --silent --show-error --max-time 10 -X "$method" -H 'Content-Type: application/json' -w $'\n%{http_code}' "$url" --data "$body")
  else
    response=$(curl --silent --show-error --max-time 10 -X "$method" -w $'\n%{http_code}' "$url")
  fi
  status=${response##*$'\n'}
  response=${response%$'\n'*}
  if [[ ! "$status" =~ ^2 ]]; then
    echo "HTTP $status from $method $url" >&2
    echo "$response" >&2
    return 1
  fi
  jq -e . >/dev/null <<<"$response" || { echo "Invalid JSON from $url: $response" >&2; return 1; }
  printf '%s' "$response"
}

signed_callback() {
  local body=$1 timestamp signature response status
  timestamp=$(date +%s)
  signature=$(printf '%s' "$timestamp.$body" | openssl dgst -sha256 -hmac "${PAYMENT_WEBHOOK_SECRET:-demo-webhook-secret}" -hex | awk '{print $NF}')
  response=$(curl --silent --show-error --max-time 10 -X POST -H 'Content-Type: application/json' -H "X-Webhook-Signature: t=$timestamp,v1=$signature" -w $'\n%{http_code}' "$BASE_URL/api/v1/payments/callback" --data "$body")
  status=${response##*$'\n'}; response=${response%$'\n'*}; [[ "$status" =~ ^2 ]] || { echo "$response" >&2; return 1; }; printf '%s' "$response"
}

assert_json() {
  local response=$1 filter=$2 expected=$3 label=$4 actual
  actual=$(jq -r "$filter" <<<"$response")
  if [[ "$actual" != "$expected" ]]; then
    echo "Assertion failed: $label expected '$expected', got '$actual'." >&2
    echo "$response" >&2
    exit 1
  fi
}

wait_for_order_status() {
  local order_id=$1 expected_status=$2 timeout_seconds=${3:-30}
  local deadline=$((SECONDS + timeout_seconds)) last_response="" current=""
  while (( SECONDS < deadline )); do
    last_response=$(api_request GET "$BASE_URL/api/v1/orders/$order_id")
    current=$(jq -r '.data.status' <<<"$last_response")
    if [[ "$current" == "$expected_status" ]]; then
      printf '%s' "$last_response"
      return 0
    fi
    sleep 2
  done
  echo "Timed out waiting for Order $order_id to reach $expected_status (last status: $current)." >&2
  echo "$last_response" >&2
  return 1
}

run_id="$(date +%s)-${RANDOM:-0}"
customer_id="customer-demo-$run_id"
tracking_number="CN-DEMO-$run_id"

echo "[1/10] Checking gateway and services..."
BASE_URL="$BASE_URL" WAIT_TIMEOUT_SECONDS=120 "$SCRIPT_DIR/wait-for-services.sh"

echo "[2/10] Creating quotation..."
quotation=$(api_request POST "$BASE_URL/api/v1/quotations/extract" "$(jq -nc --arg customer "$customer_id" --arg run "$run_id" '{customerId:$customer,productUrl:("https://shop.example/item/keyboard?name=Wireless%20Keyboard&price=50&currency=USD&demo="+$run),quantity:1}')")
quotation_id=$(jq -r '.data.id' <<<"$quotation")
assert_json "$quotation" '.data.status' 'PENDING_CONFIRMATION' 'Quotation status'
expected_total=$(jq -r '(.data.exchangeRate * 50) as $amount | $amount + (((($amount * 5) + 50) / 100) | floor) + 120000' <<<"$quotation")
assert_json "$quotation" '.data.totalAmountVnd|tostring' "$expected_total" 'Quotation total'
echo "Quotation ID: $quotation_id"
echo "Total amount: $(jq -r '.data.totalAmountVnd' <<<"$quotation") VND"

echo "[3/10] Creating order..."
order=$(api_request POST "$BASE_URL/api/v1/orders" "$(jq -nc --arg quotation "$quotation_id" '{quotationId:$quotation,deliveryAddress:"Thu Duc City, Ho Chi Minh City"}')")
order_id=$(jq -r '.data.orderId' <<<"$order")
assert_json "$order" '.data.status' 'WAITING_DEPOSIT' 'Initial Order status'
echo "Order ID: $order_id"
echo "Order status: WAITING_DEPOSIT"

echo "[4/10] Creating deposit payment..."
payment=$(api_request POST "$BASE_URL/api/v1/payments/deposit" "$(jq -nc --arg order "$order_id" '{orderId:$order}')")
payment_id=$(jq -r '.data.paymentId' <<<"$payment")
assert_json "$payment" '.data.status' 'PENDING' 'Initial Payment status'
echo "Payment ID: $payment_id"
echo "Payment status: PENDING"

echo "[5/10] Simulating payment success..."
provider_reference=$(jq -r '.data.providerReference' <<<"$payment")
callback_event="callback-$run_id"
callback_body=$(jq -nc --arg event "$callback_event" --arg reference "$provider_reference" '{eventId:$event,providerReference:$reference,status:"SUCCEEDED"}')
payment=$(signed_callback "$callback_body")
assert_json "$payment" '.data.status' 'SUCCEEDED' 'Successful Payment status'
payment=$(signed_callback "$callback_body")
assert_json "$payment" '.data.status' 'SUCCEEDED' 'Idempotent callback replay'
echo "Payment status: SUCCEEDED"

echo "[6/10] Waiting for Kafka payment event..."
order=$(wait_for_order_status "$order_id" WAITING_PURCHASE "$DEMO_TIMEOUT_SECONDS")
echo "Order status: $(jq -r '.data.status' <<<"$order")"

echo "[7/10] Receiving package at foreign warehouse..."
package=$(api_request POST "$BASE_URL/api/v1/warehouse/packages/receive" "$(jq -nc --arg order "$order_id" --arg tracking "$tracking_number" '{orderId:$order,sourceTrackingNumber:$tracking,warehouseCode:"CN-GZ-01",weightKg:1.4,lengthCm:30,widthCm:20,heightCm:15}')")
package_id=$(jq -r '.data.packageId' <<<"$package")
assert_json "$package" '.data.status' 'RECEIVED_AT_FOREIGN_WAREHOUSE' 'Package status'
echo "Package ID: $package_id"

echo "[8/10] Waiting for Kafka package event..."
order=$(wait_for_order_status "$order_id" ARRIVED_FOREIGN_WAREHOUSE "$DEMO_TIMEOUT_SECONDS")
assert_json "$order" '.data.status' 'ARRIVED_FOREIGN_WAREHOUSE' 'Final Order status'
package=$(api_request GET "$BASE_URL/api/v1/warehouse/packages/$package_id")
assert_json "$package" '.data.status' 'RECEIVED_AT_FOREIGN_WAREHOUSE' 'Retrieved Package status'
echo "Order status: ARRIVED_FOREIGN_WAREHOUSE"

echo "[9/10] Reading tracking timeline and Admin rates..."
timeline=$(api_request GET "$BASE_URL/api/v1/orders/$order_id/timeline")
for status in WAITING_DEPOSIT WAITING_PURCHASE ARRIVED_FOREIGN_WAREHOUSE; do
  jq -e --arg status "$status" '.data | any(.status == $status and (.description | type == "string") and (.description | length > 0))' >/dev/null <<<"$timeline" || {
    echo "Timeline is missing the natural-language $status entry." >&2; echo "$timeline" >&2; exit 1;
  }
done
jq -r '.data[] | "- " + .description' <<<"$timeline"
rates=$(api_request GET "$BASE_URL/api/v1/admin/rates")
assert_json "$rates" '.data.serviceFeePercent|tostring' '5' 'Admin service fee'
assert_json "$rates" '.data.estimatedShippingFeeVnd|tostring' '120000' 'Admin shipping fee'
assert_json "$rates" '.data.depositPercent|tostring' '70' 'Admin deposit percentage'

echo "[10/10] Demo completed successfully."
echo
echo "Final summary:"
echo "Quotation: $quotation_id"
echo "Order: $order_id"
echo "Payment: $payment_id"
echo "Package: $package_id"
echo "Tracking number: $tracking_number"
echo "Final Order Status: ARRIVED_FOREIGN_WAREHOUSE"
