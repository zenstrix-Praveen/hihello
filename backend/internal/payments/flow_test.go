package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func signature(body, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}

func TestCheckoutEligibility(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		period string
		price  float64
		want   bool
	}{
		{"professional paid", "fixed", "monthly", 6, true},
		{"business fixed price", "fixed", "monthly", 5, true},
		{"personal free", "free", "monthly", 0, false},
		{"enterprise custom", "custom", "custom", 0, false},
		{"zero fixed amount", "fixed", "monthly", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkoutEligible(tc.kind, tc.period, tc.price); got != tc.want {
				t.Fatalf("checkoutEligible(%q, %q, %v) = %v, want %v", tc.kind, tc.period, tc.price, got, tc.want)
			}
		})
	}
}

func testConfig(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "local")
	t.Setenv("PAYMENT_PROVIDER", "razorpay")
	t.Setenv("RAZORPAY_MODE", "test")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_test_fixture")
	t.Setenv("RAZORPAY_KEY_SECRET", "fixture-secret")
	t.Setenv("RAZORPAY_WEBHOOK_SECRET", "fixture-webhook")
}

// All writes use connection-local TEMP tables cloned from the inspected schema.
// No public rows or sequences are changed. The provider transport never reaches the network.
func flowHandler(t *testing.T) (*Handler, *int) {
	t.Helper()
	testConfig(t)
	dsn := os.Getenv("PAYMENT_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PAYMENT_TEST_DATABASE_URL to run isolated PostgreSQL payment integration tests")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid test database configuration")
	}
	cfg.MaxConns = 1
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal("test database unavailable")
	}
	t.Cleanup(db.Close)
	if _, err := db.Exec(context.Background(), `CREATE TEMP TABLE platform_settings (setting_key text PRIMARY KEY, setting_value text, updated_at timestamptz DEFAULT NOW())`); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"users", "website_pricing_plans", "subscriptions", "payments", "payment_transactions", "processed_payment_webhook_events"} {
		_, err = db.Exec(context.Background(), fmt.Sprintf("CREATE TEMP TABLE %s (LIKE public.%s INCLUDING DEFAULTS INCLUDING CONSTRAINTS INCLUDING INDEXES)", table, table))
		if err != nil {
			t.Fatal(err)
		}
		if table != "processed_payment_webhook_events" {
			_, err = db.Exec(context.Background(), fmt.Sprintf("CREATE TEMP SEQUENCE fixture_%s; ALTER TABLE %s ALTER COLUMN id SET DEFAULT nextval('pg_temp.fixture_%s')", table, table, table))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE payments ADD COLUMN IF NOT EXISTS billing_period varchar(20)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), `ALTER TABLE subscriptions ADD COLUMN IF NOT EXISTS is_trial boolean NOT NULL DEFAULT false`); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(context.Background(), `INSERT INTO users(id,name,email,status) VALUES(1,'Fixture','fixture@example.invalid','active'),(2,'Other','other@example.invalid','active'); INSERT INTO website_pricing_plans(id,code,name,price,currency,billing_period,is_enabled) VALUES(1,'fixture-one','Monthly',100,'INR','monthly',true),(2,'fixture-two','Disabled',1,'USD','yearly',false),(3,'fixture-three','Lifetime',1,'USD','lifetime',true)`)
	if err != nil {
		t.Fatal(err)
	}
	migration, readErr := os.ReadFile("../../migrations/000033_global_subscription_prices.sql")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if _, err = db.Exec(context.Background(), string(migration)); err != nil {
		t.Fatal(err)
	}
	calls := 0
	h := &Handler{DB: db, ProviderHTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Method == "POST" {
			calls++
			var order struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			}
			if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
				t.Fatal(err)
			}
			if order.Amount != 10000 || order.Currency != "INR" {
				t.Errorf("untrusted amount sent: %+v", order)
			}
			body = fmt.Sprintf(`{"id":"order_fixture_%d","amount":10000,"currency":"INR","status":"created"}`, calls)
		} else if strings.Contains(r.URL.Path, "/orders/") {
			body = fmt.Sprintf(`{"id":"%s","amount":10000,"currency":"INR","status":"created"}`, strings.TrimPrefix(r.URL.Path, "/v1/orders/"))
		} else {
			body = `{"id":"pay_fixture","order_id":"order_fixture_1","amount":10000,"currency":"INR","status":"captured","method":"upi"}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	return h, &calls
}

func callHandler(f http.HandlerFunc, body string, user int64, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), middleware.UserIDKey, user))
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	f(w, r)
	return w
}
func verifyBody() string {
	return fmt.Sprintf(`{"razorpay_payment_id":"pay_fixture","razorpay_order_id":"order_fixture_1","razorpay_signature":%q}`, signature("order_fixture_1|pay_fixture", "fixture-secret"))
}
func scalar(t *testing.T, h *Handler, query string) int {
	t.Helper()
	var n int
	if err := h.DB.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func paymentDetailsRequest(h *Handler, ref string, userID int64) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/billing/history/"+ref, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", ref)
	ctx := context.WithValue(r.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, middleware.UserIDKey, userID)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	h.UserPaymentDetails(w, r)
	return w
}

func adminPaymentDeleteRequest(h *Handler, id int64) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/admin/payments/%d", id), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprint(id))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.DeleteAdminPayment(w, r)
	return w
}

func adminPaymentCancelRequest(h *Handler, id int64) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/admin/payments/%d/status", id), strings.NewReader(`{"status":"cancelled"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprint(id))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.CancelAdminPayment(w, r)
	return w
}

