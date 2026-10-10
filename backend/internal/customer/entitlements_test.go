package customer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNormalizePlanEntitlements(t *testing.T) {
	tests := []struct {
		name, features, limits string
		code                   string
		enabled                bool
		limit                  int
		invalid                bool
	}{
		{"boolean included", `["analytics"]`, `{}`, "analytics", true, 0, false},
		{"boolean excluded", `[]`, `{}`, "analytics", false, 0, false},
		{"positive limit", `[]`, `{"digital_cards_limit":5}`, "digital_cards", true, 5, false},
		{"zero limit", `[]`, `{"digital_cards_limit":0}`, "digital_cards", false, 0, false},
		{"unlimited", `[]`, `{"digital_cards_limit":-1}`, "digital_cards", true, -1, false},
		{"invalid negative", `[]`, `{"digital_cards_limit":-2}`, "digital_cards", false, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePlanEntitlements([]byte(tc.features), []byte(tc.limits))
			if tc.invalid {
				if err == nil {
					t.Fatal("expected invalid configuration error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got[tc.code].Enabled != tc.enabled || got[tc.code].Limit != tc.limit {
				t.Fatalf("got %#v", got[tc.code])
			}
		})
	}
}

// This test uses a dedicated disposable migrated DB URL and owns every row it
// creates. It never falls back to DATABASE_URL or any development setting.
func TestEntitlementsDatabaseScenarios(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("PLAN_WRITE_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("WEBSITE_TEST_DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL or WEBSITE_TEST_DATABASE_URL to run entitlement PostgreSQL tests")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable entitlement test database: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Ping(t.Context()); err != nil {
		t.Fatalf("ping disposable entitlement test database: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	userEmail := "core002-" + suffix + "@example.invalid"
	var userID int64
	if err := db.QueryRow(t.Context(), `INSERT INTO users(name,email) VALUES('CORE-002 fixture',$1) RETURNING id`, userEmail).Scan(&userID); err != nil {
		t.Fatalf("create fixture user: %v", err)
	}
	t.Cleanup(func() {
		_, e := db.Exec(context.Background(), `DELETE FROM subscriptions WHERE user_id=$1`, userID)
		if e != nil {
			t.Errorf("delete fixture subscriptions: %v", e)
		}
		_, e = db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
		if e != nil {
			t.Errorf("delete fixture user: %v", e)
		}
	})

	type planFixture struct {
		id   int64
		code string
	}
	createPlan := func(label, pricingType, features, limits string, order int) planFixture {
		t.Helper()
		code := "core002-" + label + "-" + suffix
		var id int64
		if err := db.QueryRow(t.Context(), `INSERT INTO website_pricing_plans(name,code,description,price,pricing_type,currency,billing_period,trial_days,features,limits,is_featured,is_enabled,sort_order) VALUES($1,$2,'',CASE WHEN $3='free' THEN 0 ELSE 19 END,$3,'USD','monthly',0,$4::jsonb,$5::jsonb,false,true,$6) RETURNING id`, strings.ToUpper(label)+" display", code, pricingType, features, limits, order).Scan(&id); err != nil {
			t.Fatalf("create %s fixture plan: %v", label, err)
		}
		t.Cleanup(func() {
			if _, e := db.Exec(context.Background(), `DELETE FROM subscriptions WHERE plan_id=$1`, id); e != nil {
				t.Errorf("delete fixture plan subscriptions %s: %v", label, e)
			}
			if _, e := db.Exec(context.Background(), `DELETE FROM website_pricing_plans WHERE id=$1`, id); e != nil {
				t.Errorf("delete fixture plan %s: %v", label, e)
			}
		})
		return planFixture{id, code}
	}
	free := createPlan("free", "free", `["analytics"]`, `{"digital_cards_limit":1,"contacts_limit":0}`, 0)
	professional := createPlan("professional", "fixed", `["analytics"]`, `{"digital_cards_limit":5,"contacts_limit":25}`, 1)
	business := createPlan("business", "fixed", `["analytics","custom_domain"]`, `{"digital_cards_limit":20,"contacts_limit":250}`, 2)
	enterprise := createPlan("enterprise", "fixed", `["analytics","custom_domain","branding_removal"]`, `{"digital_cards_limit":-1,"contacts_limit":-1}`, 3)
	h := &Handler{DB: db}
	insertSub := func(plan planFixture, status string, started, end string) int64 {
		t.Helper()
		var id int64
		if err := db.QueryRow(t.Context(), `INSERT INTO subscriptions(user_id,plan_id,status,started_at,ends_at) VALUES($1,$2,$3,$4::timestamptz,NULLIF($5,'')::timestamptz) RETURNING id`, userID, plan.id, status, started, end).Scan(&id); err != nil {
			t.Fatalf("create %s subscription: %v", status, err)
		}
		return id
	}
	clearSubs := func() {
		t.Helper()
		if _, err := db.Exec(t.Context(), `DELETE FROM subscriptions WHERE user_id=$1`, userID); err != nil {
			t.Fatalf("clear fixture subscriptions: %v", err)
		}
	}
	resolve := func() EntitlementResult {
		t.Helper()
		result, err := h.ResolveEntitlements(t.Context(), userID)
		if err != nil {
			t.Fatalf("resolve entitlement fixture: %v", err)
		}
		return result
	}
	assertPlan := func(result EntitlementResult, plan planFixture) {
		t.Helper()
		if result.Plan == nil || result.Plan.ID != plan.id || result.Plan.Code != plan.code {
			t.Fatalf("expected plan %s (%d), got %#v", plan.code, plan.id, result.Plan)
		}
	}

	t.Run("PlanFixtureMappingsDoNotDependOnDisplayNames", func(t *testing.T) {
		for _, tc := range []struct {
			p                 planFixture
			cards, contacts   int
			analytics, domain bool
		}{{free, 1, 0, true, false}, {professional, 5, 25, true, false}, {business, 20, 250, true, true}} {
			clearSubs()
			insertSub(tc.p, "active", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
			r := resolve()
			assertPlan(r, tc.p)
			if r.GetLimit("digital_cards") != tc.cards || r.GetLimit("contacts") != tc.contacts || r.HasFeature("analytics") != tc.analytics || r.HasFeature("custom_domain") != tc.domain {
				t.Fatalf("unexpected mapping for %s: %#v", tc.p.code, r.Entitlements)
			}
		}
		clearSubs()
		insertSub(enterprise, "active", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
		r := resolve()
		assertPlan(r, enterprise)
		if !r.IsUnlimited("digital_cards") || r.GetLimit("digital_cards") != -1 || !r.HasFeature("branding_removal") {
			t.Fatalf("enterprise mapping incorrect: %#v", r.Entitlements)
		}
	})

	t.Run("SubscriptionPrecedenceAndFallback", func(t *testing.T) {
		clearSubs()
		insertSub(professional, "active", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
		assertPlan(resolve(), professional)
		clearSubs()
		insertSub(professional, "active", time.Now().Add(-48*time.Hour).Format(time.RFC3339), time.Now().Add(-24*time.Hour).Format(time.RFC3339))
		assertPlan(resolve(), free)
		for _, status := range []string{"cancelled", "expired"} {
			clearSubs()
			insertSub(professional, status, time.Now().Add(-time.Hour).Format(time.RFC3339), "")
			assertPlan(resolve(), free)
		}
		clearSubs()
		assertPlan(resolve(), free)
		clearSubs()
		insertSub(professional, "active", time.Now().Add(24*time.Hour).Format(time.RFC3339), "")
		r := resolve()
		assertPlan(r, free)
		if r.GetLimit("digital_cards") != 1 {
			t.Fatal("future paid plan granted early")
		}
		clearSubs()
		insertSub(professional, "expired", time.Now().Add(-72*time.Hour).Format(time.RFC3339), "")
		insertSub(business, "active", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
		assertPlan(resolve(), business)
	})

	t.Run("EntitlementsEndpointAuthenticatedAndUnauthorized", func(t *testing.T) {
		clearSubs()
		insertSub(enterprise, "active", time.Now().Add(-time.Hour).Format(time.RFC3339), "")
		secret := "CORE002-test-signing-secret-that-is-long-enough"
		t.Setenv("JWT_SECRET", secret)
		router := chi.NewRouter()
		router.With(middleware.UserAuth).Get("/api/v1/user/me/entitlements", h.Entitlements)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me/entitlements", nil)
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.UserClaims{UserID: userID, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}})
		signed, e := token.SignedString([]byte(secret))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+signed)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated endpoint status %d: %s", w.Code, w.Body.String())
		}
		var envelope struct {
			Data EntitlementResult `json:"data"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil {
			t.Fatal(e)
		}
		assertPlan(envelope.Data, enterprise)
		if !envelope.Data.HasFeature("analytics") || !envelope.Data.HasFeature("custom_domain") || envelope.Data.GetLimit("digital_cards") != -1 {
			t.Fatalf("unexpected endpoint data: %#v", envelope.Data)
		}
		unauth := httptest.NewRequest(http.MethodGet, "/api/v1/user/me/entitlements", nil)
		uw := httptest.NewRecorder()
		router.ServeHTTP(uw, unauth)
		if uw.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated endpoint status %d: %s", uw.Code, uw.Body.String())
		}
	})
}

func TestEntitlementEnforcementHelpers(t *testing.T) {
	r := EntitlementResult{Entitlements: map[string]Entitlement{"cards": {Enabled: true, Limit: 2}, "unlimited": {Enabled: true, Limit: -1}, "off": {Limit: 0}}}
	if !r.HasFeature("cards") || !r.CanUse("cards", 1) || r.CanUse("cards", 2) {
		t.Fatal("limited feature enforcement is incorrect")
	}
	if !r.IsUnlimited("unlimited") || !r.CanUse("unlimited", 10000) || r.Remaining("unlimited", 10000) != -1 {
		t.Fatal("unlimited feature enforcement is incorrect")
	}
	if r.CanUse("off", 0) {
		t.Fatal("excluded feature should not be usable")
	}
}

func TestRegistryCodesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range FeatureRegistry {
		if f.Code == "" || seen[f.Code] || (f.Type != "boolean" && f.Type != "numeric") {
			t.Fatalf("invalid registry item: %#v", f)
		}
		seen[f.Code] = true
	}
	var parsed map[string]Entitlement
	if err := json.Unmarshal([]byte(`{"ok":{"enabled":true,"limit":-1}}`), &parsed); err != nil {
		t.Fatal(err)
	}
}
