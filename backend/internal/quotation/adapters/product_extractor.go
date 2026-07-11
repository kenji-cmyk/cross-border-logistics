package adapters

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
	"github.com/example/cross-border-logistics/internal/quotation/ports"
)

// DemoProductExtractor is deterministic and performs no outbound fetch. The
// URL query models metadata returned by an allowlisted marketplace adapter.
type DemoProductExtractor struct{}

func (DemoProductExtractor) Extract(_ context.Context, raw string) (ports.ExtractedProduct, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 2048 {
		return ports.ExtractedProduct{}, domain.ErrUnsafeProductURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" {
		return ports.ExtractedProduct{}, domain.ErrUnsafeProductURL
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" {
		return ports.ExtractedProduct{}, domain.ErrUnsafeProductURL
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.Equal(net.ParseIP("100.100.100.200"))) {
		return ports.ExtractedProduct{}, domain.ErrUnsafeProductURL
	}
	if host != "shop.example" && host != "example.com" {
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	q := u.Query()
	name, price, currency := strings.TrimSpace(q.Get("name")), strings.TrimSpace(q.Get("price")), strings.ToUpper(strings.TrimSpace(q.Get("currency")))
	if name == "" {
		name = "Demo marketplace product"
	}
	if price == "" {
		price = "50"
	}
	if currency == "" {
		currency = "USD"
	}
	if len(name) > 300 {
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	return ports.ExtractedProduct{URL: u.String(), Name: name, SourcePrice: price, Currency: currency, ImageURL: q.Get("image")}, nil
}
