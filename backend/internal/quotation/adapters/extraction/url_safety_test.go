package extraction

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/example/cross-border-logistics/internal/quotation/domain"
)

type fakeResolver map[string][]string

func (f fakeResolver) LookupIPAddr(_ context.Context, host string) ([]net.IPAddr, error) {
	values, ok := f[normalizeHost(host)]
	if !ok {
		return nil, errors.New("not found")
	}
	result := make([]net.IPAddr, 0, len(values))
	for _, value := range values {
		result = append(result, net.IPAddr{IP: net.ParseIP(value)})
	}
	return result, nil
}

func TestParseProductURLRejectsUnsafeForms(t *testing.T) {
	urls := []string{
		"http://store.example/item",
		"https://user:pass@store.example/item",
		"https://127.0.0.1/item",
		"https://[::1]/item",
		"https://10.0.0.1/item",
		"https://172.16.0.1/item",
		"https://192.168.0.1/item",
		"https://[::ffff:10.0.0.1]/item",
		"https://[fe80::1]/item",
		"https://169.254.169.254/latest/meta-data",
		"https://224.0.0.1/item",
		"https://store.example:bad/item",
	}
	for _, raw := range urls {
		if _, err := ParseProductURL(raw); !errors.Is(err, domain.ErrUnsafeProductURL) {
			t.Fatalf("%s error = %v", raw, err)
		}
	}
}

func TestURLSafetyRejectsPrivateAndMixedDNSAnswers(t *testing.T) {
	hosts, _ := NewHostMatcher([]string{"store.example"})
	for _, answers := range [][]string{{"10.0.0.1"}, {"93.184.216.34", "192.168.1.10"}} {
		validator := NewURLSafetyValidator(hosts, fakeResolver{"store.example": answers})
		u, _ := ParseProductURL("https://store.example/item")
		if err := validator.Validate(context.Background(), u); !errors.Is(err, domain.ErrUnsafeProductURL) {
			t.Fatalf("answers=%v error=%v", answers, err)
		}
	}
}

func TestURLSafetyAcceptsConfiguredPublicHost(t *testing.T) {
	hosts, _ := NewHostMatcher([]string{"store.example"})
	validator := NewURLSafetyValidator(hosts, fakeResolver{"store.example": {"93.184.216.34"}})
	u, _ := ParseProductURL("https://store.example/item")
	if err := validator.Validate(context.Background(), u); err != nil {
		t.Fatal(err)
	}
}
