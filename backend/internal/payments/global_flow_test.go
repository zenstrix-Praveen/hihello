package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGlobalCheckoutCurrencyAndVerificationMatrix(t *testing.T) {
	for _, tc := range []struct {
		code, country, amount string
		minor                 int64
	}{
		{"INR", "IN", "10.25", 1025}, {"USD", "US", "10.25", 1025}, {"EUR", "DE", "10.25", 1025}, {"AED", "AE", "10.25", 1025}, {"SAR", "SA", "10.25", 1025}, {"QAR", "QA", "10.25", 1025}, {"KWD", "KW", "1.230", 1230}, {"BHD", "BH", "1.230", 1230}, {"OMR", "OM", "1.230", 1230}, {"JPY", "JP", "123", 123}} {
		t.Run(tc.code, func(t *testing.T) {
			h, _ := flowHandler(t)
			t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", tc.code)
			config := fmt.Sprintf(`{"providers":["razorpay"],"prices":[{"currency":"%s","country":"","billing_period":"monthly","amount":"%s","tax_bps":0}]}`, tc.code, tc.amount)
			tx, err := h.DB.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Exec(context.Background(), `UPDATE website_pricing_plans SET billing_config=$1::jsonb WHERE id=1`, config)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			posts := 0
			h.ProviderHTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				var body string
				if r.Method == "POST" {
					posts++
					var order struct {
						Amount   int64
						Currency string
					}
					json.NewDecoder(r.Body).Decode(&order)
					if order.Amount != tc.minor || order.Currency != tc.code {
						t.Fatalf("Provider quote differs: %+v", order)
					}
					body = fmt.Sprintf(`{"id":"order_global","amount":%d,"currency":"%s","status":"created"}`, order.Amount, order.Currency)
				} else {
					body = fmt.Sprintf(`{"id":"pay_global","order_id":"order_global","amount":%d,"currency":"%s","status":"captured","method":"card"}`, tc.minor, tc.code)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			req := fmt.Sprintf(`{"plan_id":1,"currency":"%s","country":"%s","billing_period":"monthly","expected_amount":"%s"}`, tc.code, tc.country, tc.amount)
			w := callHandler(h.CreateOrder, req, 1, nil)
			if w.Code != 201 {
				t.Fatalf("Checkout %d %s", w.Code, w.Body)
			}
			if scalar(t, h, "SELECT COUNT(*) FROM subscriptions") != 0 {
				t.Fatal("Activated before payment")
			}
			verify := fmt.Sprintf(`{"razorpay_payment_id":"pay_global","razorpay_order_id":"order_global","razorpay_signature":%q}`, signature("order_global|pay_global", "fixture-secret"))
			for i := 0; i < 2; i++ {
				w = callHandler(h.VerifyCheckout, verify, 1, nil)
				if w.Code != 200 {
					t.Fatalf("Verification %d %s", w.Code, w.Body)
				}
			}
			var count int
			err = h.DB.QueryRow(context.Background(), `SELECT COUNT(*) FROM subscriptions WHERE status='active' AND currency=$1 AND amount=$2::numeric`, tc.code, tc.amount).Scan(&count)
			if err != nil || count != 1 || posts != 1 {
				t.Fatalf("Activation snapshot/duplicate %d %v posts=%d", count, err, posts)
			}
		})
	}
}
func TestQuoteChangeConsentAndCountryTax(t *testing.T) {
	h, calls := flowHandler(t)
	err := executeFixtureStatement(t, h, `UPDATE website_pricing_plans SET billing_config='{"providers":["razorpay"],"prices":[{"currency":"INR","country":"IN","billing_period":"monthly","amount":"100","tax_bps":1800}]}' WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	w := callHandler(h.CreateOrder, `{"plan_id":1,"currency":"INR","country":"IN","expected_amount":"100"}`, 1, nil)
	if w.Code != 409 || *calls != 0 {
		t.Fatalf("Stale quote charged: %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"total":"118.00"`) {
		t.Fatal("Changed tax total not shown")
	}
}
func TestCurrencyCountryPeriodChangesReplaceOnlyUnpaidOrders(t *testing.T) {
	h, _ := flowHandler(t)
	t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", "INR,USD")
	err := executeFixtureStatement(t, h, `UPDATE website_pricing_plans SET billing_config='{"providers":["razorpay"],"prices":[{"currency":"USD","country":"","billing_period":"monthly","amount":"10"},{"currency":"USD","country":"","billing_period":"yearly","amount":"100"}]}' WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	posts := 0
	h.ProviderHTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Method == "POST" {
			posts++
			var order struct {
				Amount   int64
				Currency string
			}
			json.NewDecoder(r.Body).Decode(&order)
			body = fmt.Sprintf(`{"id":"order_switch_%d","amount":%d,"currency":"%s","status":"created"}`, posts, order.Amount, order.Currency)
		} else {
			body = fmt.Sprintf(`{"id":"%s","amount":10000,"currency":"INR","status":"created"}`, strings.TrimPrefix(r.URL.Path, "/v1/orders/"))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	for _, body := range []string{`{"plan_id":1,"currency":"INR"}`, `{"plan_id":1,"currency":"USD","country":"US"}`, `{"plan_id":1,"currency":"USD","country":"US","billing_period":"yearly"}`, `{"plan_id":1,"currency":"USD","country":"CA","billing_period":"yearly"}`} {
		w := callHandler(h.CreateOrder, body, 1, nil)
		if w.Code != 201 {
			t.Fatalf("Replacement %d %s", w.Code, w.Body)
		}
	}
	if scalar(t, h, "SELECT COUNT(*) FROM payments WHERE status='cancelled'") != 3 || scalar(t, h, "SELECT COUNT(*) FROM payment_transactions WHERE event_type='checkout.invalidated'") != 3 {
		t.Fatal("Unpaid checkout history lost")
	}
}
func TestStalePaidProviderOrderBlocksReplacement(t *testing.T) {
	h, calls := flowHandler(t)
	executeFixtureStatement(t, h, `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,status) VALUES(1,1,'razorpay','order_old_paid',100,'SAR','pending')`)
	h.ProviderHTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"order_old_paid","amount":10000,"currency":"SAR","status":"paid"}`)), Header: make(http.Header)}, nil
	})}
	w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 409 || *calls != 0 || scalar(t, h, "SELECT COUNT(*) FROM payments WHERE status='cancelled'") != 0 {
		t.Fatal("Successful provider order was invalidated")
	}
}

