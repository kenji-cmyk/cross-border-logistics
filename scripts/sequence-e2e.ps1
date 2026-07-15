[CmdletBinding()]
param(
    [string]$BaseUrl = "http://localhost",
    [string]$WebhookSecret = "demo-webhook-secret",
    [int]$TimeoutSeconds = 45,
    [switch]$IncludeDependencyFailure
)

$ErrorActionPreference = "Stop"
$BaseUrl = $BaseUrl.TrimEnd("/")
$script:passed = 0
$script:failed = [System.Collections.Generic.List[string]]::new()
$script:runId = "e2e-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function Test-Case([string]$Name, [scriptblock]$Body) {
    try {
        & $Body
        $script:passed++
        Write-Host "[PASS] $Name" -ForegroundColor Green
    } catch {
        $script:failed.Add("$Name :: $($_.Exception.Message)")
        Write-Host "[FAIL] $Name :: $($_.Exception.Message)" -ForegroundColor Red
    }
}

function Invoke-Api {
    param([string]$Method, [string]$Path, $Body = $null, [hashtable]$Headers = @{}, [int]$Timeout = 10)
    $params = @{ Method = $Method; Uri = "$BaseUrl$Path"; Headers = $Headers; UseBasicParsing = $true; TimeoutSec = $Timeout }
    if ($null -ne $Body) {
        $params.ContentType = "application/json"
        $params.Body = if ($Body -is [string]) { $Body } else { $Body | ConvertTo-Json -Compress -Depth 10 }
    }
    $watch = [Diagnostics.Stopwatch]::StartNew()
    try {
        $response = Invoke-WebRequest @params
        $status = [int]$response.StatusCode
        $content = [string]$response.Content
        $responseHeaders = $response.Headers
    } catch [System.Net.WebException] {
        if ($null -eq $_.Exception.Response) { throw }
        $status = [int]$_.Exception.Response.StatusCode
        $content = [string]$_.ErrorDetails.Message
        if (-not $content) {
            $reader = [IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
            try { $content = $reader.ReadToEnd() } finally { $reader.Dispose() }
        }
        $responseHeaders = $_.Exception.Response.Headers
    } finally {
        $watch.Stop()
    }
    $json = $null
    if ($content) { try { $json = $content | ConvertFrom-Json } catch {} }
    [pscustomobject]@{ Status = $status; Content = $content; Json = $json; Headers = $responseHeaders; DurationMs = $watch.ElapsedMilliseconds }
}

function New-Signature([string]$RawBody, [long]$Timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()) {
    $hmac = [Security.Cryptography.HMACSHA256]::new([Text.Encoding]::UTF8.GetBytes($WebhookSecret))
    try { $hash = $hmac.ComputeHash([Text.Encoding]::UTF8.GetBytes("$Timestamp.$RawBody")) } finally { $hmac.Dispose() }
    $hex = ([BitConverter]::ToString($hash)).Replace("-", "").ToLowerInvariant()
    "sha256=$hex"
}

function New-SePayHeaders([string]$RawBody, [long]$Timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()) {
    @{ "X-SePay-Timestamp" = "$Timestamp"; "X-SePay-Signature" = (New-Signature $RawBody $Timestamp) }
}

function New-SePayPayload([long]$TransactionId, [string]$ProviderReference, [long]$Amount) {
    @{ id = $TransactionId; gateway = "DemoBank"; transactionDate = "2026-07-15 09:30:00"; accountNumber = "0000000000"; subAccount = ""; code = $ProviderReference; content = $ProviderReference; transferType = "in"; description = "demo"; transferAmount = $Amount; accumulated = $Amount; referenceCode = "E2E$TransactionId" } | ConvertTo-Json -Compress
}

function Invoke-SignedCallback([long]$TransactionId, [string]$ProviderReference, [long]$Amount) {
    $raw = New-SePayPayload $TransactionId $ProviderReference $Amount
    Invoke-Api POST "/api/v1/payments/sepay/webhook" $raw (New-SePayHeaders $raw)
}

function Wait-OrderStatus([string]$OrderId, [string]$Expected) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        $response = Invoke-Api GET "/api/v1/orders/$OrderId"
        if ($response.Status -eq 200 -and $response.Json.data.status -eq $Expected) { return $response }
        Start-Sleep -Milliseconds 500
    } while ((Get-Date) -lt $deadline)
    throw "Order $OrderId did not reach $Expected; last response: $($response.Content)"
}

