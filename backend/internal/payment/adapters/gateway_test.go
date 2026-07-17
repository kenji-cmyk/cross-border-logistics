package adapters_test

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/adapters"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
)

var (
	_ ports.PaymentGateway  = (*adapters.SePayPGGateway)(nil)
	_ ports.CheckoutGateway = (*adapters.SePayPGGateway)(nil)
)

func TestSePayGatewayBuildsVietQRTransaction(t *testing.T) {
	gateway, err := adapters.NewSePayGateway(adapters.SePayGatewayConfig{
		BankCode: "MBBank", AccountNumber: "0123456789", AccountHolder: "CROSS BORDER", PaymentCodePrefix: "CBL",
	})
	if err != nil {
		t.Fatal(err)
	}
	transaction, err := gateway.CreateTransaction(context.Background(), "9f42fc31-e997-4b6f-a742-981ca145bacc", 1_039_500, "VND")
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Reference != "CBL9F42FC31E997" {
		t.Fatalf("reference = %q", transaction.Reference)
	}
	parsed, err := url.Parse(transaction.HostedURL)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if parsed.Scheme != "https" || parsed.Host != "vietqr.app" || query.Get("acc") != "0123456789" || query.Get("bank") != "MBBank" || query.Get("amount") != "1039500" || query.Get("des") != transaction.Reference {
		t.Fatalf("unexpected QR URL: %s", transaction.HostedURL)
	}
}

func TestSePayGatewayRejectsIncompleteConfiguration(t *testing.T) {
	if _, err := adapters.NewSePayGateway(adapters.SePayGatewayConfig{PaymentCodePrefix: "TOOLONG"}); err == nil {
		t.Fatal("expected invalid SePay configuration")
	}
}

func TestSePayPGGatewayCreatesLocalCheckoutTransactionWithFullUUIDReference(t *testing.T) {
	gateway := newSePayPGGateway(t, "sandbox")
	const paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"

	transaction, err := gateway.CreateTransaction(context.Background(), paymentID, 188_202, "VND")
	if err != nil {
		t.Fatal(err)
	}
	if transaction.Reference != paymentID {
		t.Fatalf("reference = %q, want full payment UUID %q", transaction.Reference, paymentID)
	}
	if transaction.HostedURL != "/api/v1/payments/"+paymentID+"/checkout" {
		t.Fatalf("hosted URL = %q", transaction.HostedURL)
	}
}

func TestSePayPGGatewayBuildsCheckoutInOfficialSigningOrder(t *testing.T) {
	gateway := newSePayPGGateway(t, "sandbox")
	payment := sePayPGPayment()

	form, err := gateway.BuildCheckout(context.Background(), payment)
	if err != nil {
		t.Fatal(err)
	}
	if form.Action != "https://pay-sandbox.sepay.vn/v1/checkout/init" {
		t.Fatalf("action = %q", form.Action)
	}

	const callbackBase = "https://calculation-ali-friend-cultural.trycloudflare.com/orders/4d990da6-d505-54d8-9324-c2de78134347/payment?paymentId=9f42fc31-e997-4b6f-a742-981ca145bacc&sepay="
	want := []ports.CheckoutField{
		{Name: "order_amount", Value: "188202"},
		{Name: "merchant", Value: "SP-TEST-KN3A55A6"},
		{Name: "currency", Value: "VND"},
		{Name: "operation", Value: "PURCHASE"},
		{Name: "order_description", Value: "CrossBorder payment 9f42fc31-e997-4b6f-a742-981ca145bacc"},
		{Name: "order_invoice_number", Value: "9f42fc31-e997-4b6f-a742-981ca145bacc"},
		{Name: "payment_method", Value: "BANK_TRANSFER"},
		{Name: "success_url", Value: callbackBase + "success"},
		{Name: "error_url", Value: callbackBase + "error"},
		{Name: "cancel_url", Value: callbackBase + "cancel"},
		{Name: "signature", Value: "A9UOX+5e54dYFjzBYlUFgtjrjb9la0qAOk+nj9/TX5E="},
	}
	if !reflect.DeepEqual(form.Fields, want) {
		t.Fatalf("checkout fields mismatch\n got: %#v\nwant: %#v", form.Fields, want)
	}
}

func TestSePayPGGatewaySelectsEnvironmentCheckoutAction(t *testing.T) {
	tests := []struct {
		environment string
		wantAction  string
	}{
		{environment: "sandbox", wantAction: "https://pay-sandbox.sepay.vn/v1/checkout/init"},
		{environment: " PRODUCTION ", wantAction: "https://pay.sepay.vn/v1/checkout/init"},
	}
	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.environment), func(t *testing.T) {
			gateway := newSePayPGGateway(t, tt.environment)
			form, err := gateway.BuildCheckout(context.Background(), sePayPGPayment())
			if err != nil {
				t.Fatal(err)
			}
			if form.Action != tt.wantAction {
				t.Fatalf("action = %q, want %q", form.Action, tt.wantAction)
			}
		})
	}
}

