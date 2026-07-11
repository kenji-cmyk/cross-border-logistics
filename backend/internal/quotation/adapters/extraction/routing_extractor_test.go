package extraction

import (
	"context"
	"errors"
	"testing"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
	"github.com/example/cross-border-logistics/internal/quotation/ports"
)

type countingExtractor struct {
	calls int
	value ports.ExtractedProduct
	err   error
}

func (f *countingExtractor) Extract(context.Context, string) (ports.ExtractedProduct, error) {
	f.calls++
	return f.value, f.err
}

func TestRoutingProductExtractor(t *testing.T) {
	hosts, _ := NewHostMatcher([]string{"store.example"})
	demo := &countingExtractor{value: ports.ExtractedProduct{Name: "demo"}}
	real := &countingExtractor{value: ports.ExtractedProduct{Name: "real"}}
	router := NewRoutingProductExtractor(demo, real, hosts)

	product, err := router.Extract(context.Background(), "https://shop.example/item?price=50")
	if err != nil || product.Name != "demo" || demo.calls != 1 || real.calls != 0 {
		t.Fatalf("demo route: product=%+v err=%v calls=%d/%d", product, err, demo.calls, real.calls)
	}
	product, err = router.Extract(context.Background(), "https://store.example/item")
	if err != nil || product.Name != "real" || real.calls != 1 {
		t.Fatalf("real route: product=%+v err=%v calls=%d", product, err, real.calls)
	}
	_, err = router.Extract(context.Background(), "https://unsupported.example/item")
	if !errors.Is(err, domain.ErrExtractionUnavailable) {
		t.Fatalf("unsupported error = %v", err)
	}
}

func TestRoutingRejectsUnsafeURLBeforeCallingExtractors(t *testing.T) {
	hosts, _ := NewHostMatcher([]string{"127.0.0.1"})
	demo, real := &countingExtractor{}, &countingExtractor{}
	_, err := NewRoutingProductExtractor(demo, real, hosts).Extract(context.Background(), "https://127.0.0.1/item")
	if !errors.Is(err, domain.ErrUnsafeProductURL) || demo.calls != 0 || real.calls != 0 {
		t.Fatalf("err=%v calls=%d/%d", err, demo.calls, real.calls)
	}
}