function Invoke-DbScalar([string]$Database, [string]$Sql) {
    $output = & docker compose exec -T postgres psql -U logistics -d $Database -Atc $Sql
    if ($LASTEXITCODE -ne 0) { throw "psql failed for $Database" }
    ([string]($output | Select-Object -Last 1)).Trim()
}

function Invoke-ConcurrentPost([string]$Path, $Body, [int]$Count = 5, [hashtable]$Headers = @{}) {
    $raw = if ($Body -is [string]) { $Body } else { $Body | ConvertTo-Json -Compress -Depth 10 }
    $jobs = 1..$Count | ForEach-Object {
        Start-Job -ScriptBlock {
            param($Url, $Payload, $RequestHeaders)
            try {
                $response = Invoke-WebRequest -UseBasicParsing -Method POST -Uri $Url -ContentType "application/json" -Headers $RequestHeaders -Body $Payload
                [pscustomobject]@{ Status = [int]$response.StatusCode; Content = [string]$response.Content }
            } catch [System.Net.WebException] {
                $content = [string]$_.ErrorDetails.Message
                if (-not $content) {
                    $reader = [IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
                    try { $content = $reader.ReadToEnd() } finally { $reader.Dispose() }
                }
                [pscustomobject]@{ Status = [int]$_.Exception.Response.StatusCode; Content = $content }
            }
        } -ArgumentList "$BaseUrl$Path", $raw, $Headers
    }
    try { @($jobs | Wait-Job | Receive-Job) } finally { $jobs | Remove-Job -Force }
}

function Publish-KafkaPayload([string]$Topic, [string]$Payload) {
    $encoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($Payload))
    $command = "echo '$encoded' | base64 -d | /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server kafka:9092 --topic $Topic"
    & docker compose exec -T kafka sh -lc $command | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Kafka publish failed" }
}

Write-Host "Sequence E2E run: $script:runId" -ForegroundColor Cyan

Test-Case "Gateway and public services are healthy" {
    Assert-True ((Invoke-Api GET "/health").Status -eq 200) "gateway health failed"
    Assert-True ((Invoke-Api GET "/api/v1/admin/rates").Status -eq 200) "admin route failed"
}

Test-Case "Canonical quotation extraction is below 2.5 seconds" {
    $script:quotation = Invoke-Api POST "/api/v1/quotations/extract" @{
        customerId = "customer-$script:runId"
        productUrl = "https://shop.example/item/main?name=Wireless%20Keyboard&price=50&currency=USD&image=https%3A%2F%2Fcdn.example%2Fkeyboard.png"
        quantity = 1
    }
    Assert-True ($script:quotation.Status -eq 200) $script:quotation.Content
    Assert-True ($script:quotation.DurationMs -lt 2500) "latency was $($script:quotation.DurationMs) ms"
    $productAmount = 50 * $script:quotation.Json.data.exchangeRate
    $expectedTotal = $productAmount + [math]::Floor((($productAmount * 5) + 50) / 100) + 120000
    Assert-True ($script:quotation.Json.data.totalAmountVnd -eq $expectedTotal) "unexpected quotation total"
    Assert-True ($script:quotation.Json.data.productName -eq "Wireless Keyboard") "metadata extraction failed"
}

