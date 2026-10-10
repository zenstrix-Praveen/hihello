package auth

import (
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestOTPEmailSubjectsAndExpiry(t *testing.T) {
	tests := []struct{ purpose, subject string }{
		{"customer_registration", "Verify your Dbizcard account"},
		{"customer_password_reset", "Reset your Dbizcard password"},
		{"admin_password_reset", "Dbizcard administrator password reset"},
	}
	expiresAt := time.Now().Add(10 * time.Minute)
	for _, test := range tests {
		subject, body, err := otpEmailContent(test.purpose, "123456", expiresAt)
		if err != nil || subject != test.subject {
			t.Fatalf("purpose %s: subject=%q err=%v", test.purpose, subject, err)
		}
		for _, expected := range []string{"123456", "10 minutes", "Do not share", "Dbizcard"} {
			if !strings.Contains(body, expected) {
				t.Errorf("purpose %s body lacks %q", test.purpose, expected)
			}
		}
	}
	if _, _, err := otpEmailContent("unknown", "123456", expiresAt); err == nil {
		t.Fatal("unsupported purpose must not create an email")
	}
}

func TestSMTPAuthRequiresTLS(t *testing.T) {
	auth := securePlainAuth{username: "user", password: "secret"}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false}); err == nil {
		t.Fatal("authentication must reject an unencrypted SMTP session")
	}
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true}); err != nil {
		t.Fatalf("STARTTLS session should authenticate: %v", err)
	}
	auth.implicitTLS = true
	if _, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false}); err != nil {
		t.Fatalf("implicit TLS session should authenticate: %v", err)
	}
}

func TestSMTPConfigRequiresAllCredentials(t *testing.T) {
	for _, key := range []string{"MAIL_HOST", "MAIL_PORT", "MAIL_USERNAME", "MAIL_PASSWORD", "MAIL_FROM_ADDRESS", "MAIL_FROM_NAME", "MAIL_FROM_EMAIL"} {
		t.Setenv(key, "")
	}
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "pass")
	t.Setenv("SMTP_FROM_EMAIL", "verify@example.com")
	t.Setenv("SMTP_FROM_NAME", "Digital Card Platform")
	if _, err := smtpConfigFromEnv(); err != nil {
		t.Fatalf("complete SMTP config rejected: %v", err)
	}
	t.Setenv("SMTP_PASSWORD", "")
	if _, err := smtpConfigFromEnv(); err == nil {
		t.Fatal("incomplete SMTP credentials must be rejected")
	}
}

func TestSMTPConfigSupportsExistingLaravelMailVariables(t *testing.T) {
	for _, key := range []string{"SMTP_HOST", "SMTP_PORT", "SMTP_USERNAME", "SMTP_PASSWORD", "SMTP_FROM_EMAIL", "SMTP_FROM_NAME"} {
		t.Setenv(key, "")
	}
	t.Setenv("MAIL_HOST", "smtp.zeptomail.in")
	t.Setenv("MAIL_PORT", "587")
	t.Setenv("MAIL_USERNAME", "configured-user")
	t.Setenv("MAIL_PASSWORD", "configured-secret")
	t.Setenv("MAIL_FROM_ADDRESS", "verified@example.test")
	t.Setenv("MAIL_FROM_NAME", "Dbizcard")
	cfg, err := smtpConfigFromEnv()
	if err != nil {
		t.Fatalf("existing MAIL_* configuration rejected: %v", err)
	}
	if cfg.host != "smtp.zeptomail.in" || cfg.port != 587 || cfg.from != "verified@example.test" || cfg.fromName != "Dbizcard" {
		t.Fatalf("MAIL_* configuration was not mapped correctly")
	}
}

func TestCanonicalSMTPVariablesTakePrecedence(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.zeptomail.in")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "canonical-user")
	t.Setenv("SMTP_PASSWORD", "canonical-secret")
	t.Setenv("SMTP_FROM_EMAIL", "canonical@example.test")
	t.Setenv("SMTP_FROM_NAME", "Dbizcard")
	t.Setenv("MAIL_HOST", "smtp.other.test")
	t.Setenv("MAIL_PORT", "465")
	t.Setenv("MAIL_USERNAME", "other-user")
	t.Setenv("MAIL_PASSWORD", "other-secret")
	t.Setenv("MAIL_FROM_ADDRESS", "other@example.test")
	t.Setenv("MAIL_FROM_NAME", "Other")
	cfg, err := smtpConfigFromEnv()
	if err != nil {
		t.Fatalf("canonical SMTP configuration rejected: %v", err)
	}
	if cfg.host != "smtp.zeptomail.in" || cfg.port != 587 || cfg.username != "canonical-user" || cfg.from != "canonical@example.test" {
		t.Fatal("canonical SMTP_* values should take precedence")
	}
}

func TestProductionSMTPRejectsPlaceholders(t *testing.T) {
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "operator")
	t.Setenv("SMTP_PASSWORD", "strong-private-value")
	t.Setenv("SMTP_FROM_EMAIL", "verify@example.com")
	t.Setenv("SMTP_FROM_NAME", "Digital Card Platform")
	if err := ValidateProductionSMTPConfig(); err == nil {
		t.Fatal("production SMTP configuration must reject example addresses")
	}
	t.Setenv("SMTP_HOST", "smtp.mail-provider.invalid")
	t.Setenv("SMTP_FROM_EMAIL", "verify@platform.invalid")
	if err := ValidateProductionSMTPConfig(); err != nil {
		t.Fatalf("complete non-placeholder configuration rejected: %v", err)
	}
}
