package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func validPlanInput(limits string) planInput {
	return planInput{
		Name: "Example", Code: "example", Price: 0, Currency: "USD", BillingPeriod: "monthly",
		Limits: json.RawMessage(limits),
	}
}

func TestPlanStatusRequiresExplicitBoolean(t *testing.T) {
	h := &PlansBillingHandler{}
	for _, body := range []string{`{}`, `{"active":false}`, `{"is_active":"false"}`} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPatch, "/", bytes.NewBufferString(body))
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "1")
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
			w := httptest.NewRecorder()
			h.PlanStatus(w, r)
			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode validation response: %v", err)
			}
			if w.Code != http.StatusBadRequest || response["error"] != "is_active is required" {
				t.Fatalf("expected HTTP 400 for missing/invalid is_active, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// Set WEBSITE_TEST_DATABASE_URL to run this read-only endpoint regression test
// against a migrated PostgreSQL test database.
func TestPlansEndpointsLoadFromDatabase(t *testing.T) {
	dsn := os.Getenv("WEBSITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_TEST_DATABASE_URL to run PostgreSQL endpoint regression tests")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(db.Close)

	h := &PlansBillingHandler{DB: db}
	for name, handler := range map[string]func(http.ResponseWriter, *http.Request){
		"admin plans":  h.ListPlans,
		"public plans": h.PublicPlans,
	} {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			w := httptest.NewRecorder()
			handler(w, r)
			if w.Code != http.StatusOK {
				t.Fatalf("endpoint returned %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestPlanCreateEditAndStatusPersist(t *testing.T) {
	dsn := os.Getenv("PLAN_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL to run plan CRUD tests against a disposable database")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable test database: %v", err)
	}
	t.Cleanup(db.Close)
	h := &PlansBillingHandler{DB: db}
	code := fmt.Sprintf("adm002-crud-%d", time.Now().UnixNano())
	makeBody := func(name, period string, price float64, featured, active bool, order, cardLimit int) []byte {
		body, marshalErr := json.Marshal(map[string]any{
			"name": name, "code": code, "description": "CRUD regression", "price": price,
			"pricing_type": "fixed", "currency": "USD", "billing_period": period,
			"trial_days": 7, "features": []string{"digital_cards"},
			"limits":      map[string]any{"digital_cards_limit": cardLimit, "storage_limit": 5, "storage_unlimited": true},
			"is_featured": featured, "is_active": active, "sort_order": order,
		})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return body
	}
	created := httptest.NewRecorder()
	h.CreatePlan(created, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(makeBody("ADM002 Test Plan", "monthly", 19.50, false, true, 80, 5))))
	if created.Code != http.StatusOK {
		t.Fatalf("create returned %d: %s", created.Code, created.Body.String())
	}
	var initial map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &initial); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	id := int64(initial["id"].(float64))
	t.Cleanup(func() {
		if _, cleanupErr := db.Exec(context.Background(), `DELETE FROM website_pricing_plans WHERE id=$1`, id); cleanupErr != nil {
			t.Errorf("clean up plan fixture: %v", cleanupErr)
		}
	})

	updateReq := httptest.NewRequest(http.MethodPut, "/", bytes.NewReader(makeBody("ADM002 Edited Plan", "yearly", 29.99, true, true, 99, 12)))
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("id", fmt.Sprint(id))
	updateReq = updateReq.WithContext(context.WithValue(updateReq.Context(), chi.RouteCtxKey, ctx))
	updated := httptest.NewRecorder()
	h.UpdatePlan(updated, updateReq)
	if updated.Code != http.StatusOK {
		t.Fatalf("update returned %d: %s", updated.Code, updated.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/", nil)
	getCtx := chi.NewRouteContext()
	getCtx.URLParams.Add("id", fmt.Sprint(id))
	getReq = getReq.WithContext(context.WithValue(getReq.Context(), chi.RouteCtxKey, getCtx))
	reloaded := httptest.NewRecorder()
	h.GetPlan(reloaded, getReq)
	var persisted map[string]any
	if reloaded.Code != http.StatusOK || json.Unmarshal(reloaded.Body.Bytes(), &persisted) != nil {
		t.Fatalf("reload returned %d: %s", reloaded.Code, reloaded.Body.String())
	}
	limits := persisted["limits"].(map[string]any)
	if persisted["name"] != "ADM002 Edited Plan" || persisted["price"] != 29.99 || persisted["billing_period"] != "yearly" || persisted["currency"] != "USD" || persisted["is_featured"] != true || persisted["is_active"] != true || persisted["sort_order"] != float64(99) || limits["digital_cards_limit"] != float64(12) || limits["storage_limit"] != float64(5) || limits["storage_unlimited"] != true {
		t.Fatalf("edited fields did not persist: %#v", persisted)
	}

	for _, active := range []bool{false, true} {
		body, _ := json.Marshal(map[string]bool{"is_active": active})
		r := httptest.NewRequest(http.MethodPatch, "/", bytes.NewReader(body))
		statusCtx := chi.NewRouteContext()
		statusCtx.URLParams.Add("id", fmt.Sprint(id))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, statusCtx))
		w := httptest.NewRecorder()
		h.PlanStatus(w, r)
		var statusBody map[string]any
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &statusBody) != nil || statusBody["is_active"] != active {
			t.Fatalf("status active=%v returned %d: %s", active, w.Code, w.Body.String())
		}
		var persistedActive bool
		if err := db.QueryRow(t.Context(), `SELECT is_enabled FROM website_pricing_plans WHERE id=$1`, id).Scan(&persistedActive); err != nil || persistedActive != active {
			t.Fatalf("status active=%v did not persist: got=%v err=%v", active, persistedActive, err)
		}
	}
}

func createPlanHistoryFixture(t *testing.T, db *pgxpool.Pool) (int64, int64, int64, int64, int64) {
	t.Helper()
	ctx := t.Context()
	code := fmt.Sprintf("adm002-history-%d", time.Now().UnixNano())
	var planID int64
	err := db.QueryRow(ctx, `INSERT INTO website_pricing_plans(name,code,description,price,pricing_type,currency,billing_period,trial_days,features,limits,is_featured,is_enabled,sort_order) VALUES($1,$2,'',12.50,'fixed','USD','monthly',0,'[]'::jsonb,'{}'::jsonb,false,true,0) RETURNING id`, "ADM-002 history test", code).Scan(&planID)
	if err != nil {
		t.Fatalf("create disposable plan: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM website_pricing_plans WHERE id=$1`, planID); err != nil {
			t.Errorf("clean up disposable plan %d: %v", planID, err)
		}
	})

	var subscriptionID int64
	err = db.QueryRow(ctx, `INSERT INTO subscriptions(plan_id,status) VALUES($1,'active') RETURNING id`, planID).Scan(&subscriptionID)
	if err != nil {
		t.Fatalf("create subscription reference: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM subscriptions WHERE id=$1`, subscriptionID); err != nil {
			t.Errorf("clean up subscription %d: %v", subscriptionID, err)
		}
	})

	var userID int64
	email := fmt.Sprintf("adm002-history-%d@example.invalid", time.Now().UnixNano())
	err = db.QueryRow(ctx, `INSERT INTO users(name,email) VALUES('ADM-002 history test',$1) RETURNING id`, email).Scan(&userID)
	if err != nil {
		t.Fatalf("create payment owner: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID); err != nil {
			t.Errorf("clean up test user %d: %v", userID, err)
		}
	})

	var paymentID int64
	orderID := fmt.Sprintf("adm002-order-%d", time.Now().UnixNano())
	err = db.QueryRow(ctx, `INSERT INTO payments(user_id,subscription_id,plan_id,provider_order_id,amount,currency,status) VALUES($1,$2,$3,$4,12.50,'USD','paid') RETURNING id`, userID, subscriptionID, planID, orderID).Scan(&paymentID)
	if err != nil {
		t.Fatalf("create payment history reference: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM payments WHERE id=$1`, paymentID); err != nil {
			t.Errorf("clean up payment %d: %v", paymentID, err)
		}
	})
	var transactionID int64
	err = db.QueryRow(ctx, `INSERT INTO payment_transactions(payment_id,event_type,amount,currency,payload) VALUES($1,'adm002_test_history',12.50,'USD','{}'::jsonb) RETURNING id`, paymentID).Scan(&transactionID)
	if err != nil {
		t.Fatalf("create transaction history reference: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM payment_transactions WHERE id=$1`, transactionID); err != nil {
			t.Errorf("clean up payment transaction %d: %v", transactionID, err)
		}
	})
	return planID, subscriptionID, paymentID, transactionID, userID
}