$quotationNegatives = @(
    @{ Name = "Malformed quotation JSON"; Body = "{"; Status = 400; Code = "VALIDATION_ERROR" },
    @{ Name = "Unknown quotation field"; Body = '{"customerId":"c","productUrl":"https://shop.example/item/1","quantity":1,"rawCard":"x"}'; Status = 400; Code = "VALIDATION_ERROR" },
    @{ Name = "Restricted product"; Body = '{"customerId":"c","productUrl":"https://shop.example/item/1?name=weapon&price=50&currency=USD","quantity":1}'; Status = 400; Code = "RESTRICTED_ITEM" },
    @{ Name = "HTTP product URL"; Body = '{"customerId":"c","productUrl":"http://shop.example/item/1","quantity":1}'; Status = 400; Code = "UNSAFE_PRODUCT_URL" },
    @{ Name = "Loopback product URL"; Body = '{"customerId":"c","productUrl":"https://127.0.0.1/item/1","quantity":1}'; Status = 400; Code = "UNSAFE_PRODUCT_URL" },
    @{ Name = "Metadata-service URL"; Body = '{"customerId":"c","productUrl":"https://169.254.169.254/latest/meta-data","quantity":1}'; Status = 400; Code = "UNSAFE_PRODUCT_URL" },
    @{ Name = "Unsupported marketplace"; Body = '{"customerId":"c","productUrl":"https://unsupported.example/item/1","quantity":1}'; Status = 502; Code = "PRODUCT_EXTRACTION_UNAVAILABLE" },
    @{ Name = "Unsupported currency"; Body = '{"customerId":"c","productUrl":"https://shop.example/item/1?name=Book&price=10&currency=EUR","quantity":1}'; Status = 400; Code = "VALIDATION_ERROR" }
)
foreach ($case in $quotationNegatives) {
    Test-Case $case.Name {
        $response = Invoke-Api POST "/api/v1/quotations/extract" $case.Body
        Assert-True ($response.Status -eq $case.Status) "expected HTTP $($case.Status), got $($response.Status): $($response.Content)"
        Assert-True ($response.Json.error.code -eq $case.Code) "expected $($case.Code), got $($response.Json.error.code)"
    }
}

Test-Case "Caller request ID is preserved" {
    $requestId = "request-$script:runId"
    $response = Invoke-Api GET "/api/v1/admin/rates" $null @{ "X-Request-ID" = $requestId }
    Assert-True ($response.Headers["X-Request-ID"] -eq $requestId) "response header lost request ID"
    Assert-True ($response.Json.meta.requestId -eq $requestId) "response envelope lost request ID"
}

Test-Case "Order validation and missing quotation contracts" {
    $invalid = Invoke-Api POST "/api/v1/orders" @{ quotationId = "bad"; deliveryAddress = "HCMC" }
    Assert-True ($invalid.Status -eq 400 -and $invalid.Json.error.code -eq "VALIDATION_ERROR") $invalid.Content
    $missing = Invoke-Api POST "/api/v1/orders" @{ quotationId = "11111111-1111-4111-8111-111111111111"; deliveryAddress = "HCMC" }
    Assert-True ($missing.Status -eq 404) $missing.Content
}

Test-Case "Quotation customer compatibility field cannot be spoofed" {
    $response = Invoke-Api POST "/api/v1/orders" @{ quotationId = $script:quotation.Json.data.id; customerId = "other-customer"; deliveryAddress = "HCMC" }
    Assert-True ($response.Status -eq 409) $response.Content
}

Test-Case "Concurrent Order creation is idempotent" {
    $responses = Invoke-ConcurrentPost "/api/v1/orders" @{ quotationId = $script:quotation.Json.data.id; deliveryAddress = "Thu Duc City, Ho Chi Minh City" } 6
    $bad = @($responses | Where-Object Status -ne 201)
    Assert-True ($bad.Count -eq 0) "non-201 responses: $($bad | ConvertTo-Json -Compress)"
    $ids = @($responses | ForEach-Object { ($_.Content | ConvertFrom-Json).data.orderId } | Select-Object -Unique)
    Assert-True ($ids.Count -eq 1) "multiple Order IDs: $($ids -join ',')"
    $script:orderId = $ids[0]
    Assert-True ((Invoke-DbScalar order_db "SELECT count(*) FROM orders WHERE quotation_id='$($script:quotation.Json.data.id)';") -eq "1") "duplicate Order row"
}

Test-Case "Quotation is explicitly confirmed" {
    $response = Invoke-Api GET "/api/v1/quotations/$($script:quotation.Json.data.id)"
    Assert-True ($response.Status -eq 200 -and $response.Json.data.status -eq "CONFIRMED") $response.Content
}

