package payments

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestProviderMethodsCountryCurrencyAndMinimum(t *testing.T) {
	c := GlobalConfig{MerchantConfirmed: true, Capabilities: map[string]Capability{"razorpay": {MinimumMinor: map[string]int64{"USD": 100, "INR": 100}, Methods: []Method{{ID: "card", Currencies: []string{"INR", "USD"}, Countries: []string{"IN", "US"}}, {ID: "upi", Currencies: []string{"INR"}, Countries: []string{"IN"}}}, Renewal: "manual"}}}
	if err := validateGlobal(c); err != nil {
		t.Fatal(err)
	}
	m, e := c.Check("razorpay", "INR", "IN", 100)
	if e != nil || len(m) != 2 {
		t.Fatalf("%v %v", m, e)
	}
	m, e = c.Check("razorpay", "USD", "US", 100)
	if e != nil || len(m) != 1 || m[0] != "card" {
		t.Fatalf("%v %v", m, e)
	}
	if _, e = c.Check("razorpay", "USD", "US", 99); e == nil {
		t.Fatal("Minimum not enforced")
	}
	if _, e = c.Check("razorpay", "USD", "AE", 100); e == nil {
		t.Fatal("Unsupported methods accepted")
	}
	c.DisabledCurrencies = []string{"USD"}
	if _, e = c.Check("razorpay", "USD", "US", 100); e == nil {
		t.Fatal("Inactive currency accepted")
	}
}
func TestUnsupportedMethodDeclarationsDoNotActivateProviders(t *testing.T) {
	c := GlobalConfig{MerchantConfirmed: true, Capabilities: map[string]Capability{"razorpay": {Methods: []Method{{ID: "upi", Currencies: []string{"USD"}, Countries: []string{"US"}}}}}}
	if validateGlobal(c) == nil {
		t.Fatal("International UPI forced")
	}
	c.Capabilities = map[string]Capability{"stripe": {Renewal: "manual"}}
	if err := validateGlobal(c); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PAYMENT_PROVIDER", "stripe")
	w := httptest.NewRecorder()
	(&Handler{}).Provider(w, httptest.NewRequest("GET", "/", nil))
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	if v["provider"] != "unavailable" {
		t.Fatal("New provider activated")
	}
}
func TestLocationOnlySuggestsBehindTrustedProxy(t *testing.T) {
	for _, trusted := range []string{"false", "true"} {
		t.Setenv("BILLING_TRUST_LOCATION_HEADERS", trusted)
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("CF-IPCountry", "SA")
		w := httptest.NewRecorder()
		(&Handler{}).CurrencyCatalog(w, r)
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v)
		expected := ""
		if trusted == "true" {
			expected = "SAR"
		}
		if v["suggested_currency"] != expected {
			t.Fatalf("%v", v)
		}
	}
}