func TestAdminPriceConfigurationPersistsWithoutChangingBasePlan(t *testing.T) {
	h, _ := flowHandler(t)
	send := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PUT", "/admin/plans/1/billing-prices", strings.NewReader(body))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		h.PlanPrices(w, r)
		return w
	}
	w := send(`{"providers":["razorpay"],"prices":[{"currency":"USD","country":"US","billing_period":"yearly","amount":"99.00","tax_bps":750}]}`)
	if w.Code != 200 {
		t.Fatalf("Save prices %d %s", w.Code, w.Body)
	}
	var raw []byte
	if err := h.DB.QueryRow(context.Background(), `SELECT billing_config FROM website_pricing_plans WHERE id=1`).Scan(&raw); err != nil || !strings.Contains(string(raw), "99.00") {
		t.Fatalf("Persistence %s %v", raw, err)
	}
	if scalar(t, h, "SELECT COUNT(*) FROM website_pricing_plans WHERE id=1 AND price=100 AND currency='INR'") != 1 {
		t.Fatal("Base price overwritten")
	}
	w = send(`{"providers":["razorpay"],"prices":[{"currency":"JPY","country":"JP","billing_period":"yearly","amount":"99.01","tax_bps":0}]}`)
	if w.Code != 400 {
		t.Fatal("Fractional JPY saved")
	}
}
func TestConfiguredMinimumRejectsBeforeProviderOrder(t *testing.T) {
	h, calls := flowHandler(t)
	body := `{"disabled_currencies":[],"merchant_confirmed":true,"capabilities":{"razorpay":{"countries":["IN"],"currencies":["INR"],"minimum_minor":{"INR":20000},"methods":[{"id":"card","countries":["IN"],"currencies":["INR"]}],"renewal":"manual"}}}`
	w := callHandler(h.AdminGlobalBilling, body, 1, nil)
	if w.Code != 200 {
		t.Fatalf("Capability save %d %s", w.Code, w.Body)
	}
	w = callHandler(h.CreateOrder, `{"plan_id":1,"currency":"INR","country":"IN","expected_amount":"100"}`, 1, nil)
	if w.Code != 400 || *calls != 0 || !strings.Contains(w.Body.String(), "minimum") {
		t.Fatalf("Minimum not enforced %d %s", w.Code, w.Body)
	}
}
func TestActivePlanCanChangePeriodAfterVerifiedPayment(t *testing.T) {
	h, _ := flowHandler(t)
	for _, sql := range []string{
		`INSERT INTO subscriptions(user_id,plan_id,status,started_at,amount,currency,billing_period) VALUES(1,1,'active',NOW(),100,'INR','monthly')`,
		`UPDATE website_pricing_plans SET billing_config='{"providers":["razorpay"],"prices":[{"currency":"INR","country":"","billing_period":"yearly","amount":"100","tax_bps":0}]}' WHERE id=1`,
	} {
		if err := executeFixtureStatement(t, h, sql); err != nil {
			t.Fatal(err)
		}
	}
	w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"yearly","currency":"INR"}`, 1, nil)
	if w.Code != 201 {
		t.Fatalf("Period switch %d %s", w.Code, w.Body)
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='active' AND billing_period='monthly'") != 1 {
		t.Fatal("Subscription changed before payment")
	}
	w = callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	if w.Code != 200 || scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='active' AND billing_period='yearly'") != 1 {
		t.Fatalf("Period activation %d %s", w.Code, w.Body)
	}
}
