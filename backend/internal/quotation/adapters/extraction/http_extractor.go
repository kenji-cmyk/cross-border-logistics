package extraction

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/ports"
)

const productUserAgent = "cross-border-logistics-product-extractor/1.0"

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type HTTPStructuredProductExtractor struct {
	doer             HTTPDoer
	safety           *URLSafetyValidator
	maxResponseBytes int64
	logger           *slog.Logger
}

func NewHTTPStructuredProductExtractor(doer HTTPDoer, safety *URLSafetyValidator, maxResponseBytes int64, logger *slog.Logger) *HTTPStructuredProductExtractor {
	return &HTTPStructuredProductExtractor{doer: doer, safety: safety, maxResponseBytes: maxResponseBytes, logger: logger}
}

func (e *HTTPStructuredProductExtractor) Extract(ctx context.Context, raw string) (ports.ExtractedProduct, error) {
	started := time.Now()
	u, err := ParseProductURL(raw)
	if err != nil {
		return ports.ExtractedProduct{}, err
	}
	host := normalizeHost(u.Hostname())
	if err := e.safety.Validate(ctx, u); err != nil {
		e.log(ctx, host, 0, started, "", errorCategory(err))
		return ports.ExtractedProduct{}, unsafeError(err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ports.ExtractedProduct{}, domain.ErrUnsafeProductURL
	}
	request.Header.Set("Accept", "text/html,application/xhtml+xml")
	request.Header.Set("User-Agent", productUserAgent)
	response, err := e.doer.Do(request)
	if err != nil {
		e.log(ctx, host, 0, started, "", errorCategory(err))
		return ports.ExtractedProduct{}, unsafeError(err)
	}
	defer response.Body.Close()
	if response.Request == nil || response.Request.URL == nil {
		e.log(ctx, host, response.StatusCode, started, "", "invalid_response")
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	if err := e.safety.Validate(ctx, response.Request.URL); err != nil {
		e.log(ctx, host, response.StatusCode, started, "", errorCategory(err))
		return ports.ExtractedProduct{}, unsafeError(err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		e.log(ctx, host, response.StatusCode, started, "", "http_status")
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	contentType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (contentType != "text/html" && contentType != "application/xhtml+xml") {
		e.log(ctx, host, response.StatusCode, started, "", "content_type")
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, e.maxResponseBytes+1))
	if err != nil || int64(len(body)) > e.maxResponseBytes {
		e.log(ctx, host, response.StatusCode, started, "", "response_size")
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}

	finalURL := response.Request.URL
	data, strategy := parseMetadata(body, finalURL)
	if strategy == "" {
		e.log(ctx, host, response.StatusCode, started, "", "metadata")
		return ports.ExtractedProduct{}, domain.ErrExtractionUnavailable
	}
	productURL := finalURL
	if data.canonical != "" {
		if canonical, parseErr := ParseProductURL(data.canonical); parseErr == nil && e.safety.Validate(ctx, canonical) == nil {
			productURL = canonical
		}
	}
	productURL.Fragment = ""
	e.log(ctx, host, response.StatusCode, started, strategy, "")
	return ports.ExtractedProduct{
		URL:         productURL.String(),
		Name:        normalizeText(data.name),
		SourcePrice: strings.TrimSpace(data.price),
		Currency:    strings.ToUpper(strings.TrimSpace(data.currency)),
		ImageURL:    data.image,
	}, nil
}

func (e *HTTPStructuredProductExtractor) log(ctx context.Context, host string, status int, started time.Time, strategy, category string) {
	if e.logger == nil {
		return
	}
	attributes := []any{"host", host, "http_status", status, "duration_ms", time.Since(started).Milliseconds()}
	if strategy != "" {
		attributes = append(attributes, "extraction_strategy", strategy)
	}
	if category != "" {
		attributes = append(attributes, "error_category", category)
		e.logger.WarnContext(ctx, "product extraction failed", attributes...)
		return
	}
	e.logger.InfoContext(ctx, "product extraction completed", attributes...)
}

func errorCategory(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, domain.ErrUnsafeProductURL):
		return "unsafe_url"
	default:
		return fmt.Sprintf("dependency_%T", err)
	}
}