// This regression creates its own billing references and verifies that delete
// archives the plan without mutating subscriptions, payments, or transactions.
func TestDeleteReferencedPlanArchivesAndPreservesHistory(t *testing.T) {
	dsn := os.Getenv("PLAN_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL to run write endpoint tests against a disposable database")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(db.Close)
	id, subscriptionID, paymentID, transactionID, userID := createPlanHistoryFixture(t, db)
	var references int
	if err := db.QueryRow(t.Context(), `SELECT (SELECT COUNT(*) FROM subscriptions WHERE id=$1 AND plan_id=$3)+(SELECT COUNT(*) FROM payments WHERE id=$2 AND plan_id=$3)`, subscriptionID, paymentID, id).Scan(&references); err != nil || references != 2 {
		t.Fatalf("expected both disposable billing references before delete: count=%d err=%v", references, err)
	}
	r := httptest.NewRequest(http.MethodDelete, "/", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprint(id))
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	(&PlansBillingHandler{DB: db}).DeletePlan(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 when archiving referenced plan %d, got %d: %s", id, w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["message"] != "Plan archived" {
		t.Fatalf("expected structured archive success message, got %s", w.Body.String())
	}
	var stillExists, enabled bool
	var archived bool
	if err := db.QueryRow(t.Context(), `SELECT is_enabled,archived_at IS NOT NULL FROM website_pricing_plans WHERE id=$1`, id).Scan(&enabled, &archived); err != nil || enabled || !archived {
		t.Fatalf("referenced plan %d was not archived (enabled=%v archived=%v err=%v)", id, enabled, archived, err)
	}
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE id=$1 AND plan_id=$3 AND status='active') AND EXISTS(SELECT 1 FROM payments WHERE id=$2 AND plan_id=$3 AND status='paid') AND EXISTS(SELECT 1 FROM payment_transactions WHERE id=$4 AND payment_id=$2 AND event_type='adm002_test_history')`, subscriptionID, paymentID, id, transactionID).Scan(&stillExists); err != nil || !stillExists {
		t.Fatalf("archive changed historical references: preserved=%v err=%v", stillExists, err)
	}
	list := httptest.NewRecorder()
	(&PlansBillingHandler{DB: db}).ListPlans(list, httptest.NewRequest(http.MethodGet, "/", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("admin plan list returned %d: %s", list.Code, list.Body.String())
	}
	var listBody struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode admin plans: %v", err)
	}
	for _, listed := range listBody.Data {
		if listed["id"] == float64(id) {
			t.Fatalf("archived plan %d remained in normal admin list", id)
		}
	}
	public := httptest.NewRecorder()
	(&PlansBillingHandler{DB: db}).PublicPlans(public, httptest.NewRequest(http.MethodGet, "/", nil))
	if public.Code != http.StatusOK {
		t.Fatalf("public plan list returned %d: %s", public.Code, public.Body.String())
	}
	var publicBody struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(public.Body.Bytes(), &publicBody); err != nil {
		t.Fatalf("decode public plans: %v", err)
	}
	for _, listed := range publicBody.Data {
		if listed["id"] == float64(id) {
			t.Fatalf("archived plan %d remained selectable publicly", id)
		}
	}
	planRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	planContext := chi.NewRouteContext()
	planContext.URLParams.Add("id", fmt.Sprint(id))
	planRequest = planRequest.WithContext(context.WithValue(planRequest.Context(), chi.RouteCtxKey, planContext))
	historicalPlan := httptest.NewRecorder()
	(&PlansBillingHandler{DB: db}).GetPlan(historicalPlan, planRequest)
	if historicalPlan.Code != http.StatusOK {
		t.Fatalf("historical plan lookup returned %d: %s", historicalPlan.Code, historicalPlan.Body.String())
	}
	if err := json.Unmarshal(historicalPlan.Body.Bytes(), &body); err != nil || body["name"] != "ADM-002 history test" {
		t.Fatalf("historical plan details unavailable: %s (%v)", historicalPlan.Body.String(), err)
	}
	newSubscriptionBody, _ := json.Marshal(map[string]any{"plan_id": id, "user_id": userID, "status": "active"})
	newSubscription := httptest.NewRecorder()
	(&PlansBillingHandler{DB: db}).CreateSubscription(newSubscription, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(newSubscriptionBody)))
	var unavailable map[string]string
	if err := json.Unmarshal(newSubscription.Body.Bytes(), &unavailable); err != nil || newSubscription.Code != http.StatusBadRequest || unavailable["error"] != "Plan not found" {
		t.Fatalf("archived plan was selectable for a new subscription: status=%d body=%s err=%v", newSubscription.Code, newSubscription.Body.String(), err)
	}
	if err := db.QueryRow(t.Context(), `SELECT COUNT(*) FROM website_pricing_plans WHERE id=$1`, id).Scan(&references); err != nil || references != 1 {
		t.Fatalf("archived plan row missing: count=%d err=%v", references, err)
	}
}

// PLAN_WRITE_TEST_DATABASE_URL must point to a disposable migrated DB. The
// test creates and deletes its own unused plan through the HTTP handlers.
func TestDeleteUnusedPlanSucceeds(t *testing.T) {
	dsn := os.Getenv("PLAN_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL to run write endpoint tests against a disposable database")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable test database: %v", err)
	}
	t.Cleanup(db.Close)
	h := &PlansBillingHandler{DB: db}
	code := fmt.Sprintf("delete-regression-%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]any{"name": "Delete regression test", "code": code, "price": 0, "pricing_type": "free", "currency": "USD", "billing_period": "monthly", "limits": map[string]any{}})
	created := httptest.NewRecorder()
	h.CreatePlan(created, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
	if created.Code != http.StatusOK {
		t.Fatalf("create test plan returned %d: %s", created.Code, created.Body.String())
	}
	var plan map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode created test plan: %v", err)
	}
	id := int64(plan["id"].(float64))
	var hasReferences bool
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE plan_id=$1) OR EXISTS(SELECT 1 FROM payments WHERE plan_id=$1)`, id).Scan(&hasReferences); err != nil || hasReferences {
		t.Fatalf("new test plan should have no protected references: referenced=%v err=%v", hasReferences, err)
	}
	deleteRequest := func() *http.Request {
		r := httptest.NewRequest(http.MethodDelete, "/", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprint(id))
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM website_pricing_plans WHERE id=$1`, id); err != nil {
			t.Errorf("clean up unused plan %d: %v", id, err)
		}
	})
	deleted := httptest.NewRecorder()
	h.DeletePlan(deleted, deleteRequest())
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete unused plan returned %d: %s", deleted.Code, deleted.Body.String())
	}
	var deleteResponse map[string]string
	if err := json.Unmarshal(deleted.Body.Bytes(), &deleteResponse); err != nil || deleteResponse["message"] != "Plan archived" {
		t.Fatalf("unexpected successful delete response: %s (%v)", deleted.Body.String(), err)
	}
	getRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	getContext := chi.NewRouteContext()
	getContext.URLParams.Add("id", fmt.Sprint(id))
	getRequest = getRequest.WithContext(context.WithValue(getRequest.Context(), chi.RouteCtxKey, getContext))
	getAfterDelete := httptest.NewRecorder()
	h.GetPlan(getAfterDelete, getRequest)
	if getAfterDelete.Code != http.StatusOK {
		t.Fatalf("historical GET should retain archived plan, got %d: %s", getAfterDelete.Code, getAfterDelete.Body.String())
	}
	listAfterArchive := httptest.NewRecorder()
	h.ListPlans(listAfterArchive, httptest.NewRequest(http.MethodGet, "/", nil))
	var plansAfterArchive struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(listAfterArchive.Body.Bytes(), &plansAfterArchive); err != nil {
		t.Fatalf("decode plans after archive: %v", err)
	}
	for _, listed := range plansAfterArchive.Data {
		if listed["id"] == float64(id) {
			t.Fatalf("archived plan %d remained in admin list", id)
		}
	}
}

func TestDeactivateReferencedPlanPreservesHistory(t *testing.T) {
	dsn := os.Getenv("PLAN_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL to run status endpoint tests against a disposable database")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable test database: %v", err)
	}
	t.Cleanup(db.Close)
	id, subscriptionID, paymentID, transactionID, _ := createPlanHistoryFixture(t, db)
	h := &PlansBillingHandler{DB: db}
	t.Cleanup(func() {
		if _, err := db.Exec(context.Background(), `DELETE FROM admin_security_activity WHERE entity_type='plan' AND entity_id=$1 AND action='plan_status_changed'`, fmt.Sprint(id)); err != nil {
			t.Errorf("clean up plan status audit event: %v", err)
		}
	})
	statusRequest := func(enabled bool) *http.Request {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprint(id))
		r := httptest.NewRequest(http.MethodPatch, "/", bytes.NewReader([]byte(fmt.Sprintf(`{"is_active":%t}`, enabled))))
		return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	}
	w := httptest.NewRecorder()
	h.PlanStatus(w, statusRequest(false))
	if w.Code != http.StatusOK {
		t.Fatalf("deactivate referenced plan returned %d: %s", w.Code, w.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response["is_active"] != false {
		t.Fatalf("deactivate response did not report inactive plan: %s (%v)", w.Body.String(), err)
	}
	var enabled, planExists, subscriptionExists, paymentExists, transactionExists bool
	if err := db.QueryRow(t.Context(), `SELECT is_enabled FROM website_pricing_plans WHERE id=$1`, id).Scan(&enabled); err != nil || enabled {
		t.Fatalf("plan was not persisted inactive: enabled=%v err=%v", enabled, err)
	}
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM website_pricing_plans WHERE id=$1)`, id).Scan(&planExists); err != nil || !planExists {
		t.Fatalf("deactivation deleted the plan: exists=%v err=%v", planExists, err)
	}
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM subscriptions WHERE id=$1 AND plan_id=$2 AND status='active')`, subscriptionID, id).Scan(&subscriptionExists); err != nil || !subscriptionExists {
		t.Fatalf("deactivation changed subscription history: exists=%v err=%v", subscriptionExists, err)
	}
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM payments WHERE id=$1 AND plan_id=$2 AND subscription_id=$3 AND status='paid' AND amount=12.50 AND currency='USD')`, paymentID, id, subscriptionID).Scan(&paymentExists); err != nil || !paymentExists {
		t.Fatalf("deactivation changed payment history: exists=%v err=%v", paymentExists, err)
	}
	if err := db.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM payment_transactions WHERE id=$1 AND payment_id=$2 AND event_type='adm002_test_history')`, transactionID, paymentID).Scan(&transactionExists); err != nil || !transactionExists {
		t.Fatalf("deactivation changed transaction history: exists=%v err=%v", transactionExists, err)
	}
	public := httptest.NewRecorder()
	h.PublicPlans(public, httptest.NewRequest(http.MethodGet, "/", nil))
	if public.Code != http.StatusOK {
		t.Fatalf("public plans returned %d: %s", public.Code, public.Body.String())
	}
	var publicPlans struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(public.Body.Bytes(), &publicPlans); err != nil {
		t.Fatalf("decode public plans: %v", err)
	}
	for _, plan := range publicPlans.Data {
		if plan.ID == id {
			t.Fatalf("deactivated plan %d remained available for new purchases", id)
		}
	}
}

func TestNormalizePlanAcceptsExplicitUnlimitedLimits(t *testing.T) {
	p := validPlanInput(`{"digital_cards_limit":0,"digital_cards_unlimited":true,"qr_codes_limit":0,"storage_limit":25}`)
	if err := normalizePlan(&p); err != nil {
		t.Fatalf("expected explicit unlimited limit to be valid: %v", err)
	}
}

func TestNormalizePlanRejectsNegativeNumericLimit(t *testing.T) {
	p := validPlanInput(`{"digital_cards_limit":-1}`)
	if err := normalizePlan(&p); err == nil {
		t.Fatal("expected negative numeric limit to be rejected; unlimited uses its explicit flag")
	}
}

func TestNormalizePlanRejectsLimitsBelowMinusOne(t *testing.T) {
	p := validPlanInput(`{"digital_cards_limit":-2}`)
	if err := normalizePlan(&p); err == nil {
		t.Fatal("expected limits below -1 to be rejected")
	}
}

func TestNormalizePlanPricingTypes(t *testing.T) {
	t.Run("free", func(t *testing.T) {
		p := validPlanInput(`{"digital_cards_limit":4}`)
		if err := normalizePlan(&p); err != nil || p.PricingType != "free" || p.Price != 0 {
			t.Fatalf("free plan should normalize without payment: type=%q price=%v err=%v", p.PricingType, p.Price, err)
		}
	})
	t.Run("fixed", func(t *testing.T) {
		p := validPlanInput(`{}`)
		p.Price = 6
		if err := normalizePlan(&p); err != nil || p.PricingType != "fixed" {
			t.Fatalf("paid plan should normalize as fixed: type=%q err=%v", p.PricingType, err)
		}
	})
	t.Run("custom ignores fake amount", func(t *testing.T) {
		p := validPlanInput(`{"custom_domain_enabled":true}`)
		p.PricingType = "custom"
		p.Price = 999
		if err := normalizePlan(&p); err != nil || p.Price != 0 || p.BillingPeriod != "custom" {
			t.Fatalf("custom plan should not preserve a checkout amount: price=%v period=%q err=%v", p.Price, p.BillingPeriod, err)
		}
	})
	p := validPlanInput(`{}`)
	p.PricingType = "fixed"
	if normalizePlan(&p) == nil {
		t.Fatal("fixed-price plans must have a positive price")
	}
}

func TestNormalizePlanUsesCurrencyDecimalPrecision(t *testing.T) {
	for _, code := range []string{"KWD", "BHD", "OMR", "JOD"} {
		p := validPlanInput(`{}`)
		p.PricingType, p.BillingPeriod, p.Currency, p.Price = "fixed", "monthly", code, 1.234
		if err := normalizePlan(&p); err != nil {
			t.Errorf("%s should accept three decimals: %v", code, err)
		}
	}
	p := validPlanInput(`{}`)
	p.PricingType, p.BillingPeriod, p.Currency, p.Price = "fixed", "monthly", "SAR", 1.234
	if err := normalizePlan(&p); err == nil {
		t.Fatal("two-decimal currency accepted excess price precision")
	}
}
