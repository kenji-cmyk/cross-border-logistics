package quotationclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/order/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/order/ports"
)

type Client struct {
	baseURL string
	client  *http.Client
}

func (c *Client) ConfirmQuotation(ctx context.Context, id, orderID string) (ports.QuotationSnapshot, error) {
	body, _ := json.Marshal(map[string]string{"orderId": orderID})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/quotations/"+url.PathEscape(id)+"/confirm", bytes.NewReader(body))
	if err != nil {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: create confirmation request", domain.ErrDependency)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: confirm quotation: %v", domain.ErrDependency, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusConflict {
		return ports.QuotationSnapshot{}, domain.ErrQuotationConflict
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: confirmation HTTP %d", domain.ErrDependency, response.StatusCode)
	}
	var payload successResponse
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload) != nil {
		return ports.QuotationSnapshot{}, domain.ErrDependency
	}
	return ports.QuotationSnapshot{QuotationID: payload.Data.QuotationID, CustomerID: payload.Data.CustomerID, ProductURL: payload.Data.ProductURL, ProductName: payload.Data.ProductName, Quantity: payload.Data.Quantity, TotalAmountVND: payload.Data.TotalAmountVND, Status: payload.Data.Status, CreatedAt: payload.Data.CreatedAt}, nil
}

type successResponse struct {
	Data struct {
		QuotationID    string    `json:"quotationId"`
		CustomerID     string    `json:"customerId"`
		ProductURL     string    `json:"productUrl"`
		ProductName    string    `json:"productName"`
		Quantity       int       `json:"quantity"`
		TotalAmountVND int64     `json:"totalAmountVnd"`
		Status         string    `json:"status"`
		CreatedAt      time.Time `json:"createdAt"`
	} `json:"data"`
}

func New(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: timeout}}
}

func (c *Client) GetQuotation(ctx context.Context, id string) (ports.QuotationSnapshot, error) {
	endpoint := c.baseURL + "/internal/quotations/" + url.PathEscape(id)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: create request: %v", domain.ErrDependency, err)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: request quotation: %v", domain.ErrDependency, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, response.Body)
		return ports.QuotationSnapshot{}, domain.ErrQuotationNotFound
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: quotation service returned HTTP %d", domain.ErrDependency, response.StatusCode)
	}
	var payload successResponse
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return ports.QuotationSnapshot{}, fmt.Errorf("%w: decode quotation: %v", domain.ErrDependency, err)
	}
	return ports.QuotationSnapshot{QuotationID: payload.Data.QuotationID, CustomerID: payload.Data.CustomerID,
		ProductURL: payload.Data.ProductURL, ProductName: payload.Data.ProductName, Quantity: payload.Data.Quantity,
		TotalAmountVND: payload.Data.TotalAmountVND, Status: payload.Data.Status, CreatedAt: payload.Data.CreatedAt}, nil
}