Test-Case "Concurrent deposit creation is idempotent and card fields are rejected" {
    $card = Invoke-Api POST "/api/v1/payments/deposit" @{ orderId = $script:orderId; cardNumber = "4111111111111111" }
    Assert-True ($card.Status -eq 400) "raw card field was accepted"
    $responses = Invoke-ConcurrentPost "/api/v1/payments/deposit" @{ orderId = $script:orderId } 6
    $bad = @($responses | Where-Object Status -notin @(200, 201))
    Assert-True ($bad.Count -eq 0) "deposit errors: $($bad | ConvertTo-Json -Compress)"
    $ids = @($responses | ForEach-Object { ($_.Content | ConvertFrom-Json).data.paymentId } | Select-Object -Unique)
    Assert-True ($ids.Count -eq 1) "multiple payment IDs: $($ids -join ',')"
    $script:paymentId = $ids[0]
    $script:payment = (Invoke-Api GET "/api/v1/payments/$script:paymentId").Json.data
    Assert-True ($script:payment.paymentUrl -match '^https://') "hosted URL is not HTTPS"
    Assert-True ((Invoke-DbScalar payment_db "SELECT count(*) FROM payments WHERE order_id='$script:orderId' AND type='DEPOSIT';") -eq "1") "duplicate Payment row"
}

Test-Case "Invalid, stale, and malformed signed callbacks do not mutate Payment" {
    $invalid = Invoke-Api POST "/api/v1/payments/sepay/webhook" '{"id":1}' @{ "X-SePay-Timestamp" = "0"; "X-SePay-Signature" = "sha256=00" }
    Assert-True ($invalid.Status -eq 401) $invalid.Content
    $raw = '{"id":1}'
    $staleTimestamp = [DateTimeOffset]::UtcNow.AddMinutes(-10).ToUnixTimeSeconds()
    $stale = Invoke-Api POST "/api/v1/payments/sepay/webhook" $raw (New-SePayHeaders $raw $staleTimestamp)
    Assert-True ($stale.Status -eq 401) $stale.Content
    $malformed = '{'
    $badJson = Invoke-Api POST "/api/v1/payments/sepay/webhook" $malformed (New-SePayHeaders $malformed)
    Assert-True ($badJson.Status -eq 400) $badJson.Content
    $current = Invoke-Api GET "/api/v1/payments/$script:paymentId"
    Assert-True ($current.Json.data.status -eq "PENDING") "invalid callback changed Payment"
}

