package razorpay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestAmountToMinor(t *testing.T) {
	tests := []struct {
		amount, currency string
		want             int64
	}{
		{"199.00", "INR", 19900}, {"19.9", "USD", 1990}, {"1.234", "KWD", 1234}, {"1.234", "JOD", 1234}, {"100", "JPY", 100}, {"1.230", "BHD", 1230}, {"1.230", "OMR", 1230}, {"10.25", "CAD", 1025},
	}
	for _, tt := range tests {
		got, err := AmountToMinor(tt.amount, tt.currency)
		if err != nil || got != tt.want {
			t.Errorf("AmountToMinor(%q,%q) = %d, %v; want %d", tt.amount, tt.currency, got, err, tt.want)
		}
	}
	if _, err := AmountToMinor("1.001", "INR"); err == nil {
		t.Fatal("expected fractional precision error")
	}
	if _, err := AmountToMinor("1.01", "JPY"); err == nil {
		t.Fatal("fractional JPY must be rejected")
	}
}

func TestSignatures(t *testing.T) {
	secret := "test_secret"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("order_123|pay_123"))
	signature := hex.EncodeToString(mac.Sum(nil))
	if !VerifyCheckoutSignature("order_123", "pay_123", signature, secret) {
		t.Fatal("checkout signature should verify")
	}
	if VerifyCheckoutSignature("order_other", "pay_123", signature, secret) {
		t.Fatal("tampered order must fail verification")
	}
	body := []byte(`{"event":"payment.captured"}`)
	mac = hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	webhookSig := hex.EncodeToString(mac.Sum(nil))
	if !VerifyWebhookSignature(body, webhookSig, secret) {
		t.Fatal("webhook signature should verify")
	}
	if VerifyWebhookSignature(append(body, ' '), webhookSig, secret) {
		t.Fatal("altered body must fail verification")
	}
}
