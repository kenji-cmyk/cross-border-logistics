package orderclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/payment/ports"
)

type Client struct {
	baseURL string
	client  *http.Client
}

type successResponse struct {
	Data struct {
		OrderID            string `json:"orderId"`
		DepositAmountVND   int64  `json:"depositAmountVnd"`
		RemainingAmountVND int64  `json:"remainingAmountVnd"`
		Status             string `json:"status"`
	} `json:"data"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}}
}

func (c *Client) GetPaymentSummary(ctx context.Context, id string) (ports.OrderPaymentSummary, error) {
	endpoint := c.baseURL + "/internal/orders/" + url.PathEscape(id) + "/payment-summary"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ports.OrderPaymentSummary{}, fmt.Errorf("%w: create request: %v", domain.ErrDependency, err)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return ports.OrderPaymentSummary{}, fmt.Errorf("%w: request order: %v", domain.ErrDependency, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)
		return ports.OrderPaymentSummary{}, domain.ErrOrderNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ports.OrderPaymentSummary{}, fmt.Errorf("%w: order service returned HTTP %d", domain.ErrDependency, response.StatusCode)
	}
	var payload successResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return ports.OrderPaymentSummary{}, fmt.Errorf("%w: decode order: %v", domain.ErrDependency, err)
	}
	return ports.OrderPaymentSummary{OrderID: payload.Data.OrderID, DepositAmountVND: payload.Data.DepositAmountVND, RemainingAmountVND: payload.Data.RemainingAmountVND, Status: payload.Data.Status}, nil
}