Test-Case "Unknown provider callback has a stable 404 contract" {
    $response = Invoke-SignedCallback ([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()) "UNKNOWN123456" 1
    Assert-True ($response.Status -eq 404) $response.Content
}

Test-Case "Concurrent signed callback replay is idempotent" {
    $transactionId = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
    $raw = New-SePayPayload $transactionId $script:payment.providerReference $script:payment.amountVnd
    $headers = New-SePayHeaders $raw
    $responses = Invoke-ConcurrentPost "/api/v1/payments/sepay/webhook" $raw 6 $headers
    $bad = @($responses | Where-Object Status -ne 200)
    Assert-True ($bad.Count -eq 0) "callback errors: $($bad | ConvertTo-Json -Compress)"
    $script:callbackEventId = "sepay:$transactionId"
    Wait-OrderStatus $script:orderId "WAITING_PURCHASE" | Out-Null
}

Test-Case "Payment, outbox, processed-event, and timeline invariants hold" {
    Assert-True ((Invoke-DbScalar payment_db "SELECT count(*) FROM provider_callbacks WHERE event_id='$script:callbackEventId';") -eq "1") "callback replay row duplicated"
    Assert-True ((Invoke-DbScalar payment_db "SELECT count(*) FROM outbox_events WHERE payload->'data'->>'paymentId'='$script:paymentId';") -eq "1") "payment outbox duplicated"
    Assert-True ((Invoke-DbScalar order_db "SELECT count(*) FROM tracking_events WHERE order_id='$script:orderId' AND status='WAITING_PURCHASE';") -eq "1") "payment timeline duplicated"
    $paymentEventId = Invoke-DbScalar payment_db "SELECT payload->>'eventId' FROM outbox_events WHERE payload->'data'->>'paymentId'='$script:paymentId' LIMIT 1;"
    Assert-True ((Invoke-DbScalar order_db "SELECT count(*) FROM processed_events WHERE event_type='payment.deposit_succeeded.v1' AND event_id='$paymentEventId';") -eq "1") "processed event is missing or duplicated"
}

Test-Case "Kafka duplicate does not duplicate Order state or timeline" {
    $payload = Invoke-DbScalar payment_db "SELECT payload::text FROM outbox_events WHERE payload->'data'->>'paymentId'='$script:paymentId' LIMIT 1;"
    Publish-KafkaPayload "payment.deposit_succeeded.v1" $payload
    Start-Sleep -Seconds 2
    Assert-True ((Invoke-DbScalar order_db "SELECT count(*) FROM tracking_events WHERE order_id='$script:orderId' AND status='WAITING_PURCHASE';") -eq "1") "Kafka duplicate added timeline entry"
}

Test-Case "SSE provides event IDs, replay, and Last-Event-ID reconnect" {
    $stream = (& curl.exe -sN --max-time 3 "$BaseUrl/api/v1/notifications/orders/$script:orderId/stream" 2>$null) -join "`n"
    Assert-True ($stream -match "event: order.status_changed") "SSE event missing"
    $eventMatch = [regex]::Match($stream, "(?m)^id: ([0-9a-f-]+)$")
    Assert-True ($eventMatch.Success) "SSE event ID missing"
    $lastId = $eventMatch.Groups[1].Value
    $reconnected = (& curl.exe -sN --max-time 1 -H "Last-Event-ID: $lastId" "$BaseUrl/api/v1/notifications/orders/$script:orderId/stream" 2>$null) -join "`n"
    Assert-True ($reconnected -match ": connected") "SSE reconnect did not connect"
    Assert-True ($reconnected -notmatch "id: $lastId") "Last-Event-ID replayed an already seen event"
}

Test-Case "Warehouse flow remains compatible after sequence refactor" {
    $package = Invoke-Api POST "/api/v1/warehouse/packages/receive" @{
        orderId = $script:orderId; sourceTrackingNumber = "CN-$script:runId"; warehouseCode = "CN-GZ-01"
        weightKg = 1.4; lengthCm = 30; widthCm = 20; heightCm = 15
    }
    Assert-True ($package.Status -eq 201) $package.Content
    Wait-OrderStatus $script:orderId "ARRIVED_FOREIGN_WAREHOUSE" | Out-Null
    $timeline = Invoke-Api GET "/api/v1/orders/$script:orderId/timeline"
    Assert-True (@($timeline.Json.data | Where-Object status -eq "ARRIVED_FOREIGN_WAREHOUSE").Count -eq 1) "warehouse timeline missing or duplicated"
}

Test-Case "Deprecated public endpoints are unavailable" {
    Assert-True ((Invoke-Api POST "/api/v1/quotations" '{}').Status -in @(404, 405)) "legacy quotation endpoint is active"
    Assert-True ((Invoke-Api POST "/api/v1/payments/deposits" '{}').Status -in @(404, 405)) "legacy deposit endpoint is active"
}

if ($IncludeDependencyFailure) {
    Test-Case "Exhausted exchange-rate dependency returns 502 without persistence" {
        $customer = "rate-failure-$script:runId"
        try {
            $env:EXCHANGE_RATE_BASE_URL = "http://unreachable-rate-provider:8080"
            & docker compose up -d --force-recreate quotation-service | Out-Null
            & docker compose up -d --wait quotation-service api-gateway frontend | Out-Null
            $response = Invoke-Api POST "/api/v1/quotations/extract" @{
                customerId = $customer; productUrl = "https://shop.example/item/rate?name=Book&price=10&currency=USD"; quantity = 1
            }  @{} 15
            Assert-True ($response.Status -eq 502 -and $response.Json.error.code -eq "EXCHANGE_RATE_UNAVAILABLE") $response.Content
            Assert-True ((Invoke-DbScalar quotation_db "SELECT count(*) FROM quotations WHERE customer_id='$customer';") -eq "0") "incomplete quotation was persisted"
        } finally {
            Remove-Item Env:EXCHANGE_RATE_BASE_URL -ErrorAction SilentlyContinue
            & docker compose up -d --force-recreate quotation-service | Out-Null
            & docker compose up -d --wait quotation-service api-gateway frontend | Out-Null
        }
    }
}

Write-Host ""
Write-Host "Passed: $script:passed; Failed: $($script:failed.Count)" -ForegroundColor Cyan
if ($script:failed.Count -gt 0) {
    $script:failed | ForEach-Object { Write-Host " - $_" -ForegroundColor Red }
    exit 1
}
Write-Host "All sequence E2E scenarios passed." -ForegroundColor Green
