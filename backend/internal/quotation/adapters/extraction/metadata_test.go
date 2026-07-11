package extraction

import (
	"net/url"
	"strings"
	"testing"
)

func TestJSONLDProductFormats(t *testing.T) {
	tests := []struct {
		name, json, wantName, wantPrice, wantImage string
	}{
		{"basic offer and string image", `{"@context":"https://schema.org","@type":"Product","name":"  Wireless   Keyboard ","image":"/keyboard.jpg","offers":{"@type":"Offer","price":"50.25","priceCurrency":"usd"}}`, "Wireless Keyboard", "50.25", "https://store.example/keyboard.jpg"},
		{"graph and image array", `{"@graph":[{"@type":"Organization","name":"Seller"},{"@type":"Product","name":"Mouse","image":["/mouse.png","/other.png"],"offers":{"@type":"Offer","price":12,"priceCurrency":"USD"}}]}`, "Mouse", "12", "https://store.example/mouse.png"},
		{"offers array and image object", `{"@type":"Product","name":"Camera","image":{"@type":"ImageObject","contentUrl":"/camera.webp"},"offers":[{"@type":"Offer","price":"99.9900","priceCurrency":"JPY"}]}`, "Camera", "99.9900", "https://store.example/camera.webp"},
		{"aggregate offer low price", `{"@type":"Product","name":"Book","image":{"url":"/book.jpg"},"offers":{"@type":"AggregateOffer","lowPrice":"8.5","priceCurrency":"CNY"}}`, "Book", "8.5", "https://store.example/book.jpg"},
	}
	base, _ := url.Parse("https://store.example/products/1")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `<html><head><script type="application/ld+json">` + tt.json + `</script></head></html>`
			got, strategy := parseMetadata([]byte(body), base)
			if strategy != "json_ld" || got.name != tt.wantName || got.price != tt.wantPrice || got.image != tt.wantImage {
				t.Fatalf("strategy=%q metadata=%+v", strategy, got)
			}
		})
	}
}

func TestMultipleJSONLDBlocksUseCompleteProduct(t *testing.T) {
	base, _ := url.Parse("https://store.example/item")
	body := `<script type="application/ld+json">{"@type":"Product","name":"Incomplete"}</script>
<script type="application/ld+json">{"@type":"Product","name":"Complete","offers":{"price":"20","priceCurrency":"KRW"}}</script>`
	got, strategy := parseMetadata([]byte(body), base)
	if strategy != "json_ld" || got.name != "Complete" || got.price != "20" || got.currency != "KRW" {
		t.Fatalf("strategy=%q metadata=%+v", strategy, got)
	}
}

func TestOpenGraphAndHTMLFallback(t *testing.T) {
	base, _ := url.Parse("https://store.example/products/1")
	body := `<html><head>
<meta property="og:title" content=" Phone &amp; Case ">
<meta property="product:price:amount" content="15.75">
<meta property="product:price:currency" content="usd">
<meta property="og:image" content="../images/phone.jpg">
</head></html>`
	got, strategy := parseMetadata([]byte(body), base)
	if strategy != "open_graph" || got.name != "Phone & Case" || got.price != "15.75" || got.currency != "USD" || got.image != "https://store.example/images/phone.jpg" {
		t.Fatalf("strategy=%q metadata=%+v", strategy, got)
	}

	standard := `<title>Standard Product</title><meta itemprop="price" content="3"><meta itemprop="priceCurrency" content="JPY">`
	got, strategy = parseMetadata([]byte(standard), base)
	if strategy != "html_metadata" || got.name != "Standard Product" || got.price != "3" || got.currency != "JPY" {
		t.Fatalf("strategy=%q metadata=%+v", strategy, got)
	}
}

func TestMetadataValidationFailures(t *testing.T) {
	tests := []string{
		`{"@type":"Product","offers":{"price":"1","priceCurrency":"USD"}}`,
		`{"@type":"Product","name":"Missing price","offers":{"priceCurrency":"USD"}}`,
		`{"@type":"Product","name":"Zero","offers":{"price":"0","priceCurrency":"USD"}}`,
		`{"@type":"Product","name":"Negative","offers":{"price":"-1","priceCurrency":"USD"}}`,
		`{"@type":"Product","name":"Invalid","offers":{"price":"1,20","priceCurrency":"USD"}}`,
		`{"@type":"Product","name":"Missing currency","offers":{"price":"1"}}`,
	}
	base, _ := url.Parse("https://store.example/item")
	for _, data := range tests {
		body := `<script type="application/ld+json">` + data + `</script>`
		if got, strategy := parseMetadata([]byte(body), base); strategy != "" || strings.TrimSpace(got.name) != "" {
			t.Fatalf("expected failure for %s, strategy=%q metadata=%+v", data, strategy, got)
		}
	}
}
