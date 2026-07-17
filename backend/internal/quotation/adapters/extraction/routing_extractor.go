package extraction

import (
	"context"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/ports"
)

type RoutingProductExtractor struct {
	demo      ports.ProductExtractor
	http      ports.ProductExtractor
	realHosts HostMatcher
}

func NewRoutingProductExtractor(demo, httpExtractor ports.ProductExtractor, realHosts HostMatcher) *RoutingProductExtractor {
	return &RoutingProductExtractor{demo: demo, http: httpExtractor, realHosts: realHosts}
}

func (r *RoutingProductExtractor) Extract(ctx context.Context, raw string) (ports.ExtractedProduct, error) {
	u, err := ParseProductURL(raw)
	if err != nil {
		return ports.ExtractedProduct{}, err
	}
	host := normalizeHost(u.Hostname())
	if host == "shop.example" || host == "example.com" {
		return r.demo.Extract(ctx, raw)
	}
	if r.realHosts.Match(host) {
		return r.http.Extract(ctx, raw)
	}
	return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
}
