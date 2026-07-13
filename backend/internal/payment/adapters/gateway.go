package adapters

import (
	"context"
	"fmt"
	"net/url"

	"github.com/example/cross-border-logistics/internal/payment/ports"
)

type MockHostedGateway struct{}

func (MockHostedGateway) CreateTransaction(_ context.Context, id, _ string, amount int64, currency string) (ports.GatewayTransaction, error) {
	if amount <= 0 || currency != "VND" {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid gateway transaction")
	}
	ref := "mock-" + id
	return ports.GatewayTransaction{Reference: ref, HostedURL: "https://mock-payments.local/hosted/" + url.PathEscape(ref)}, nil
}

func (MockHostedGateway) QueryTransaction(context.Context, string, string) (ports.GatewayResult, error) {
	return ports.GatewayResult{ResultCode: 1000, Message: "pending"}, nil
}
func (MockHostedGateway) Refund(_ context.Context, _, requestID, _ string, _ int64) (ports.GatewayResult, error) {
	return ports.GatewayResult{TransactionID: requestID, ResultCode: 0, Message: "Successful."}, nil
}
func (MockHostedGateway) QueryRefund(context.Context, string, string) (ports.GatewayResult, error) {
	return ports.GatewayResult{ResultCode: 0, Message: "Successful."}, nil
}
