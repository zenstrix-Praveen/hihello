package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/currency"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestProviderCurrencyAvailability(t *testing.T) {
	for _, tc := range []struct {
		provider, configured string
		want                 []string
	}{
		{"razorpay", "", []string{"INR"}},
		{"razorpay", " inr, AED,jod,ZZZ,JPY,AED ", []string{"INR", "AED", "JOD", "JPY"}},
		{"razorpay", "ZZZ", []string{}},
		{"mock", "", currency.Codes()},
		{"unavailable", "", []string{}},
	} {
		t.Run(tc.provider+tc.configured, func(t *testing.T) {
			testConfig(t)
			t.Setenv("PAYMENT_PROVIDER", tc.provider)
			t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", tc.configured)
			w := httptest.NewRecorder()
			(&Handler{}).Provider(w, httptest.NewRequest("GET", "/", nil))
			var got struct {
				Supported []string `json:"supported_currencies"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Supported, tc.want) {
				t.Fatalf("supported currencies = %v; want %v; response %s", got.Supported, tc.want, w.Body.String())
			}
		})
	}
}
func TestTestCurrencyAllowlistCannotEnableLiveCurrencies(t *testing.T) {
	t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", "INR,SAR,QAR")
	t.Setenv("RAZORPAY_CURRENCY_ALLOWLIST_MODE", "test")
	t.Setenv("RAZORPAY_MODE", "live")
	if len(SupportedCurrencies("razorpay")) != 0 {
		t.Fatal("test availability leaked into live mode")
	}
}

type currencyRow struct {
	values []any
	err    error
}

func (r currencyRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("unexpected scan length %d, want %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

// Embedded interfaces make any unplanned database call fail the test immediately.
type currencyDB struct {
	Database
	tx   *currencyTx
	mock bool
}

func (d currencyDB) Begin(context.Context) (pgx.Tx, error) { return d.tx, nil }
func (d currencyDB) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	if strings.Contains(query, "platform_settings") {
		return currencyRow{err: pgx.ErrNoRows}
	}
	if strings.Contains(query, "SELECT\n\t\tEXISTS") {
		return currencyRow{values: []any{false, false}}
	}
	return d.tx.QueryRow(context.Background(), query)
}

type currencyTx struct {
	pgx.Tx
	currency, pending string
	writes            int
	committed         bool
}

func (x *currencyTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	switch {
	case strings.Contains(query, "SELECT COALESCE(jsonb_agg"):
		return currencyRow{values: []any{[]byte("[]")}}
	case strings.Contains(query, "SELECT EXISTS"):
		return currencyRow{values: []any{false}}
	case strings.Contains(query, "SELECT billing_config"):
		return currencyRow{values: []any{[]byte(`{"prices":[],"providers":["razorpay"]}`)}}
	case strings.Contains(query, "FROM website_pricing_plans"):
		values := []any{"Plan", x.currency, "10.00", "monthly", "fixed", true, true}
		if strings.Contains(query, "COALESCE(u.name") {
			values = append(values, "Customer", "customer@example.test", "")
			if strings.Contains(query, "p.trial_mode") {
				values = append(values, "none")
			}
		}
		return currencyRow{values: values}
	case strings.Contains(query, "FROM payments"):
		if x.pending == "" {
			return currencyRow{err: pgx.ErrNoRows}
		}
		if x.pending != x.currency {
			return currencyRow{err: pgx.ErrNoRows}
		}
		return currencyRow{values: []any{"payment-legacy", "order-legacy"}}
	default:
		x.writes++
		return currencyRow{values: []any{"payment-new"}}
	}
}
func (x *currencyTx) Exec(_ context.Context, query string, _ ...any) (pgconn.CommandTag, error) {
	if strings.Contains(query, "UPDATE payments SET status='cancelled'") {
		if x.pending != "" && x.pending != x.currency {
			x.writes++
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}
	if !strings.Contains(query, "pg_advisory_xact_lock") {
		x.writes++
	}
	return pgconn.NewCommandTag("INSERT 1"), nil
}
func (x *currencyTx) Rollback(context.Context) error { return nil }
func (x *currencyTx) Commit(context.Context) error   { x.committed = true; return nil }

func TestUnsupportedCheckoutCurrencyHasNoWritesOrProviderRequests(t *testing.T) {
	for _, tc := range []struct{ provider, currency, pending string }{
		{"mock", "ZZZ", ""}, {"mock", "XXX", ""}, {"razorpay", "AED", ""}, {"razorpay", "ZZZ", ""},
	} {
		t.Run(tc.provider+tc.currency+tc.pending, func(t *testing.T) {
			testConfig(t)
			t.Setenv("PAYMENT_PROVIDER", tc.provider)
			t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", "")
			x := &currencyTx{currency: tc.currency, pending: tc.pending}
			calls := 0
			h := &Handler{DB: currencyDB{tx: x}, ProviderHTTP: &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, fmt.Errorf("unexpected provider request")
			})}}
			fn := h.CreateOrder
			if tc.provider == "mock" {
				fn = h.MockOrder
			}
			w := callHandler(fn, `{"plan_id":1}`, 1, nil)
			if w.Code != 400 || !strings.Contains(strings.ToLower(w.Body.String()), "currency") {
				t.Fatalf("expected currency rejection; got %d %s", w.Code, w.Body.String())
			}
			if calls != 0 || x.writes != 0 || x.committed {
				t.Fatalf("rejected order had side effects: calls=%d writes=%d committed=%v", calls, x.writes, x.committed)
			}
		})
	}
}

func TestPendingCheckoutUsesStoredCurrency(t *testing.T) {
	testConfig(t)
	t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", "")
	x := &currencyTx{currency: "INR", pending: "INR"}
	h := &Handler{DB: currencyDB{tx: x}, ProviderHTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"order-legacy","amount":1000,"currency":"INR","status":"created"}`)), Header: make(http.Header)}, nil
	})}}
	w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 200 {
		t.Fatalf("stored INR order could not resume: %d %s", w.Code, w.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["currency"] != "INR" || out["amount"] != float64(1000) || x.writes != 0 {
		t.Fatalf("matching pending quote was not reused: %s", w.Body.String())
	}
}

func TestSelectedCurrenciesReachProviderWithoutConversion(t *testing.T) {
	for _, code := range []string{"USD", "INR", "AED", "SAR", "QAR", "KWD", "BHD", "OMR", "EGP", "JOD", "EUR", "GBP"} {
		for _, provider := range []string{"mock", "razorpay"} {
			t.Run(provider+code, func(t *testing.T) {
				testConfig(t)
				t.Setenv("PAYMENT_PROVIDER", provider)
				t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", code)
				x := &currencyTx{currency: code}
				calls := 0
				h := &Handler{DB: currencyDB{tx: x}, ProviderHTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
					calls++
					var order struct {
						Currency string `json:"currency"`
						Amount   int64  `json:"amount"`
					}
					if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
						t.Fatal(err)
					}
					want := int64(1000)
					if code == "KWD" || code == "BHD" || code == "OMR" || code == "JOD" {
						want = 10000
					}
					if order.Currency != code || order.Amount != want {
						t.Errorf("provider currency/amount changed: %+v", order)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"id":"order-new","amount":%d,"currency":%q}`, order.Amount, order.Currency))), Header: make(http.Header)}, nil
				})}}
				fn := h.CreateOrder
				if provider == "mock" {
					fn = h.MockOrder
				}
				w := callHandler(fn, `{"plan_id":1}`, 1, nil)
				if w.Code != 201 {
					t.Fatalf("currency rejected: %d %s", w.Code, w.Body.String())
				}
				if x.writes != 1 {
					t.Fatalf("expected one payment write, got %d", x.writes)
				}
				if provider == "razorpay" && calls != 1 {
					t.Fatalf("provider calls %d", calls)
				}
				var out map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &out)
				if out["currency"] != code {
					t.Fatalf("response currency changed: %s", w.Body.String())
				}
			})
		}
	}
}
