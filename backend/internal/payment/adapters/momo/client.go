package momo

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/cross-border-logistics/internal/payment/ports"
)

const maxResponseBytes = 1 << 20

type Config struct {
	PartnerCode, AccessKey, SecretKey, BaseURL, IPNURL, RedirectURL string
	Timeout                                                         time.Duration
}
type Client struct {
	cfg  Config
	http *http.Client
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.PartnerCode) == "" || strings.TrimSpace(cfg.AccessKey) == "" || strings.TrimSpace(cfg.SecretKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" || strings.TrimSpace(cfg.IPNURL) == "" || strings.TrimSpace(cfg.RedirectURL) == "" {
		return nil, fmt.Errorf("incomplete MoMo configuration")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}, nil
}

func sign(secret, raw string) string {
	m := hmac.New(sha256.New, []byte(secret))
	_, _ = m.Write([]byte(raw))
	return hex.EncodeToString(m.Sum(nil))
}
func (c *Client) CreateTransaction(ctx context.Context, orderID, requestID string, amount int64, currency string) (ports.GatewayTransaction, error) {
	if amount <= 0 || currency != "VND" {
		return ports.GatewayTransaction{}, fmt.Errorf("invalid MoMo payment")
	}
	extra := ""
	raw := fmt.Sprintf("accessKey=%s&amount=%d&extraData=%s&ipnUrl=%s&orderId=%s&orderInfo=%s&partnerCode=%s&redirectUrl=%s&requestId=%s&requestType=captureWallet", c.cfg.AccessKey, amount, extra, c.cfg.IPNURL, orderID, "CrossBorder payment "+orderID, c.cfg.PartnerCode, c.cfg.RedirectURL, requestID)
	body := map[string]any{"partnerCode": c.cfg.PartnerCode, "partnerName": "CrossBorder", "storeId": "CrossBorder", "requestId": requestID, "amount": amount, "orderId": orderID, "orderInfo": "CrossBorder payment " + orderID, "redirectUrl": c.cfg.RedirectURL, "ipnUrl": c.cfg.IPNURL, "lang": "vi", "requestType": "captureWallet", "autoCapture": true, "extraData": extra, "signature": sign(c.cfg.SecretKey, raw)}
	var out struct {
		OrderID    string `json:"orderId"`
		RequestID  string `json:"requestId"`
		PayURL     string `json:"payUrl"`
		Message    string `json:"message"`
		ResultCode int    `json:"resultCode"`
	}
	if err := c.post(ctx, "/v2/gateway/api/create", body, &out); err != nil {
		return ports.GatewayTransaction{}, err
	}
	if out.OrderID != orderID || out.RequestID != requestID || out.PayURL == "" || out.ResultCode != 0 {
		return ports.GatewayTransaction{}, fmt.Errorf("MoMo create rejected: code=%d message=%s", out.ResultCode, out.Message)
	}
	return ports.GatewayTransaction{Reference: orderID, HostedURL: out.PayURL, ResultCode: out.ResultCode, Message: out.Message}, nil
}
func (c *Client) QueryTransaction(ctx context.Context, orderID, requestID string) (ports.GatewayResult, error) {
	raw := fmt.Sprintf("accessKey=%s&orderId=%s&partnerCode=%s&requestId=%s", c.cfg.AccessKey, orderID, c.cfg.PartnerCode, requestID)
	return c.result(ctx, "/v2/gateway/api/query", map[string]any{"partnerCode": c.cfg.PartnerCode, "requestId": requestID, "orderId": orderID, "lang": "vi", "signature": sign(c.cfg.SecretKey, raw)})
}
func (c *Client) Refund(ctx context.Context, orderID, requestID, transID string, amount int64) (ports.GatewayResult, error) {
	tid, err := strconv.ParseInt(transID, 10, 64)
	if err != nil {
		return ports.GatewayResult{}, fmt.Errorf("invalid MoMo transaction id")
	}
	raw := fmt.Sprintf("accessKey=%s&amount=%d&description=%s&orderId=%s&partnerCode=%s&requestId=%s&transId=%d", c.cfg.AccessKey, amount, "CrossBorder refund", orderID, c.cfg.PartnerCode, requestID, tid)
	return c.result(ctx, "/v2/gateway/api/refund", map[string]any{"partnerCode": c.cfg.PartnerCode, "orderId": orderID, "requestId": requestID, "amount": amount, "transId": tid, "lang": "vi", "description": "CrossBorder refund", "signature": sign(c.cfg.SecretKey, raw)})
}
func (c *Client) QueryRefund(ctx context.Context, orderID, requestID string) (ports.GatewayResult, error) {
	raw := fmt.Sprintf("accessKey=%s&orderId=%s&partnerCode=%s&requestId=%s", c.cfg.AccessKey, orderID, c.cfg.PartnerCode, requestID)
	return c.result(ctx, "/v2/gateway/api/refund/query", map[string]any{"partnerCode": c.cfg.PartnerCode, "orderId": orderID, "requestId": requestID, "lang": "vi", "signature": sign(c.cfg.SecretKey, raw)})
}
func (c *Client) result(ctx context.Context, path string, body any) (ports.GatewayResult, error) {
	var out struct {
		TransID    int64  `json:"transId"`
		ResultCode int    `json:"resultCode"`
		Message    string `json:"message"`
	}
	if err := c.post(ctx, path, body, &out); err != nil {
		return ports.GatewayResult{}, err
	}
	return ports.GatewayResult{TransactionID: strconv.FormatInt(out.TransID, 10), ResultCode: out.ResultCode, Message: out.Message}, nil
}
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.cfg.BaseURL, "/")+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("MoMo request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return fmt.Errorf("MoMo response too large")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("MoMo HTTP status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode MoMo response: %w", err)
	}
	return nil
}

type IPN struct {
	PartnerCode  string `json:"partnerCode"`
	OrderID      string `json:"orderId"`
	RequestID    string `json:"requestId"`
	OrderInfo    string `json:"orderInfo"`
	OrderType    string `json:"orderType"`
	Message      string `json:"message"`
	PayType      string `json:"payType"`
	ExtraData    string `json:"extraData"`
	Signature    string `json:"signature"`
	Amount       int64  `json:"amount"`
	TransID      int64  `json:"transId"`
	ResponseTime int64  `json:"responseTime"`
	ResultCode   int    `json:"resultCode"`
}

func (c *Client) VerifyIPN(body []byte) (IPN, error) {
	var p IPN
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, err
	}
	raw := fmt.Sprintf("accessKey=%s&amount=%d&extraData=%s&message=%s&orderId=%s&orderInfo=%s&orderType=%s&partnerCode=%s&payType=%s&requestId=%s&responseTime=%d&resultCode=%d&transId=%d", c.cfg.AccessKey, p.Amount, p.ExtraData, p.Message, p.OrderID, p.OrderInfo, p.OrderType, p.PartnerCode, p.PayType, p.RequestID, p.ResponseTime, p.ResultCode, p.TransID)
	expected := sign(c.cfg.SecretKey, raw)
	if p.PartnerCode != c.cfg.PartnerCode || !hmac.Equal([]byte(expected), []byte(strings.ToLower(p.Signature))) {
		return p, fmt.Errorf("invalid MoMo IPN signature")
	}
	return p, nil
}
