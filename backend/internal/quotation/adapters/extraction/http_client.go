package extraction

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

func NewSafeHTTPClient(timeout time.Duration, maxRedirects int, safety *URLSafetyValidator) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, domain.ErrUnsafeProductURL
		}
		addresses, err := safety.Resolve(ctx, host)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, candidate := range addresses {
			target := net.JoinHostPort(candidate.IP.String(), port)
			conn, dialErr := dialer.DialContext(ctx, network, target)
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, fmt.Errorf("connect to validated product host on port %s (%d addresses): %w", port, len(addresses), lastErr)
	}
	return newHTTPClient(timeout, maxRedirects, safety, transport)
}

func newHTTPClient(timeout time.Duration, maxRedirects int, safety *URLSafetyValidator, transport http.RoundTripper) *http.Client {
	return &http.Client{
		Timeout: timeout, Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return fmt.Errorf("maximum redirects exceeded: %w", domain.ErrExtractionUnavailable)
			}
			if err := safety.Validate(req.Context(), req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

func portForURLPort(raw string) (int, error) {
	if raw == "" {
		return 443, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1 || port > 65535 {
		return 0, domain.ErrUnsafeProductURL
	}
	return port, nil
}
