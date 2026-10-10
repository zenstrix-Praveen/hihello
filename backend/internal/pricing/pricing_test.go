package pricing

import (
	"digital-card-platform/backend/internal/razorpay"
	"testing"
)

func TestResolveExactCurrencyCountryTaxAndPeriods(t *testing.T) {
	cfg, err := Parse([]byte(`{"providers":["razorpay"],"prices":[{"currency":"USD","country":"","billing_period":"monthly","amount":"10.00","tax_bps":0},{"currency":"USD","country":"US","billing_period":"monthly","amount":"9.99","tax_bps":750},{"currency":"USD","country":"","billing_period":"yearly","amount":"99.00","tax_bps":0}]}`))
	if err != nil {
		t.Fatal(err)
	}
	q, err := Resolve(cfg, "INR", "100", "monthly", "USD", "US", "monthly")
	if err != nil || q.Subtotal != "9.99" || q.Tax != "0.75" || q.Total != "10.74" || q.Minor != 1074 {
		t.Fatalf("quote=%+v error=%v", q, err)
	}
	q, err = Resolve(cfg, "INR", "100", "monthly", "USD", "CA", "yearly")
	if err != nil || q.Total != "99.00" {
		t.Fatalf("yearly %+v %v", q, err)
	}
	if _, err = Resolve(cfg, "INR", "100", "monthly", "EUR", "DE", "monthly"); err == nil {
		t.Fatal("Implicit FX fallback accepted")
	}
}
func TestGlobalPrecisionAndProviderRules(t *testing.T) {
	for _, tc := range []struct {
		code, amount string
		minor        int64
		accepted     bool
	}{
		{"INR", "0.50", 50, false}, {"INR", "1", 100, true}, {"USD", "1.23", 123, true}, {"EUR", "1.23", 123, true}, {"AED", "1.23", 123, true}, {"SAR", "1.23", 123, true}, {"QAR", "1.23", 123, true}, {"KWD", "1.230", 1230, true}, {"BHD", "1.230", 1230, true}, {"OMR", "1.230", 1230, true}, {"KWD", "1.234", 1234, false}, {"BHD", "1.234", 1234, false}, {"OMR", "1.234", 1234, false}, {"JPY", "123", 123, true}} {
		t.Run(tc.code+tc.amount, func(t *testing.T) {
			minor, err := Minor(tc.amount, tc.code)
			if err != nil || minor != tc.minor {
				t.Fatalf("minor=%d %v", minor, err)
			}
			_, err = razorpay.CheckoutAmountToMinor(tc.amount, tc.code)
			if (err == nil) != tc.accepted {
				t.Fatalf("provider acceptance=%v %v", tc.accepted, err)
			}
		})
	}
	for _, amount := range []string{"1.1", "1.01"} {
		if _, err := Minor(amount, "JPY"); err == nil {
			t.Fatal("Fractional JPY accepted")
		}
	}
	if _, err := Minor("1", "ZZZ"); err == nil {
		t.Fatal("Unknown currency accepted")
	}
}
func TestRoundingConfigIsExplicitAndExact(t *testing.T) {
	for _, tc := range []struct{ rounding, total string }{{"half_up", "1.240"}, {"down", "1.230"}, {"up", "1.240"}} {
		precision := 2
		c := Config{Providers: []string{"razorpay"}, Prices: []Price{{Currency: "KWD", Period: "monthly", Amount: "1.235", Precision: &precision, Rounding: tc.rounding}}}
		q, err := Resolve(c, "INR", "100", "monthly", "KWD", "", "monthly")
		if err != nil || q.Total != tc.total {
			t.Fatalf("%s: %+v %v", tc.rounding, q, err)
		}
	}
	c, err := Parse([]byte(`{"prices":[{"currency":"JPY","country":"","billing_period":"monthly","amount":"1.5"}]}`))
	_ = c
	if err == nil {
		t.Fatal("Invalid zero-decimal price accepted")
	}
	if _, err = Parse([]byte(`{"prices":[{"currency":"USD","country":"","billing_period":"monthly","amount":"1"},{"currency":"USD","country":"","billing_period":"monthly","amount":"2"}]}`)); err == nil {
		t.Fatal("Duplicate price accepted")
	}
}
