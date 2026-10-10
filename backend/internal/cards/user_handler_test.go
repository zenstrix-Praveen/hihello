package cards

import "testing"

func TestCustomerStatusAllowsOnlyCustomerStates(t *testing.T) {
	for _, status := range []string{"draft", "published", "archived"} {
		if !customerStatus(status) {
			t.Errorf("customerStatus(%q) = false, want true", status)
		}
	}
	for _, status := range []string{"", "suspended", "active", "deleted"} {
		if customerStatus(status) {
			t.Errorf("customerStatus(%q) = true, want false", status)
		}
	}
}

func TestValidCustomerURL(t *testing.T) {
	for _, value := range []string{"", "https://example.com", "http://localhost:3000/card", "https://example.com/path?q=1"} {
		if !validCustomerURL(value) {
			t.Errorf("validCustomerURL(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"javascript:alert(1)", "data:text/html,hi", "//example.com/path", "https://", "not a URL"} {
		if validCustomerURL(value) {
			t.Errorf("validCustomerURL(%q) = true, want false", value)
		}
	}
}

func TestValidateCustomerURLsRejectsInvalidLinks(t *testing.T) {
	in := adminCardInput{Website: "https://example.com", SocialLinks: []LinkObject{{"label": "LinkedIn", "url": "javascript:alert(1)"}}}
	if err := validateCustomerURLs(in); err == nil {
		t.Fatal("expected invalid social URL to be rejected")
	}
}
