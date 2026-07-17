package orderclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/warehouse/ports"
	"github.com/google/uuid"
)

type Client struct {
	baseURL string
	client  *http.Client
	logger  *slog.Logger
}
type response struct {
	Data struct {
		OrderID    uuid.UUID `json:"orderId"`
		CustomerID string    `json:"customerId"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"createdAt"`
	} `json:"data"`
}

func New(baseURL string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}, logger: logger}
}
func (c *Client) GetWarehouseSummary(ctx context.Context, id uuid.UUID, requestID string) (ports.OrderWarehouseSummary, error) {
	endpoint := c.baseURL + "/internal/orders/" + url.PathEscape(id.String()) + "/warehouse-summary"
	var last error
	for attempt := 1; attempt <= 2; attempt++ {
		if ctx.Err() != nil {
			return ports.OrderWarehouseSummary{}, ctx.Err()
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return ports.OrderWarehouseSummary{}, fmt.Errorf("%w: create request", domain.ErrOrderServiceUnavailable)
		}
		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
		res, err := c.client.Do(req)
		if err != nil {
			last = err
			c.logger.WarnContext(ctx, "order warehouse summary request failed", "order_id", id, "attempt", attempt, "error", err)
			continue
		}
		result, done, err := decode(res, id)
		if done {
			return result, err
		}
		last = err
	}
	return ports.OrderWarehouseSummary{}, fmt.Errorf("%w: %v", domain.ErrOrderServiceUnavailable, last)
}
func decode(res *http.Response, id uuid.UUID) (ports.OrderWarehouseSummary, bool, error) {
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, res.Body)
		return ports.OrderWarehouseSummary{}, true, domain.ErrOrderNotFound
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 1<<20))
		return ports.OrderWarehouseSummary{}, false, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	var body response
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return ports.OrderWarehouseSummary{}, false, err
	}
	return ports.OrderWarehouseSummary{OrderID: body.Data.OrderID, CustomerID: body.Data.CustomerID, Status: body.Data.Status, CreatedAt: body.Data.CreatedAt}, true, nil
}
