package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/adapters"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
)

func TestDemoProductExtractorCompatibility(t *testing.T) {
	extractor := adapters.DemoProductExtractor{}
	product, err := extractor.Extract(context.Background(), "https://shop.example/item/1?name=Wireless%20Keyboard&price=50&currency=usd&image=https%3A%2F%2Fcdn.example%2Fkeyboard.png")
	if err != nil {
		t.Fatal(err)
	}
	if product.Name != "Wireless Keyboard" || product.SourcePrice != "50" || product.Currency != "USD" || product.ImageURL != "https://cdn.example/keyboard.png" {
		t.Fatalf("query metadata changed: %+v", product)
	}
	defaults, err := extractor.Extract(context.Background(), "https://example.com/product/1")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Name != "Demo marketplace product" || defaults.SourcePrice != "50" || defaults.Currency != "USD" {
		t.Fatalf("defaults changed: %+v", defaults)
	}
}

func TestDemoProductExtractorRejectsUnsafeDestinations(t *testing.T) {
	for _, raw := range []string{"https://localhost/item", "https://[::1]/item", "https://224.0.0.1/item", "https://user:pass@shop.example/item"} {
		if _, err := (adapters.DemoProductExtractor{}).Extract(context.Background(), raw); !errors.Is(err, domain.ErrUnsafeProductURL) {
			t.Fatalf("%s error=%v", raw, err)
		}
	}
}
