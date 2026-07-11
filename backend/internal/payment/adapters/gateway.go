package adapters

import (
	"context"
	"fmt"
	"net/url"

	"github.com/example/cross-border-logistics/internal/payment/ports"
)

type MockHostedGateway struct{}

func (MockHostedGateway) CreateTransaction(_ context.Context, id string, amount int64, currency string) (ports.GatewayTransaction, error) {
	if amount <= 0 || currency != "VND" {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid gateway transaction")
	}
	ref := "mock-" + id
	return ports.GatewayTransaction{Reference: ref, HostedURL: "https://mock-payments.local/hosted/" + url.PathEscape(ref)}, nil
}
