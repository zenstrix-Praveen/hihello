package admin

import "testing"

func TestNormalizePlanCurrencies(t *testing.T) {
	for _, code := range []string{"USD", "INR", "AED", "SAR", "QAR", "KWD", "BHD", "OMR", "EGP", "JOD", "EUR", "GBP", " aed "} {
		p := validPlanInput(`{}`)
		p.Currency = code
		if err := normalizePlan(&p); err != nil {
			t.Errorf("currency %q rejected: %v", code, err)
		}
	}
	for _, code := range []string{"", "ZZZ", "ABC", "US", "USDD", "123", "XXX"} {
		p := validPlanInput(`{}`)
		p.Currency = code
		if err := normalizePlan(&p); err == nil {
			t.Errorf("new currency %q accepted", code)
		}
	}
}

func TestNormalizePlanPreservesOnlyUnchangedLegacyCurrency(t *testing.T) {
	p := validPlanInput(`{}`)
	p.Currency = "ZZZ"
	if err := normalizePlanWithExistingCurrency(&p, "ZZZ"); err != nil {
		t.Fatal(err)
	}
	if p.Currency != "ZZZ" {
		t.Fatalf("legacy currency changed to %q", p.Currency)
	}
	p.Currency = "ABC"
	if err := normalizePlanWithExistingCurrency(&p, "ZZZ"); err == nil {
		t.Fatal("changed legacy currency accepted")
	}
	p.Currency = "AED"
	if err := normalizePlanWithExistingCurrency(&p, "ZZZ"); err != nil {
		t.Fatal(err)
	}
}
