package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/example/cross-border-logistics/internal/payment/application"
	"github.com/example/cross-border-logistics/internal/payment/domain"
	"github.com/example/cross-border-logistics/internal/payment/ports"
)

const (
	orderID   = "46ab7a1a-bab7-4a46-b9f9-d7572a284895"
	paymentID = "9f42fc31-e997-4b6f-a742-981ca145bacc"
)

type fakeOrderReader struct {
	summary ports.OrderPaymentSummary
	err     error
}

func (f fakeOrderReader) GetPaymentSummary(context.Context, string) (ports.OrderPaymentSummary, error) {
	return f.summary, f.err
}

type fakeRepository struct {
	payment      domain.Payment
	createErr    error
	succeedCalls int
	outbox       ports.OutboxEvent
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
