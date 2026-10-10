package payments

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// ValidateConfig never includes environment values in errors.
func ValidateConfig() error {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_PROVIDER")))
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE")))
	if mode != "test" && mode != "live" {
		return errors.New("RAZORPAY_MODE must explicitly be test or live")
	}
	if mode == "live" && (os.Getenv("APP_ENV") != "production" || provider != "razorpay") {
		return errors.New("live payments require APP_ENV=production and PAYMENT_PROVIDER=razorpay")
	}
	if provider == "mock" && mockPaymentsEnabled() {
		return nil
	}
	if provider != "razorpay" {
		return errors.New("PAYMENT_PROVIDER must be razorpay or local mock")
	}
	for _, key := range []string{"RAZORPAY_KEY_ID", "RAZORPAY_KEY_SECRET", "RAZORPAY_WEBHOOK_SECRET"} {
		value := strings.TrimSpace(os.Getenv(key))
		lower := strings.ToLower(value)
		if value == "" || strings.Contains(lower, "replace") || strings.Contains(lower, "your_") || strings.Contains(lower, "placeholder") || strings.Contains(lower, "changeme") || strings.Contains(lower, "change-me") || strings.Contains(lower, "example") {
			return errors.New(key + " must contain a non-placeholder value")
		}
	}
	if !strings.HasPrefix(strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")), "rzp_"+mode+"_") {
		return errors.New("RAZORPAY_KEY_ID does not match RAZORPAY_MODE")
	}
	if mode == "live" {
		u, err := url.Parse(strings.TrimSpace(os.Getenv("FRONTEND_URL")))
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" {
			return errors.New("live payments require an HTTPS FRONTEND_URL")
		}
	}
	return nil
}
