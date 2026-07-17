package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/application"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
)

const (
	orderID   = "46ab7a1a-bab7-4a46-b9f9-d7572a284895"
	paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"
)

type fakeOrderReader struct {
	summary ports.OrderPaymentSummary
	err     error
}

type fakeCheckoutGateway struct {
	form ports.CheckoutForm
	err  error
}

func (f fakeCheckoutGateway) CreateTransaction(context.Context, string, int64, string) (ports.GatewayTransaction, error) {
	return ports.GatewayTransaction{}, nil
}

func (f fakeCheckoutGateway) BuildCheckout(context.Context, domain.Payment) (ports.CheckoutForm, error) {
	return f.form, f.err
}

func (f fakeOrderReader) GetPaymentSummary(context.Context, string) (ports.OrderPaymentSummary, error) {
	return f.summary, f.err
}

type fakeRepository struct {
	payment      domain.Payment
	createErr    error
	succeedCalls int
	outbox       ports.OutboxEvent
	callbacks    map[string]bool
}

func (f *fakeRepository) Create(_ context.Context, payment domain.Payment) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.payment = payment
	return nil
}
func (f *fakeRepository) FindByID(_ context.Context, id string) (domain.Payment, error) {
	if f.payment.ID != id {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return f.payment, nil
}
func (f *fakeRepository) FindByOrderIDAndType(_ context.Context, orderID string, paymentType domain.PaymentType) (domain.Payment, error) {
	if f.payment.OrderID != orderID || f.payment.Type != paymentType {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return f.payment, nil
}
func (f *fakeRepository) FindByProviderReference(_ context.Context, reference string) (domain.Payment, error) {
	if f.payment.ProviderReference != reference {
		return domain.Payment{}, domain.ErrPaymentNotFound
	}
	return f.payment, nil
}
func (f *fakeRepository) Succeed(_ context.Context, id string, outbox ports.OutboxEvent) (domain.Payment, bool, error) {
	if f.payment.ID != id {
		return domain.Payment{}, false, domain.ErrPaymentNotFound
	}
	f.succeedCalls++
	if f.payment.Status == domain.StatusSucceeded {
		return f.payment, false, nil
	}
	f.payment.Status = domain.StatusSucceeded
	f.payment.UpdatedAt = outbox.CreatedAt
	f.outbox = outbox
	return f.payment, true, nil
}
func (f *fakeRepository) SucceedCallback(ctx context.Context, id, eventID string, outbox ports.OutboxEvent) (domain.Payment, bool, error) {
	if f.callbacks == nil {
		f.callbacks = map[string]bool{}
	}
	if f.callbacks[eventID] {
		return f.payment, false, nil
	}
	f.callbacks[eventID] = true
	return f.Succeed(ctx, id, outbox)
}

func validSummary() ports.OrderPaymentSummary {
	return ports.OrderPaymentSummary{OrderID: orderID, DepositAmountVND: 1_039_500, RemainingAmountVND: 445_500, Status: "WAITING_DEPOSIT"}
}

func TestCreateDeposit(t *testing.T) {
	repository := &fakeRepository{}
	result, err := application.NewService(repository, fakeOrderReader{summary: validSummary()}).CreateDeposit(context.Background(), application.CreateDepositInput{OrderID: orderID})
	if err != nil {
		t.Fatal(err)
	}
	if result.OrderID != orderID || result.Type != domain.TypeDeposit || result.AmountVND != 1_039_500 || result.Currency != domain.CurrencyVND || result.Status != domain.StatusPending {
		t.Fatalf("unexpected payment: %+v", result)
	}
	if result.PaymentURL == "" || result.ProviderReference == "" {
		t.Fatalf("mock provider fields are missing: %+v", result)
	}
}

func TestCreateDepositRejectsInvalidOrderStateAndDuplicate(t *testing.T) {
	badState := validSummary()
	badState.Status = "WAITING_PURCHASE"
	_, err := application.NewService(&fakeRepository{}, fakeOrderReader{summary: badState}).CreateDeposit(context.Background(), application.CreateDepositInput{OrderID: orderID})
	if !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("state error = %v", err)
	}
	_, err = application.NewService(&fakeRepository{createErr: domain.ErrPaymentConflict}, fakeOrderReader{summary: validSummary()}).CreateDeposit(context.Background(), application.CreateDepositInput{OrderID: orderID})
	if !errors.Is(err, domain.ErrPaymentConflict) {
		t.Fatalf("duplicate error = %v", err)
	}
}

func TestCreateRemainingBalanceUsesAuthoritativeThirtyPercent(t *testing.T) {
	summary := validSummary()
	summary.Status = "WAITING_REMAINING_PAYMENT"
	repository := &fakeRepository{}
	result, err := application.NewService(repository, fakeOrderReader{summary: summary}).CreateRemainingBalance(context.Background(), application.CreateRemainingBalanceInput{OrderID: orderID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Type != domain.TypeRemainingBalance || result.AmountVND != summary.RemainingAmountVND {
		t.Fatalf("unexpected remaining payment: %+v", result)
	}
}

func TestProcessSePayWebhookValidatesAmountAndEmitsRemainingEventOnce(t *testing.T) {
	repository := &fakeRepository{payment: domain.Payment{ID: paymentID, OrderID: orderID, Type: domain.TypeRemainingBalance, AmountVND: 445_500, Currency: domain.CurrencyVND, Status: domain.StatusPending, ProviderReference: "CBL9F42FC31E997"}}
	service := application.NewService(repository, fakeOrderReader{})
	input := application.SePayWebhookInput{ProviderReference: repository.payment.ProviderReference, EventID: "sepay:92704", TransferType: "in", TransferAmountVND: 445_500}
	result, err := service.ProcessSePayWebhook(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.StatusSucceeded || repository.outbox.EventType != "payment.remaining_balance_succeeded.v1" {
		t.Fatalf("unexpected webhook result=%+v outbox=%+v", result, repository.outbox)
	}
	if _, err := service.ProcessSePayWebhook(context.Background(), input); err != nil {
		t.Fatal(err)
	}
	if repository.succeedCalls != 1 {
		t.Fatalf("replayed SePay transaction emitted %d events", repository.succeedCalls)
	}

	badAmount := input
	badAmount.EventID = "sepay:92705"
	badAmount.TransferAmountVND--
	if _, err := service.ProcessSePayWebhook(context.Background(), badAmount); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("amount mismatch error = %v", err)
	}
	badDirection := input
	badDirection.EventID = "sepay:92706"
	badDirection.TransferType = "out"
	if _, err := service.ProcessSePayWebhook(context.Background(), badDirection); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("outgoing transfer error = %v", err)
	}
}

func TestMarkSucceededCreatesOneEvent(t *testing.T) {
	repository := &fakeRepository{payment: domain.Payment{ID: paymentID, OrderID: orderID, Type: domain.TypeDeposit, AmountVND: 1_039_500, Currency: domain.CurrencyVND, Status: domain.StatusPending}}
	service := application.NewService(repository, fakeOrderReader{})
	result, err := service.MarkSucceeded(context.Background(), paymentID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != domain.StatusSucceeded || repository.succeedCalls != 1 {
		t.Fatalf("result=%+v calls=%d", result, repository.succeedCalls)
	}
	var payload map[string]any
	if err := json.Unmarshal(repository.outbox.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["eventType"] != "payment.deposit_succeeded.v1" || payload["aggregateId"] != orderID || payload["producer"] != "payment-service" {
		t.Fatalf("unexpected outbox payload: %s", repository.outbox.Payload)
	}
	if _, err := service.MarkSucceeded(context.Background(), paymentID); err != nil {
		t.Fatal(err)
	}
	if repository.succeedCalls != 1 {
		t.Fatalf("second success created another event; calls=%d", repository.succeedCalls)
	}
}

func TestMarkSucceededMissingPayment(t *testing.T) {
	_, err := application.NewService(&fakeRepository{}, fakeOrderReader{}).MarkSucceeded(context.Background(), paymentID)
	if !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildCheckoutRequiresPendingHostedPayment(t *testing.T) {
	form := ports.CheckoutForm{Action: "https://pay-sandbox.sepay.vn/v1/checkout/init", Fields: []ports.CheckoutField{{Name: "merchant", Value: "SP-TEST"}}}
	repository := &fakeRepository{payment: domain.Payment{ID: paymentID, OrderID: orderID, Status: domain.StatusPending}}
	service := application.NewService(repository, fakeOrderReader{}, fakeCheckoutGateway{form: form})
	result, err := service.BuildCheckout(context.Background(), paymentID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Action != form.Action || len(result.Fields) != 1 {
		t.Fatalf("unexpected checkout form: %+v", result)
	}

	service = application.NewService(repository, fakeOrderReader{})
	if _, err := service.BuildCheckout(context.Background(), paymentID); !errors.Is(err, domain.ErrCheckoutUnavailable) {
		t.Fatalf("missing hosted gateway error = %v", err)
	}

	repository.payment.Status = domain.StatusSucceeded
	service = application.NewService(repository, fakeOrderReader{}, fakeCheckoutGateway{form: form})
	if _, err := service.BuildCheckout(context.Background(), paymentID); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("succeeded payment checkout error = %v", err)
	}
}
