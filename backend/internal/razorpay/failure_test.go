package razorpay

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type failureTransport func(*http.Request) (*http.Response, error)

func (f failureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProviderFailureClassificationAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		body    string
		network bool
		want    string
	}{
		{"network blocked", 0, "", true, "network"},
		{"bad credentials", 401, `{"error":{"code":"BAD_REQUEST_ERROR","description":"secret key123"}}`, false, "authentication"},
		{"merchant currency", 400, `{"error":{"code":"BAD_REQUEST_ERROR","description":"Currency is not supported"}}`, false, "currency"},
		{"provider unavailable", 503, `{}`, false, "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Client{KeyID: "key123", KeySecret: "secret", HTTP: &http.Client{Transport: failureTransport(func(*http.Request) (*http.Response, error) {
				if tc.network {
					return nil, errors.New("secret URL contains key123")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			_, err := c.CreateOrder(context.Background(), 2500, "INR", "qa")
			if err == nil {
				t.Fatal("failure was accepted")
			}
			if FailureKind(err) != tc.want {
				t.Fatalf("kind=%s want=%s", FailureKind(err), tc.want)
			}
			if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "key123") {
				t.Fatal("credential leaked")
			}
		})
	}
}
func TestCheckoutAmountPreservesPriceAndRejectsGatewayPrecision(t *testing.T) {
	for _, code := range []string{"KWD", "BHD", "OMR", "JOD"} {
		if _, err := CheckoutAmountToMinor("1.234", code); err == nil {
			t.Fatalf("%s silently accepted unchargeable exact price", code)
		}
		if n, err := CheckoutAmountToMinor("1.230", code); err != nil || n != 1230 {
			t.Fatalf("%s valid exact amount: %d %v", code, n, err)
		}
	}
}

func TestCheckoutAmountRejectsINRBelowRazorpayMinimum(t *testing.T) {
	if _, err := CheckoutAmountToMinor("0.50", "INR"); err == nil || !strings.Contains(err.Error(), "minimum") {
		t.Fatalf("INR 0.50 should be rejected with the gateway minimum, got %v", err)
	}
	if amount, err := CheckoutAmountToMinor("1.00", "INR"); err != nil || amount != 100 {
		t.Fatalf("INR 1.00 should be accepted as 100 paise, got %d, %v", amount, err)
	}
}