func TestSelectedBillingPeriodIsValidated(t *testing.T) {
	h, calls := flowHandler(t)
	w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"monthly"}`, 1, nil)
	if w.Code != 201 {
		t.Fatalf("valid selected period: %d %s", w.Code, w.Body)
	}
	w = callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"yearly"}`, 1, nil)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "billing period") || *calls != 1 {
		t.Fatalf("mismatched period reached checkout: %d %s", w.Code, w.Body)
	}
}

func TestFreeForeverIgnoresPaidTrialSettings(t *testing.T) {
	h, _ := flowHandler(t)
	rows, err := h.DB.Query(context.Background(), `UPDATE website_pricing_plans SET price=0,pricing_type='free',billing_period='lifetime',trial_mode='limited',trial_days=7 WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	w := callHandler(h.ActivateFreePlan, `{"plan_id":1}`, 1, nil)
	if w.Code != 201 || !strings.Contains(w.Body.String(), `"status":"active"`) {
		t.Fatalf("free forever must be active: %d %s", w.Code, w.Body)
	}
}

func TestTrialCannotBeRepeatedAfterCancellation(t *testing.T) {
	h, _ := flowHandler(t)
	rows, err := h.DB.Query(context.Background(), `UPDATE website_pricing_plans SET trial_mode='limited',trial_days=20 WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if w := callHandler(h.ActivateFreePlan, `{"plan_id":1}`, 1, nil); w.Code != 201 {
		t.Fatalf("first trial: %d %s", w.Code, w.Body)
	}
	rows, err = h.DB.Query(context.Background(), `UPDATE subscriptions SET status='cancelled'`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if w := callHandler(h.ActivateFreePlan, `{"plan_id":1}`, 1, nil); w.Code != 409 {
		t.Fatalf("trial repeated: %d %s", w.Code, w.Body)
	}
}
func TestTrialCheckoutRequiresImmediatePaymentConsent(t *testing.T) {
	h, calls := flowHandler(t)
	rows, err := h.DB.Query(context.Background(), `UPDATE website_pricing_plans SET trial_mode='limited',trial_days=20 WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil); w.Code != 409 || *calls != 0 {
		t.Fatalf("charged before agreement: %d %s", w.Code, w.Body)
	}
	if w := callHandler(h.CreateOrder, `{"plan_id":1,"pay_now":true}`, 1, nil); w.Code != 201 {
		t.Fatalf("agreed checkout: %d %s", w.Code, w.Body)
	}
}

func TestCapturedUpgradeReplacesTrial(t *testing.T) {
	h, _ := flowHandler(t)
	rows, err := h.DB.Query(context.Background(), `INSERT INTO subscriptions(user_id,plan_id,status,started_at,amount,currency,billing_period) VALUES(1,3,'trial',NOW(),0,'INR','lifetime')`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil); w.Code != 201 {
		t.Fatalf("order: %d %s", w.Code, w.Body)
	}
	if w := callHandler(h.VerifyCheckout, verifyBody(), 1, nil); w.Code != 200 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='trial'") != 0 {
		t.Fatal("paid upgrade left old trial current")
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='active' AND plan_id=1 AND amount=100 AND currency='INR'") != 1 {
		t.Fatal("purchased plan, price or currency incorrect")
	}
}

func TestOrderTrustAndSamePlan(t *testing.T) {
	h, calls := flowHandler(t)
	for _, body := range []string{`{"plan_id":2}`, `{"plan_id":1,"amount":1}`, `{"plan_id":1,"user_id":2}`, `{"plan_id":1,"currency":"USD"}`} {
		w := callHandler(h.CreateOrder, body, 1, nil)
		if w.Code != 400 {
			t.Fatalf("request should fail: %d %s", w.Code, w.Body)
		}
	}
	if *calls != 0 {
		t.Fatal("invalid request created provider order")
	}
	w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), "fixture-secret") || strings.Contains(w.Body.String(), "fixture-webhook") {
		t.Fatal("secret exposed")
	}
	w = callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 200 || *calls != 1 {
		t.Fatal("pending checkout was not reused")
	}
	w = callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	if w.Code != 200 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	w = callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "You already have this plan") || *calls != 1 {
		t.Fatalf("same plan not blocked: %d %s", w.Code, w.Body)
	}
	if scalar(t, h, "SELECT COUNT(*) FROM payments") != 1 {
		t.Fatal("duplicate payment")
	}
}

func TestCapturedPaymentKeepsPurchasedBillingPeriod(t *testing.T) {
	h, _ := flowHandler(t)
	if w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"monthly"}`, 1, nil); w.Code != 201 {
		t.Fatalf("order: %d %s", w.Code, w.Body)
	}
	rows, err := h.DB.Query(context.Background(), `UPDATE website_pricing_plans SET billing_period='yearly' WHERE id=1`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if w := callHandler(h.VerifyCheckout, verifyBody(), 1, nil); w.Code != 200 {
		t.Fatalf("verify: %d %s", w.Code, w.Body)
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='active' AND billing_period='monthly'") != 1 {
		t.Fatal("activation used an edited plan period instead of the purchased period")
	}
}

func TestCancelledOrderCannotActivateOnVerification(t *testing.T) {
	h, _ := flowHandler(t)
	if w := callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil); w.Code != 201 {
		t.Fatalf("order: %d", w.Code)
	}
	rows, err := h.DB.Query(context.Background(), `UPDATE payments SET status='cancelled' WHERE provider_order_id='order_fixture_1'`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	w := callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	if w.Code != 409 || scalar(t, h, "SELECT COUNT(*) FROM subscriptions") != 0 {
		t.Fatalf("cancelled order activated: %d %s", w.Code, w.Body)
	}
}

func TestCancelledOrderRecordsDelayedCaptureWithoutActivation(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	rows, err := h.DB.Query(context.Background(), `UPDATE payments SET status='cancelled'`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, status := range []string{"authorized", "failed", "captured"} {
		body := fmt.Sprintf(`{"event":"payment.%s","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture_1","amount":10000,"currency":"INR","status":"%s","method":"card"}}}}`, status, status)
		w := callHandler(h.Webhook, body, 0, map[string]string{"X-Razorpay-Signature": signature(body, "fixture-webhook"), "X-Razorpay-Event-Id": "evt_cancel_" + status})
		if w.Code != 200 {
			t.Fatalf("webhook: %d %s", w.Code, w.Body)
		}
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE status='paid' AND subscription_id IS NULL AND error_code='RECONCILIATION_REQUIRED'`) != 1 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions`) != 0 {
		t.Fatal("delayed capture was not retained for reconciliation")
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payment_transactions`) != 3 {
		t.Fatal("cancelled events were not audited")
	}
}

func TestCancellationDuringProviderVerificationDoesNotReportPaid(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	original := h.ProviderHTTP.Transport
	h.ProviderHTTP.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		rows, err := h.DB.Query(context.Background(), `UPDATE payments SET status='cancelled'`)
		if err != nil {
			return nil, err
		}
		rows.Close()
		return original.RoundTrip(r)
	})
	w := callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	if w.Code != 409 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions`) != 0 {
		t.Fatalf("false activation response: %d %s", w.Code, w.Body)
	}
}

func TestVerificationOwnershipSignatureAndIdempotency(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	if w := callHandler(h.VerifyCheckout, verifyBody(), 2, nil); w.Code != 404 {
		t.Fatal("cross-user verification accepted")
	}
	bad := strings.Replace(verifyBody(), signature("order_fixture_1|pay_fixture", "fixture-secret"), "invalid", 1)
	if w := callHandler(h.VerifyCheckout, bad, 1, nil); w.Code != 400 {
		t.Fatal("invalid signature accepted")
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions") != 0 {
		t.Fatal("invalid verification activated subscription")
	}
	for i := 0; i < 2; i++ {
		if w := callHandler(h.VerifyCheckout, verifyBody(), 1, nil); w.Code != 200 {
			t.Fatalf("verify failed %d %s", w.Code, w.Body)
		}
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions WHERE status='active' AND ends_at>started_at AND renews_at=ends_at") != 1 || scalar(t, h, "SELECT COUNT(*) FROM payment_transactions") != 1 || scalar(t, h, "SELECT COUNT(*) FROM payments WHERE status='paid' AND provider_payment_id='pay_fixture' AND paid_at IS NOT NULL AND payment_method='upi'") != 1 {
		t.Fatal("activation or idempotency failed")
	}
	w := callHandler(h.UserBilling, "", 2, nil)
	if strings.Contains(w.Body.String(), "Monthly") {
		t.Fatal("history leaks other user's payment")
	}
	w = callHandler(h.UserBilling, "", 1, nil)
	if !strings.Contains(w.Body.String(), `"status":"paid"`) || !strings.Contains(w.Body.String(), `"plan_name":"Monthly"`) {
		t.Fatal("history missing payment")
	}
}

func TestCustomerPaymentDetailsEnforcesOwnership(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	var ref string
	if err := h.DB.QueryRow(context.Background(), `SELECT public_id::text FROM payments WHERE user_id=1`).Scan(&ref); err != nil {
		t.Fatal(err)
	}
	if w := paymentDetailsRequest(h, ref, 1); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "provider_order_id") {
		t.Fatalf("owner could not read payment detail: %d %s", w.Code, w.Body)
	}
	if w := paymentDetailsRequest(h, ref, 2); w.Code != http.StatusNotFound {
		t.Fatalf("cross-customer payment detail was not hidden: %d %s", w.Code, w.Body)
	}
}

func TestAdminDeleteAllowsFailedButRetainsPaid(t *testing.T) {
	h, _ := flowHandler(t)
	var failedID, paidID int64
	if err := h.DB.QueryRow(context.Background(), `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,status) VALUES(1,1,'razorpay','delete_failed_order',100,'INR','failed') RETURNING id`).Scan(&failedID); err != nil {
		t.Fatal(err)
	}
	if err := h.DB.QueryRow(context.Background(), `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,status) VALUES(1,1,'razorpay','retain_paid_order',100,'INR','paid') RETURNING id`).Scan(&paidID); err != nil {
		t.Fatal(err)
	}
	if w := adminPaymentDeleteRequest(h, failedID); w.Code != http.StatusOK {
		t.Fatalf("failed payment delete failed: %d %s", w.Code, w.Body)
	}
	if w := adminPaymentDeleteRequest(h, paidID); w.Code != http.StatusConflict {
		t.Fatalf("paid payment delete was not rejected: %d %s", w.Code, w.Body)
	} else if !strings.Contains(w.Body.String(), "Completed payments cannot be deleted") {
		t.Fatalf("paid delete error was unclear: %s", w.Body)
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE id=`+fmt.Sprint(paidID)) != 1 {
		t.Fatal("paid payment was deleted")
	}
}

func TestAdminCancelOpenPaymentDoesNotActivateSubscription(t *testing.T) {
	h, _ := flowHandler(t)
	var paymentID int64
	if err := h.DB.QueryRow(context.Background(), `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,status) VALUES(1,1,'razorpay','cancel_open_order',100,'INR','pending') RETURNING id`).Scan(&paymentID); err != nil {
		t.Fatal(err)
	}
	if w := adminPaymentCancelRequest(h, paymentID); w.Code != http.StatusOK {
		t.Fatalf("open payment cancellation failed: %d %s", w.Code, w.Body)
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE id=`+fmt.Sprint(paymentID)+` AND status='cancelled'`) != 1 || scalar(t, h, `SELECT COUNT(*) FROM payment_transactions WHERE payment_id=`+fmt.Sprint(paymentID)+` AND event_type='admin.cancelled'`) != 1 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions`) != 0 {
		t.Fatal("cancellation status/audit or subscription behavior incorrect")
	}
}

func TestWebhookCaptureIdempotencyAndLateFailure(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	body := `{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture_1","amount":10000,"currency":"INR","status":"captured","method":"card"}}}}`
	headers := map[string]string{"X-Razorpay-Signature": signature(body, "fixture-webhook"), "X-Razorpay-Event-Id": "evt_capture"}
	for i := 0; i < 2; i++ {
		w := callHandler(h.Webhook, body, 0, headers)
		if w.Code != 200 {
			t.Fatalf("webhook failed %d %s", w.Code, w.Body)
		}
		if i == 1 && !strings.Contains(w.Body.String(), "duplicate_ignored") {
			t.Fatal("duplicate not ignored")
		}
	}
	if scalar(t, h, "SELECT COUNT(*) FROM subscriptions") != 1 || scalar(t, h, "SELECT COUNT(*) FROM processed_payment_webhook_events") != 1 {
		t.Fatal("duplicate webhook effect")
	}
	callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	failed := strings.ReplaceAll(strings.Replace(body, "payment.captured", "payment.failed", 1), `"status":"captured"`, `"status":"failed"`)
	headers["X-Razorpay-Signature"] = signature(failed, "fixture-webhook")
	headers["X-Razorpay-Event-Id"] = "evt_failure"
	if w := callHandler(h.Webhook, failed, 0, headers); w.Code != 200 {
		t.Fatal("late failure processing failed")
	}
	if scalar(t, h, "SELECT COUNT(*) FROM payments WHERE status='paid'") != 1 || scalar(t, h, "SELECT COUNT(*) FROM subscriptions") != 1 {
		t.Fatal("late event reversed payment or activated twice")
	}
}

func TestInvalidWebhookSignatureWithoutDatabase(t *testing.T) {
	testConfig(t)
	h := &Handler{}
	w := callHandler(h.Webhook, `{"event":"payment.captured"}`, 0, map[string]string{"X-Razorpay-Signature": "bad"})
	if w.Code != 400 {
		t.Fatal("invalid webhook signature accepted")
	}
}

func TestMissingConfigWithoutDatabase(t *testing.T) {
	testConfig(t)
	t.Setenv("RAZORPAY_KEY_SECRET", "")
	w := callHandler((&Handler{}).CreateOrder, `{"plan_id":1}`, 1, nil)
	if w.Code != 503 {
		t.Fatal("missing configuration not rejected safely")
	}
}

func TestRefundUsesTrustedCurrencyAndCaptureCannotReactivate(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	callHandler(h.VerifyCheckout, verifyBody(), 1, nil)
	// Razorpay refund entities need not include currency: use the local payment.
	body := `{"event":"refund.processed","payload":{"refund":{"entity":{"id":"rfnd_fixture","payment_id":"pay_fixture","amount":10000,"status":"processed"}}}}`
	headers := map[string]string{"X-Razorpay-Signature": signature(body, "fixture-webhook"), "X-Razorpay-Event-Id": "evt_refund"}
	if w := callHandler(h.Webhook, body, 0, headers); w.Code != 200 {
		t.Fatalf("refund failed: %d %s", w.Code, w.Body)
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payment_transactions WHERE event_type='refund.processed' AND currency='INR' AND amount=100`) != 1 {
		t.Fatal("refund currency or amount incorrect")
	}
	capture := `{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture_1","amount":10000,"currency":"INR","status":"captured"}}}}`
	headers["X-Razorpay-Signature"] = signature(capture, "fixture-webhook")
	headers["X-Razorpay-Event-Id"] = "evt_late_capture"
	callHandler(h.Webhook, capture, 0, headers)
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE status='refunded'`) != 1 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions`) != 1 {
		t.Fatal("late capture reactivated refunded payment")
	}
}

func TestFailedWebhookDoesNotActivate(t *testing.T) {
	h, _ := flowHandler(t)
	callHandler(h.CreateOrder, `{"plan_id":1}`, 1, nil)
	body := `{"event":"payment.failed","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture_1","amount":10000,"currency":"INR","status":"failed"}}}}`
	w := callHandler(h.Webhook, body, 0, map[string]string{"X-Razorpay-Signature": signature(body, "fixture-webhook"), "X-Razorpay-Event-Id": "evt_failed"})
	if w.Code != 200 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions`) != 0 || scalar(t, h, `SELECT COUNT(*) FROM payments WHERE status='failed'`) != 1 {
		t.Fatal("failed payment handling incorrect")
	}
}

func TestLiveModeRequiresExplicitProductionConfig(t *testing.T) {
	testConfig(t)
	t.Setenv("RAZORPAY_MODE", "live")
	t.Setenv("RAZORPAY_KEY_ID", "rzp_live_fixture")
	if ValidateConfig() == nil {
		t.Fatal("local live mode accepted")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("FRONTEND_URL", "http://example.invalid")
	if ValidateConfig() == nil {
		t.Fatal("insecure live mode accepted")
	}
	t.Setenv("FRONTEND_URL", "https://example.invalid")
	if err := ValidateConfig(); err != nil {
		t.Fatalf("explicit production live config rejected: %v", err)
	}
}

func TestStaleSARCheckoutIsInvalidatedAndINRMinimumBlocksNewOrder(t *testing.T) {
	h, calls := flowHandler(t)
	if err := executeFixtureStatement(t, h, `UPDATE website_pricing_plans SET price=0.50,currency='INR' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	if err := executeFixtureStatement(t, h, `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,billing_period,status) VALUES(1,1,'razorpay','order_stale_sar',1,'SAR','monthly','pending')`); err != nil {
		t.Fatal(err)
	}
	w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"monthly"}`, 1, nil)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "INR payment of at least") {
		t.Fatalf("expected clear INR minimum error, got %d %s", w.Code, w.Body)
	}
	if *calls != 0 {
		t.Fatal("provider called for a subminimum INR price")
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE provider_order_id='order_stale_sar' AND status='cancelled'`) != 1 {
		t.Fatal("incompatible SAR checkout was not invalidated")
	}
}

func TestStaleCurrencyCheckoutCreatesFreshAuthoritativeOrder(t *testing.T) {
	h, calls := flowHandler(t)
	if err := executeFixtureStatement(t, h, `INSERT INTO payments(user_id,plan_id,provider,provider_order_id,amount,currency,billing_period,status) VALUES(1,1,'razorpay','order_stale_sar',100,'SAR','monthly','pending')`); err != nil {
		t.Fatal(err)
	}
	w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"monthly"}`, 1, nil)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"razorpay_order_id":"order_fixture_1"`) || !strings.Contains(w.Body.String(), `"amount":10000`) || !strings.Contains(w.Body.String(), `"currency":"INR"`) {
		t.Fatalf("expected a new INR 100.00 order from PostgreSQL plan data, got %d %s", w.Code, w.Body)
	}
	if *calls != 1 || scalar(t, h, `SELECT COUNT(*) FROM payments WHERE provider_order_id='order_stale_sar' AND status='cancelled'`) != 1 {
		t.Fatal("stale order was reused or duplicate order was created")
	}
	if scalar(t, h, `SELECT COUNT(*) FROM payments WHERE provider_order_id='order_fixture_1' AND status='created' AND amount=100 AND currency='INR'`) != 1 {
		t.Fatal("fresh authoritative order was not saved")
	}
}
func TestSARPlanUsesAuthoritativeCurrencyAndActivatesOnlyAfterVerification(t *testing.T) {
	h, calls := flowHandler(t)
	t.Setenv("RAZORPAY_SUPPORTED_CURRENCIES", "SAR")
	if err := executeFixtureStatement(t, h, `UPDATE website_pricing_plans SET price=100,currency='SAR',trial_mode='disabled',billing_period='monthly' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	h.ProviderHTTP = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		var body string
		if r.Method == http.MethodPost {
			*calls = *calls + 1
			var order struct {
				Amount   int64  `json:"amount"`
				Currency string `json:"currency"`
			}
			if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
				t.Fatal(err)
			}
			if order.Amount != 10000 || order.Currency != "SAR" {
				t.Errorf("unexpected SAR provider order: %+v", order)
			}
			body = fmt.Sprintf(`{"id":"order_sar_fixture","amount":%d,"currency":"%s","status":"created"}`, order.Amount, order.Currency)
		} else {
			body = `{"id":"pay_sar_fixture","order_id":"order_sar_fixture","amount":10000,"currency":"SAR","status":"captured","method":"card"}`
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	w := callHandler(h.CreateOrder, `{"plan_id":1,"billing_period":"monthly"}`, 1, nil)
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"razorpay_order_id":"order_sar_fixture"`) || !strings.Contains(w.Body.String(), `"currency":"SAR"`) || !strings.Contains(w.Body.String(), `"amount_major":"100.00"`) {
		t.Fatalf("SAR plan order did not use database quote: %d %s", w.Code, w.Body)
	}
	body := fmt.Sprintf(`{"razorpay_payment_id":"pay_sar_fixture","razorpay_order_id":"order_sar_fixture","razorpay_signature":%q}`, signature("order_sar_fixture|pay_sar_fixture", "fixture-secret"))
	w = callHandler(h.VerifyCheckout, body, 1, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"paid"`) {
		t.Fatalf("SAR verification: %d %s", w.Code, w.Body)
	}
	if *calls != 1 || scalar(t, h, `SELECT COUNT(*) FROM subscriptions WHERE status='active' AND plan_id=1 AND amount=100 AND currency='SAR'`) != 1 {
		t.Fatal("SAR subscription was not activated from the verified payment")
	}
}
func executeFixtureStatement(t *testing.T, h *Handler, statement string) error {
	t.Helper()
	rows, err := h.DB.Query(context.Background(), statement)
	if err != nil {
		return err
	}
	rows.Close()
	return rows.Err()
}
