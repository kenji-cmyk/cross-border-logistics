package adapters

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/internal/payment/ports"
	"github.com/google/uuid"
)

type MockHostedGateway struct{}

func (MockHostedGateway) CreateTransaction(_ context.Context, id string, amount int64, currency string) (ports.GatewayTransaction, error) {
	if amount <= 0 || currency != "VND" {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid gateway transaction")
	}
	ref := "mock-" + id
	return ports.GatewayTransaction{Reference: ref, HostedURL: "https://mock-payments.local/hosted/" + url.PathEscape(ref)}, nil
}

type SePayGatewayConfig struct {
	BankCode          string
	AccountNumber     string
	AccountHolder     string
	PaymentCodePrefix string
	QRBaseURL         string
}

type SePayGateway struct {
	bankCode, accountNumber, accountHolder, paymentCodePrefix, qrBaseURL string
}

var sePayPrefixPattern = regexp.MustCompile(`^[A-Z0-9]{2,5}$`)

func NewSePayGateway(config SePayGatewayConfig) (*SePayGateway, error) {
	config.BankCode = strings.TrimSpace(config.BankCode)
	config.AccountNumber = strings.TrimSpace(config.AccountNumber)
	config.AccountHolder = strings.TrimSpace(config.AccountHolder)
	config.PaymentCodePrefix = strings.ToUpper(strings.TrimSpace(config.PaymentCodePrefix))
	config.QRBaseURL = strings.TrimSpace(config.QRBaseURL)
	if config.PaymentCodePrefix == "" {
		config.PaymentCodePrefix = "CBL"
	}
	if config.QRBaseURL == "" {
		config.QRBaseURL = "https://vietqr.app/img"
	}
	parsed, err := url.Parse(config.QRBaseURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid SePay QR base URL")
	}
	if config.BankCode == "" || config.AccountNumber == "" || !sePayPrefixPattern.MatchString(config.PaymentCodePrefix) {
		return nil, fmt.Errorf("invalid SePay bank account or payment code prefix")
	}
	return &SePayGateway{
		bankCode: config.BankCode, accountNumber: config.AccountNumber, accountHolder: config.AccountHolder,
		paymentCodePrefix: config.PaymentCodePrefix, qrBaseURL: parsed.String(),
	}, nil
}

func (g *SePayGateway) CreateTransaction(_ context.Context, id string, amount int64, currency string) (ports.GatewayTransaction, error) {
	compactID := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(id), "-", ""))
	if amount <= 0 || currency != "VND" || len(compactID) < 12 {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid SePay transaction")
	}
	reference := g.paymentCodePrefix + compactID[:12]
	endpoint, err := url.Parse(g.qrBaseURL)
	if err != nil {
		return ports.GatewayTransaction{}, fmt.Errorf("parse SePay QR URL: %w", err)
	}
	query := endpoint.Query()
	query.Set("acc", g.accountNumber)
	query.Set("bank", g.bankCode)
	query.Set("amount", strconv.FormatInt(amount, 10))
	query.Set("des", reference)
	query.Set("template", "compact")
	if g.accountHolder != "" {
		query.Set("holder", g.accountHolder)
	}
	endpoint.RawQuery = query.Encode()
	return ports.GatewayTransaction{Reference: reference, HostedURL: endpoint.String()}, nil
}

const (
	sePayPGSandboxAction    = "https://pay-sandbox.sepay.vn/v1/checkout/init"
	sePayPGProductionAction = "https://pay.sepay.vn/v1/checkout/init"
)

type SePayPGGatewayConfig struct {
	Environment   string
	MerchantID    string
	SecretKey     string
	ReturnBaseURL string
}

// SePayPGGateway builds signed one-time checkout forms for SePay Payment
// Gateway. Credentials remain server-side; only the generated signature is
// returned to callers.
type SePayPGGateway struct {
	action        string
	merchantID    string
	secretKey     string
	returnBaseURL *url.URL
}

func (g *SePayPGGateway) String() string {
	return fmt.Sprintf("SePayPGGateway{action:%q, merchantID:%q, secretKey:[REDACTED], returnBaseURL:%q}", g.action, g.merchantID, g.returnBaseURL)
}

func (g *SePayPGGateway) GoString() string { return g.String() }

