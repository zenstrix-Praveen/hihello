package payments

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"digital-card-platform/backend/internal/currency"
)

// All application catalog codes appear in Razorpay's published support list:
// https://raw.githubusercontent.com/razorpay/markdown-docs/master/payments/international-payments.md
// Published support is not account enablement. A deployment must explicitly
// allow confirmed account currencies; an unset/blank setting permits INR only.
func SupportedCurrencies(provider string) []string {
	if provider == "mock" {
		return currency.Codes()
	}
	if provider != "razorpay" {
		return []string{}
	}
	configured := strings.TrimSpace(os.Getenv("RAZORPAY_SUPPORTED_CURRENCIES"))
	if scope := strings.TrimSpace(os.Getenv("RAZORPAY_CURRENCY_ALLOWLIST_MODE")); scope != "" && scope != strings.TrimSpace(os.Getenv("RAZORPAY_MODE")) {
		return []string{}
	}
	if configured == "" {
		return []string{"INR"}
	}
	allowed := make(map[string]bool)
	for _, code := range strings.Split(configured, ",") {
		code = currency.Normalize(code)
		if currency.Supported(code) && publishedRazorpayCurrency(code) {
			allowed[code] = true
		}
	}
	result := []string{}
	for _, code := range currency.Codes() {
		if allowed[code] {
			result = append(result, code)
		}
	}
	return result
}

func CurrencyConfigurationSource(provider string) string {
	if scope := strings.TrimSpace(os.Getenv("RAZORPAY_CURRENCY_ALLOWLIST_MODE")); provider == "razorpay" && scope != "" && scope != strings.TrimSpace(os.Getenv("RAZORPAY_MODE")) {
		return "currency allowlist does not match payment mode"
	}
	if provider != "razorpay" {
		return "provider configuration"
	}
	if strings.TrimSpace(os.Getenv("RAZORPAY_SUPPORTED_CURRENCIES")) == "" {
		return "safe INR-only default (RAZORPAY_SUPPORTED_CURRENCIES is unset)"
	}
	return "RAZORPAY_SUPPORTED_CURRENCIES environment allowlist"
}

func normalizeConfiguredCurrencies(codes []string) ([]string, error) {
	allowed := make(map[string]bool, len(codes))
	for _, raw := range codes {
		code := currency.Normalize(raw)
		if !currency.Supported(code) || !publishedRazorpayCurrency(code) {
			return nil, fmt.Errorf("unsupported currency code: %s", code)
		}
		allowed[code] = true
	}
	result := make([]string, 0, len(allowed))
	for _, item := range currency.Codes() {
		if allowed[item] {
			result = append(result, item)
		}
	}
	return result, nil
}

func checkoutCurrencySupported(provider, code string) bool {
	code = currency.Normalize(code)
	for _, allowed := range SupportedCurrencies(provider) {
		if code == allowed {
			return true
		}
	}
	return false
}

// Snapshot source: Razorpay official markdown-docs payments/international-payments.md.
// This is published product support, not merchant or live-mode enablement.
//
//go:embed razorpay-currencies.json
var publishedCurrenciesJSON []byte

func publishedRazorpayCurrency(code string) bool {
	var codes []string
	if json.Unmarshal(publishedCurrenciesJSON, &codes) != nil {
		return false
	}
	return contains(codes, currency.Normalize(code))
}
