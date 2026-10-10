package admin

import (
	"net/http/httptest"
	"testing"
)

func TestDefaultCardURLUsesConfiguredPublicWebOrigin(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", "https://cards.example.test/")
	t.Setenv("FRONTEND_URL", "https://fallback.example.test")
	r := httptest.NewRequest("GET", "https://api.example.test/n/token", nil)

	got := defaultCardURL(r, "jordan-example")
	if want := "https://cards.example.test/c/jordan-example"; got != want {
		t.Fatalf("defaultCardURL() = %q, want %q", got, want)
	}
}

func TestDefaultCardURLFallsBackToFrontendOrigin(t *testing.T) {
	t.Setenv("PUBLIC_WEB_URL", " ")
	t.Setenv("FRONTEND_URL", "http://localhost:3000/")
	r := httptest.NewRequest("GET", "http://api.example.test/n/token", nil)

	got := defaultCardURL(r, "jordan-example")
	if want := "http://localhost:3000/c/jordan-example"; got != want {
		t.Fatalf("defaultCardURL() = %q, want %q", got, want)
	}
}
