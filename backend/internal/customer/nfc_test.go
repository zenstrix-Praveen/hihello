package customer

import (
	"net/http/httptest"
	"testing"
)

func TestCustomerNFCRedirectURLUsesConfiguredPublicAPIOrigin(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", " https://api.example.test/ ")
	r := httptest.NewRequest("GET", "http://internal:8080/api/v1/user/nfc", nil)

	got := customerNFCRedirectURL(r, "tag-token")
	if want := "https://api.example.test/n/tag-token"; got != want {
		t.Fatalf("customerNFCRedirectURL() = %q, want %q", got, want)
	}
}

func TestCustomerNFCRedirectURLFallsBackWhenConfiguredOriginIsInvalid(t *testing.T) {
	t.Setenv("API_PUBLIC_URL", "localhost:8080/api/v1")
	r := httptest.NewRequest("GET", "http://localhost:8080/api/v1/user/nfc", nil)

	got := customerNFCRedirectURL(r, "tag-token")
	if want := "http://localhost:8080/n/tag-token"; got != want {
		t.Fatalf("customerNFCRedirectURL() = %q, want %q", got, want)
	}
}

func TestPublicAPIOriginRejectsNonOrigins(t *testing.T) {
	for _, value := range []string{
		"",
		"localhost:8080",
		"ftp://api.example.test",
		"https://user:pass@api.example.test",
		"https://api.example.test/api/v1",
		"https://api.example.test?host=other.example.test",
	} {
		if got := publicAPIOrigin(value); got != "" {
			t.Errorf("publicAPIOrigin(%q) = %q, want empty", value, got)
		}
	}
}
