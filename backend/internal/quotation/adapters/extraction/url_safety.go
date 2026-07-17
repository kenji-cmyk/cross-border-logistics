package extraction

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/kenji-cmyk/cross-border-logistics/internal/quotation/domain"
)

type DNSResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type URLSafetyValidator struct {
	hosts    HostMatcher
	resolver DNSResolver
}

func NewURLSafetyValidator(hosts HostMatcher, resolver DNSResolver) *URLSafetyValidator {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &URLSafetyValidator{hosts: hosts, resolver: resolver}
}

func ParseProductURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 2048 {
		return nil, domain.ErrUnsafeProductURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Hostname() == "" || !validHostname(normalizeHost(u.Hostname())) {
		return nil, domain.ErrUnsafeProductURL
	}
	if _, err := portForURLPort(u.Port()); err != nil {
		return nil, domain.ErrUnsafeProductURL
	}
	if isUnsafeIP(net.ParseIP(u.Hostname())) {
		return nil, domain.ErrUnsafeProductURL
	}
	return u, nil
}

func (v *URLSafetyValidator) Validate(ctx context.Context, u *url.URL) error {
	if u == nil {
		return domain.ErrUnsafeProductURL
	}
	parsed, err := ParseProductURL(u.String())
	if err != nil || !v.hosts.Match(parsed.Hostname()) {
		return domain.ErrUnsafeProductURL
	}
	return v.ValidateHost(ctx, parsed.Hostname())
}

func (v *URLSafetyValidator) ValidateHost(ctx context.Context, host string) error {
	_, err := v.Resolve(ctx, host)
	return err
}

func (v *URLSafetyValidator) Resolve(ctx context.Context, host string) ([]net.IPAddr, error) {
	host = normalizeHost(host)
	if !v.hosts.Match(host) {
		return nil, domain.ErrUnsafeProductURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return nil, domain.ErrUnsafeProductURL
		}
		return []net.IPAddr{{IP: ip}}, nil
	}
	addresses, err := v.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("resolve product host: %w", domain.ErrExtractionUnavailable)
	}
	for _, address := range addresses {
		if isUnsafeIP(address.IP) {
			return nil, domain.ErrUnsafeProductURL
		}
	}
	return addresses, nil
}

func isUnsafeIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	metadata := []string{"169.254.169.254", "100.100.100.200", "fd00:ec2::254"}
	for _, raw := range metadata {
		if ip.Equal(net.ParseIP(raw)) {
			return true
		}
	}
	return false
}

func unsafeError(err error) error {
	if errors.Is(err, domain.ErrUnsafeProductURL) {
		return domain.ErrUnsafeProductURL
	}
	return domain.ErrExtractionUnavailable
}
