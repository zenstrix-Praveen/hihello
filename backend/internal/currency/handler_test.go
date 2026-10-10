package currency

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestCurrencyCatalogHasSymbolsAndPrecision(t *testing.T) {
	catalog := Catalog()
	if len(catalog) < 150 {
		t.Fatalf("currency catalog has %d entries", len(catalog))
	}
	for _, code := range []string{"USD", "INR", "AED", "SAR", "QAR", "KWD", "BHD", "OMR", "EGP", "JOD", "EUR", "GBP"} {
		found := false
		for _, item := range catalog {
			if item.Code == code {
				found = item.Name != "" && item.Symbol != ""
			}
		}
		if !found {
			t.Errorf("missing metadata for %s", code)
		}
	}
	for _, code := range []string{"KWD", "BHD", "OMR", "JOD"} {
		if exponent, ok := Exponent(code); !ok || exponent != 3 {
			t.Errorf("%s exponent=(%d,%v), want 3", code, exponent, ok)
		}
	}
}

type stubProvider struct {
	rate     float64
	err      error
	from, to string
}

func (s *stubProvider) Rate(_ context.Context, from, to string) (float64, time.Time, error) {
	s.from, s.to = from, to
	return s.rate, time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC), s.err
}

func TestConvert(t *testing.T) {
	for _, tc := range []struct {
		name, amount, from, to string
		provider               *stubProvider
		status                 int
		rate, converted        float64
	}{
		{"AUD to INR", "1.07", "AUD", "INR", &stubProvider{rate: 60}, 200, 60, 64.2},
		{"USD to INR", "2", "USD", "INR", &stubProvider{rate: 83}, 200, 83, 166},
		{"INR to USD", "10", "INR", "USD", &stubProvider{rate: .012}, 200, .012, .12},
		{"same currency", "3.25", "INR", "INR", &stubProvider{}, 200, 1, 3.25},
		{"invalid amount", "0", "USD", "INR", &stubProvider{rate: 83}, 400, 0, 0},
		{"unsupported currency", "1", "ZZZ", "INR", &stubProvider{err: errors.New("unsupported currency")}, 422, 0, 0},
		{"provider unavailable", "1", "USD", "INR", &stubProvider{err: errors.New("network failure")}, 503, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{"amount": {tc.amount}, "from": {tc.from}, "to": {tc.to}}
			req := httptest.NewRequest("GET", "/api/v1/public/currency/convert?"+q.Encode(), nil)
			res := httptest.NewRecorder()
			Handler{Provider: tc.provider}.Convert(res, req)
			if res.Code != tc.status {
				t.Fatalf("status=%d body=%s want %d", res.Code, res.Body, tc.status)
			}
			if tc.status != 200 {
				return
			}
			var got Conversion
			if err := json.Unmarshal(res.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Rate != tc.rate || got.ConvertedAmount != tc.converted || got.From != tc.from || got.To != tc.to || got.UpdatedAt.IsZero() {
				t.Fatalf("unexpected conversion: %+v", got)
			}
			if tc.from != tc.to && (tc.provider.from != tc.from || tc.provider.to != tc.to) {
				t.Fatalf("provider called with %s -> %s", tc.provider.from, tc.provider.to)
			}
		})
	}
}

func TestConvertRejectsMalformedCurrency(t *testing.T) {
	req := httptest.NewRequest("GET", "/?amount=1&from=US&to=INR", nil)
	res := httptest.NewRecorder()
	Handler{Provider: &stubProvider{rate: 1}}.Convert(res, req)
	if res.Code != 400 {
		t.Fatalf("status=%d want 400", res.Code)
	}
}
