package payments

import (
	"testing"
	"time"
)

func TestClientRejectsPlaceholderAndWrongModeKey(t *testing.T) {
	t.Setenv("PAYMENT_PROVIDER", "razorpay")
	t.Setenv("APP_ENV", "local")
	t.Setenv("RAZORPAY_MODE", "test")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "test-webhook-secret")
	t.Setenv("RAZORPAY_KEY_SECRET", "replace_with_secret")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_public")
	if _, err := (&Handler{}).client(); err == nil {
		t.Fatal("placeholder accepted")
	}
	t.Setenv("RAZORPAY_KEY_SECRET", "test-secret")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_live_public")
	if _, err := (&Handler{}).client(); err == nil {
		t.Fatal("live key accepted in test mode")
	}
}

func TestFirstActiveIgnoresExpiredSubscription(t *testing.T) {
	expired := time.Now().Add(-time.Hour)
	if firstActive([]map[string]any{{"status": "active", "ends_at": &expired}}) != nil {
		t.Fatal("expired subscription treated as current")
	}
}

func TestCurrentSubscriptionIncludesValidTrial(t *testing.T) {
	end := time.Now().Add(time.Hour)
	trial := map[string]any{"status": "trial", "trial_ends_at": &end}
	if firstActive([]map[string]any{trial}) == nil {
		t.Fatal("valid trial omitted from dashboard")
	}
	end = time.Now().Add(-time.Hour)
	if firstActive([]map[string]any{trial}) != nil {
		t.Fatal("expired trial shown as current")
	}
}

func TestRenewalDateClampsMonthAndLeapYear(t *testing.T) {
	for _, tc := range []struct{ start, period, want string }{{"2026-01-31", "monthly", "2026-02-28"}, {"2028-02-29", "yearly", "2029-02-28"}} {
		start, _ := time.Parse("2006-01-02", tc.start)
		end := subscriptionPeriodEnd(start, tc.period)
		if end == nil || end.Format("2006-01-02") != tc.want {
			t.Fatalf("%s %s renewal=%v want %s", tc.start, tc.period, end, tc.want)
		}
	}
}

func TestClientRequiresExplicitTestMode(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("PAYMENT_PROVIDER", "razorpay")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "test-webhook-secret")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_public")
	t.Setenv("RAZORPAY_KEY_SECRET", "test-secret")
	t.Setenv("RAZORPAY_MODE", "")
	if _, err := (&Handler{}).client(); err == nil {
		t.Fatal("expected checkout to reject an unset mode")
	}
	t.Setenv("RAZORPAY_MODE", "live")
	if _, err := (&Handler{}).client(); err == nil {
		t.Fatal("expected checkout to reject live mode")
	}
	t.Setenv("RAZORPAY_MODE", "test")
	client, err := (&Handler{}).client()
	if err != nil {
		t.Fatalf("test mode should be accepted: %v", err)
	}
	if client.KeyID != "rzp_test_public" {
		t.Fatalf("unexpected public key id: %q", client.KeyID)
	}
}

func TestMockPaymentsAreLocalOnly(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("PAYMENT_PROVIDER", "mock")
	if !mockPaymentsEnabled() {
		t.Fatal("local mock payment mode should be enabled")
	}
	t.Setenv("APP_ENV", "production")
	if mockPaymentsEnabled() {
		t.Fatal("mock payments must be disabled outside local APP_ENV")
	}
	t.Setenv("APP_ENV", "local")
	t.Setenv("PAYMENT_PROVIDER", "razorpay")
	if mockPaymentsEnabled() {
		t.Fatal("mock endpoints must be disabled when Razorpay is selected")
	}
}

func TestPaymentDeletionPolicy(t *testing.T) {
	for _, status := range []string{"created", "pending", "failed", "cancelled"} {
		if !paymentRecordDeletable(status) {
			t.Errorf("status %q should be eligible for deletion when unlinked", status)
		}
	}
	for _, status := range []string{"authorized", "paid", "captured", "refunded", "partially_refunded", "active"} {
		if paymentRecordDeletable(status) {
			t.Errorf("status %q must be retained", status)
		}
	}
}
