package main

import "testing"

import userauth "digital-card-platform/backend/internal/auth"

func setProductionEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "a-valid-test-only-secret-with-more-than-32-characters")
	t.Setenv("PAYMENT_PROVIDER", "razorpay")
	t.Setenv("RAZORPAY_MODE", "test")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_unitkey")
	t.Setenv("RAZORPAY_KEY_SECRET", "test-secret")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "test-webhook-secret")
	t.Setenv("FRONTEND_URL", "https://cards.example.test")
	t.Setenv("PUBLIC_WEB_URL", "https://cards.example.test")
	t.Setenv("API_PUBLIC_URL", "https://cards.example.test")
	t.Setenv("SMTP_HOST", "smtp.test.invalid")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "operator")
	t.Setenv("SMTP_PASSWORD", "test-only-smtp-password")
	t.Setenv("SMTP_FROM_EMAIL", "verify@test.invalid")
	t.Setenv("SMTP_FROM_NAME", "Digital Card Platform")
}

func TestValidateProductionConfigAcceptsTestModeOnly(t *testing.T) {
	setProductionEnv(t)
	if err := validateProductionConfig(); err != nil {
		t.Fatalf("valid production TEST config rejected: %v", err)
	}
}

func TestValidateProductionConfigRejectsMockAndInsecureOrigin(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("PAYMENT_PROVIDER", "mock")
	if err := validateProductionConfig(); err == nil {
		t.Fatal("mock payments must be rejected in production")
	}
	setProductionEnv(t)
	t.Setenv("FRONTEND_URL", "http://localhost:3000")
	if err := validateProductionConfig(); err == nil {
		t.Fatal("non-HTTPS production origin must be rejected")
	}
}

func TestValidateProductionConfigRejectsPlaceholderOriginAndSMTP(t *testing.T) {
	setProductionEnv(t)
	t.Setenv("PUBLIC_WEB_URL", "https://YOUR_DOMAIN")
	if err := validateProductionConfig(); err == nil {
		t.Fatal("placeholder production origins must be rejected")
	}
	setProductionEnv(t)
	t.Setenv("SMTP_PASSWORD", "replace_with_smtp_password")
	if err := userauth.ValidateProductionSMTPConfig(); err == nil {
		t.Fatal("placeholder production SMTP credentials must be rejected")
	}
}

func TestFeatureItemsDoNotUseNamedCMSSectionAlias(t *testing.T) {
	for _, section := range namedWebsiteSectionAliases() {
		if section == "features" {
			t.Fatal("features CMS alias shadows the /website/features item-list route")
		}
	}
}