func NewSePayPGGateway(config SePayPGGatewayConfig) (*SePayPGGateway, error) {
	environment := strings.ToLower(strings.TrimSpace(config.Environment))
	merchantID := strings.TrimSpace(config.MerchantID)
	secretKey := strings.TrimSpace(config.SecretKey)
	returnBaseURL := strings.TrimSpace(config.ReturnBaseURL)

	var action string
	switch environment {
	case "sandbox":
		action = sePayPGSandboxAction
	case "production":
		action = sePayPGProductionAction
	default:
		return nil, fmt.Errorf("invalid SePay Payment Gateway environment")
	}
	if merchantID == "" || secretKey == "" {
		return nil, fmt.Errorf("invalid SePay Payment Gateway credentials")
	}
	parsedReturnBaseURL, err := url.Parse(returnBaseURL)
	if err != nil || (parsedReturnBaseURL.Scheme != "http" && parsedReturnBaseURL.Scheme != "https") || parsedReturnBaseURL.Host == "" || parsedReturnBaseURL.User != nil || parsedReturnBaseURL.RawQuery != "" || parsedReturnBaseURL.Fragment != "" {
		return nil, fmt.Errorf("invalid SePay Payment Gateway return base URL")
	}
	parsedReturnBaseURL.Path = strings.TrimRight(parsedReturnBaseURL.Path, "/")

	return &SePayPGGateway{
		action:        action,
		merchantID:    merchantID,
		secretKey:     secretKey,
		returnBaseURL: parsedReturnBaseURL,
	}, nil
}

func (g *SePayPGGateway) CreateTransaction(_ context.Context, id string, amount int64, currency string) (ports.GatewayTransaction, error) {
	paymentID, err := uuid.Parse(strings.TrimSpace(id))
	if err != nil || amount <= 0 || currency != string(domain.CurrencyVND) {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid SePay Payment Gateway transaction")
	}
	reference := paymentID.String()
	return ports.GatewayTransaction{
		Reference: reference,
		HostedURL: "/api/v1/payments/" + url.PathEscape(reference) + "/checkout",
	}, nil
}

func (g *SePayPGGateway) BuildCheckout(_ context.Context, payment domain.Payment) (ports.CheckoutForm, error) {
	paymentID, paymentIDErr := uuid.Parse(payment.ID)
	providerReference, providerReferenceErr := uuid.Parse(payment.ProviderReference)
	if paymentIDErr != nil || providerReferenceErr != nil || paymentID != providerReference || payment.OrderID == "" || payment.AmountVND <= 0 || payment.Currency != domain.CurrencyVND {
		return ports.CheckoutForm{}, fmt.Errorf("invalid SePay Payment Gateway payment")
	}

	successURL := g.returnURL(payment, "success")
	errorURL := g.returnURL(payment, "error")
	cancelURL := g.returnURL(payment, "cancel")
	fields := []ports.CheckoutField{
		{Name: "order_amount", Value: strconv.FormatInt(payment.AmountVND, 10)},
		{Name: "merchant", Value: g.merchantID},
		{Name: "currency", Value: string(payment.Currency)},
		{Name: "operation", Value: "PURCHASE"},
		{Name: "order_description", Value: "CrossBorder payment " + payment.ProviderReference},
		{Name: "order_invoice_number", Value: payment.ProviderReference},
		{Name: "payment_method", Value: "BANK_TRANSFER"},
		{Name: "success_url", Value: successURL},
		{Name: "error_url", Value: errorURL},
		{Name: "cancel_url", Value: cancelURL},
	}

	mac := hmac.New(sha256.New, []byte(g.secretKey))
	_, _ = mac.Write([]byte(sePayPGSigningString(fields)))
	fields = append(fields, ports.CheckoutField{
		Name:  "signature",
		Value: base64.StdEncoding.EncodeToString(mac.Sum(nil)),
	})
	return ports.CheckoutForm{Action: g.action, Fields: fields}, nil
}

func (g *SePayPGGateway) returnURL(payment domain.Payment, result string) string {
	callback := *g.returnBaseURL
	callback.Path = "/orders/" + url.PathEscape(payment.OrderID) + "/payment"
	query := callback.Query()
	query.Set("paymentId", payment.ID)
	query.Set("sepay", result)
	callback.RawQuery = query.Encode()
	return callback.String()
}

func sePayPGSigningString(fields []ports.CheckoutField) string {
	signed := make([]string, 0, len(fields))
	for _, field := range fields {
		signed = append(signed, field.Name+"="+field.Value)
	}
	return strings.Join(signed, ",")
}