func TestSePayPGGatewayRejectsInvalidConfigurationWithoutLeakingSecret(t *testing.T) {
	const secret = "super-private-key"
	valid := adapters.SePayPGGatewayConfig{
		Environment: "sandbox", MerchantID: "merchant", SecretKey: secret,
		ReturnBaseURL: "https://merchant.example",
	}
	tests := []struct {
		name   string
		mutate func(*adapters.SePayPGGatewayConfig)
	}{
		{name: "missing environment", mutate: func(c *adapters.SePayPGGatewayConfig) { c.Environment = "" }},
		{name: "unsupported environment", mutate: func(c *adapters.SePayPGGatewayConfig) { c.Environment = "staging" }},
		{name: "missing merchant", mutate: func(c *adapters.SePayPGGatewayConfig) { c.MerchantID = " " }},
		{name: "missing secret", mutate: func(c *adapters.SePayPGGatewayConfig) { c.SecretKey = " " }},
		{name: "relative return URL", mutate: func(c *adapters.SePayPGGatewayConfig) { c.ReturnBaseURL = "/callback" }},
		{name: "unsupported return URL scheme", mutate: func(c *adapters.SePayPGGatewayConfig) { c.ReturnBaseURL = "ftp://merchant.example" }},
		{name: "return URL credentials", mutate: func(c *adapters.SePayPGGatewayConfig) { c.ReturnBaseURL = "https://user:pass@merchant.example" }},
		{name: "return URL query", mutate: func(c *adapters.SePayPGGatewayConfig) { c.ReturnBaseURL = "https://merchant.example?secret=value" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := valid
			tt.mutate(&config)
			_, err := adapters.NewSePayPGGateway(config)
			if err == nil {
				t.Fatal("expected configuration error")
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("configuration error leaked secret: %v", err)
			}
		})
	}
}

func TestSePayPGGatewayRejectsInvalidTransactionsAndPayments(t *testing.T) {
	gateway := newSePayPGGateway(t, "sandbox")
	transactionTests := []struct {
		name, id, currency string
		amount             int64
	}{
		{name: "invalid ID", id: "payment-1", amount: 100, currency: "VND"},
		{name: "zero amount", id: "9f42fc31-e997-4b6f-a742-981ca145bacc", amount: 0, currency: "VND"},
		{name: "unsupported currency", id: "9f42fc31-e997-4b6f-a742-981ca145bacc", amount: 100, currency: "USD"},
	}
	for _, tt := range transactionTests {
		t.Run("transaction "+tt.name, func(t *testing.T) {
			if _, err := gateway.CreateTransaction(context.Background(), tt.id, tt.amount, tt.currency); err == nil {
				t.Fatal("expected transaction validation error")
			}
		})
	}

	paymentTests := []struct {
		name   string
		mutate func(*domain.Payment)
	}{
		{name: "invalid payment ID", mutate: func(p *domain.Payment) { p.ID = "payment-1" }},
		{name: "missing order ID", mutate: func(p *domain.Payment) { p.OrderID = "" }},
		{name: "invalid reference", mutate: func(p *domain.Payment) { p.ProviderReference = "invoice-1" }},
		{name: "reference differs from payment ID", mutate: func(p *domain.Payment) { p.ProviderReference = "5ec5df4d-4fa5-49ad-82fd-84db96fd0655" }},
		{name: "zero amount", mutate: func(p *domain.Payment) { p.AmountVND = 0 }},
		{name: "unsupported currency", mutate: func(p *domain.Payment) { p.Currency = "USD" }},
	}
	for _, tt := range paymentTests {
		t.Run("payment "+tt.name, func(t *testing.T) {
			payment := sePayPGPayment()
			tt.mutate(&payment)
			if _, err := gateway.BuildCheckout(context.Background(), payment); err == nil {
				t.Fatal("expected payment validation error")
			}
		})
	}
}

func TestSePayPGGatewayDoesNotExposeSecret(t *testing.T) {
	const secret = "unit-test-secret"
	gateway := newSePayPGGateway(t, "sandbox")
	form, err := gateway.BuildCheckout(context.Background(), sePayPGPayment())
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range form.Fields {
		if strings.Contains(field.Name, secret) || strings.Contains(field.Value, secret) {
			t.Fatalf("checkout field %q exposed secret", field.Name)
		}
	}
	if strings.Contains(fmt.Sprint(gateway), secret) || strings.Contains(fmt.Sprintf("%#v", gateway), secret) {
		t.Fatal("gateway string representation exposed secret")
	}
}

func newSePayPGGateway(t *testing.T, environment string) *adapters.SePayPGGateway {
	t.Helper()
	gateway, err := adapters.NewSePayPGGateway(adapters.SePayPGGatewayConfig{
		Environment: environment, MerchantID: "SP-TEST-KN3A55A6", SecretKey: "unit-test-secret",
		ReturnBaseURL: "https://calculation-ali-friend-cultural.trycloudflare.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return gateway
}

func sePayPGPayment() domain.Payment {
	const paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"
	return domain.Payment{
		ID: paymentID, OrderID: "4d990da6-d505-54d8-9324-c2de78134347", Type: domain.TypeDeposit,
		AmountVND: 188_202, Currency: domain.CurrencyVND, Status: domain.StatusPending,
		ProviderReference: paymentID,
	}
}
