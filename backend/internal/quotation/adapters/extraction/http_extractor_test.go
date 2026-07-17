package extraction

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func testSafety(patterns ...string) *URLSafetyValidator {
	hosts, _ := NewHostMatcher(patterns)
	addresses := fakeResolver{}
	for _, pattern := range patterns {
		if !strings.HasPrefix(pattern, "*.") {
			addresses[pattern] = []string{"93.184.216.34"}
		}
	}
	return NewURLSafetyValidator(hosts, addresses)
}

func htmlResponse(request *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func TestHTTPExtractorJSONLDAndCanonicalURL(t *testing.T) {
	safety := testSafety("store.example")
	doer := doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("User-Agent") != productUserAgent || request.Header.Get("Accept") == "" {
			t.Fatalf("safe request headers missing: %v", request.Header)
		}
		body := `<link rel="canonical" href="/canonical/1"><script type="application/ld+json">{"@type":"Product","name":"Keyboard","image":"/image.jpg","offers":{"price":"50.125","priceCurrency":"usd"}}</script>`
		return htmlResponse(request, body), nil
	})
	product, err := NewHTTPStructuredProductExtractor(doer, safety, 1024, nil).Extract(context.Background(), "https://store.example/item/1?secret=not-logged")
	if err != nil {
		t.Fatal(err)
	}
	if product.URL != "https://store.example/canonical/1" || product.Name != "Keyboard" || product.SourcePrice != "50.125" || product.Currency != "USD" || product.ImageURL != "https://store.example/image.jpg" {
		t.Fatalf("product=%+v", product)
	}
}

func TestHTTPExtractorFailureMapping(t *testing.T) {
	tests := []struct {
		name     string
		maxBytes int64
		doer     doerFunc
	}{
		{"non HTML", 1024, func(request *http.Request) (*http.Response, error) {
			response := htmlResponse(request, "binary")
			response.Header.Set("Content-Type", "image/png")
			return response, nil
		}},
		{"oversized body", 8, func(request *http.Request) (*http.Response, error) {
			return htmlResponse(request, strings.Repeat("x", 9)), nil
		}},
		{"timeout", 1024, func(*http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		}},
		{"HTTP failure", 1024, func(request *http.Request) (*http.Response, error) {
			response := htmlResponse(request, "")
			response.StatusCode = http.StatusTooManyRequests
			return response, nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewHTTPStructuredProductExtractor(tt.doer, testSafety("store.example"), tt.maxBytes, nil).Extract(context.Background(), "https://store.example/item")
			if !errors.Is(err, domain.ErrExtractionUnavailable) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestSafeHTTPClientRejectsUnsafeRedirect(t *testing.T) {
	safety := testSafety("store.example")
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{"https://127.0.0.1/private"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    request,
		}, nil
	})
	client := newHTTPClient(time.Second, 3, safety, transport)
	_, err := NewHTTPStructuredProductExtractor(client, safety, 1024, nil).Extract(context.Background(), "https://store.example/item")
	if !errors.Is(err, domain.ErrUnsafeProductURL) {
		t.Fatalf("error=%v", err)
	}
}

func TestSafeHTTPClientRejectsRedirectToUnsupportedHost(t *testing.T) {
	safety := testSafety("store.example")
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{"https://other.example/item"}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})
	client := newHTTPClient(time.Second, 3, safety, transport)
	_, err := NewHTTPStructuredProductExtractor(client, safety, 1024, nil).Extract(context.Background(), "https://store.example/item")
	if !errors.Is(err, domain.ErrUnsafeProductURL) {
		t.Fatalf("error=%v", err)
	}
}

func TestSafeHTTPClientEnforcesRedirectLimit(t *testing.T) {
	safety := testSafety("store.example")
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		current, _ := url.Parse(request.URL.String())
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": []string{current.String()}}, Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})
	client := newHTTPClient(time.Second, 2, safety, transport)
	_, err := NewHTTPStructuredProductExtractor(client, safety, 1024, nil).Extract(context.Background(), "https://store.example/item")
	if !errors.Is(err, domain.ErrExtractionUnavailable) {
		t.Fatalf("error=%v", err)
	}
}
