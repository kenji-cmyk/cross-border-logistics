package extraction

import (
	"fmt"
	"net"
	"strings"
)

type HostMatcher struct {
	exact     map[string]struct{}
	wildcards []string
	patterns  []string
}

func NewHostMatcher(patterns []string) (HostMatcher, error) {
	m := HostMatcher{exact: make(map[string]struct{})}
	for _, raw := range patterns {
		pattern := normalizeHost(raw)
		if pattern == "" {
			return HostMatcher{}, fmt.Errorf("host entries must not be empty")
		}
		if strings.ContainsAny(pattern, "/:@?#") {
			return HostMatcher{}, fmt.Errorf("%q must be a hostname without scheme, port, path, or credentials", raw)
		}
		if strings.HasPrefix(pattern, "*.") {
			base := strings.TrimPrefix(pattern, "*.")
			if !validHostname(base) || net.ParseIP(base) != nil {
				return HostMatcher{}, fmt.Errorf("%q is not a valid wildcard hostname", raw)
			}
			m.wildcards = append(m.wildcards, base)
			m.patterns = append(m.patterns, "*."+base)
			continue
		}
		if strings.Contains(pattern, "*") || !validHostname(pattern) {
			return HostMatcher{}, fmt.Errorf("%q is not a valid hostname", raw)
		}
		if _, exists := m.exact[pattern]; !exists {
			m.exact[pattern] = struct{}{}
			m.patterns = append(m.patterns, pattern)
		}
	}
	return m, nil
}

func (m HostMatcher) Match(host string) bool {
	host = normalizeHost(host)
	if _, ok := m.exact[host]; ok {
		return true
	}
	for _, base := range m.wildcards {
		if strings.HasSuffix(host, "."+base) && host != base {
			return true
		}
	}
	return false
}

func (m HostMatcher) Patterns() []string {
	return append([]string(nil), m.patterns...)
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func validHostname(host string) bool {
	if ip := net.ParseIP(host); ip != nil {
		return true
	}
	if host == "" || len(host) > 253 || host == "localhost" {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}
