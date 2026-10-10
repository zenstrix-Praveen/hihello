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

	"digital-card-platform/backend/internal/customer"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPlanEntitlementsRejectMalformedID(t *testing.T) {
	h := &PlansBillingHandler{}
	router := chi.NewRouter()
	router.Get("/admin/plans/{id}/entitlements", h.GetPlanEntitlements)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/plans/not-a-number/entitlements", nil))
	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode malformed ID response: %v", err)
	}
	if w.Code != http.StatusBadRequest || response["error"] != "Invalid plan id" {
		t.Fatalf("expected a safe 400 for malformed plan ID, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPlanEntitlementEndpointsRoundTrip(t *testing.T) {
	dsn := os.Getenv("PLAN_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set PLAN_WRITE_TEST_DATABASE_URL to run plan entitlement PostgreSQL tests")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable plan test database: %v", err)
	}
	t.Cleanup(db.Close)
	if err := db.Ping(t.Context()); err != nil {
		t.Fatalf("ping disposable plan test database: %v", err)
	}
	code := fmt.Sprintf("adm003-%d", time.Now().UnixNano())
	var id int64
	err = db.QueryRow(t.Context(), `INSERT INTO website_pricing_plans(name,code,description,price,pricing_type,currency,billing_period,trial_days,features,limits,is_featured,is_enabled,sort_order) VALUES('ADM-003 fixture',$1,'',20,'fixed','USD','monthly',0,'[]'::jsonb,'{}'::jsonb,false,true,0) RETURNING id`, code).Scan(&id)
	if err != nil {
		t.Fatalf("create plan fixture: %v", err)
	}
	t.Cleanup(func() {
		if _, e := db.Exec(context.Background(), `DELETE FROM website_pricing_plans WHERE id=$1`, id); e != nil {
			t.Errorf("delete plan fixture: %v", e)
		}
	})

	h := &PlansBillingHandler{DB: db}
	router := chi.NewRouter()
	router.Get("/admin/plans/feature-registry", h.FeatureRegistry)
	router.Get("/admin/plans/{id}/entitlements", h.GetPlanEntitlements)
	router.Put("/admin/plans/{id}/entitlements", h.SavePlanEntitlements)

	registryResponse := httptest.NewRecorder()
	router.ServeHTTP(registryResponse, httptest.NewRequest(http.MethodGet, "/admin/plans/feature-registry", nil))
	if registryResponse.Code != 200 {
		t.Fatalf("registry returned %d: %s", registryResponse.Code, registryResponse.Body.String())
	}
	var registryEnvelope struct {
		Data []customer.Feature `json:"data"`
	}
	if err := json.Unmarshal(registryResponse.Body.Bytes(), &registryEnvelope); err != nil || len(registryEnvelope.Data) == 0 {
		t.Fatalf("invalid registry response: %s (%v)", registryResponse.Body.String(), err)
	}

	configured := map[string]customer.Entitlement{}
	for _, feature := range customer.FeatureRegistry {
		if feature.Type == "numeric" {
			configured[feature.Code] = customer.Entitlement{Enabled: true, Limit: 5}
		} else {
			configured[feature.Code] = customer.Entitlement{Enabled: feature.Code == "analytics"}
		}
	}
	configured["digital_cards"] = customer.Entitlement{Enabled: true, Limit: 5}
	configured["team_members"] = customer.Entitlement{Enabled: true, Limit: -1}
	configured["contacts"] = customer.Entitlement{Enabled: false, Limit: 0}
	body, _ := json.Marshal(map[string]any{"entitlements": configured})
	put := httptest.NewRecorder()
	router.ServeHTTP(put, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/plans/%d/entitlements", id), bytes.NewReader(body)))
	if put.Code != 200 {
		t.Fatalf("save returned %d: %s", put.Code, put.Body.String())
	}
	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/plans/%d/entitlements", id), nil))
	if get.Code != 200 {
		t.Fatalf("read returned %d: %s", get.Code, get.Body.String())
	}
	var response struct {
		Data struct {
			PlanID       int64                           `json:"plan_id"`
			Entitlements map[string]customer.Entitlement `json:"entitlements"`
		} `json:"data"`
	}
	if err := json.Unmarshal(get.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.PlanID != id || response.Data.Entitlements["digital_cards"].Limit != 5 || response.Data.Entitlements["team_members"].Limit != -1 || response.Data.Entitlements["contacts"].Enabled || !response.Data.Entitlements["analytics"].Enabled {
		t.Fatalf("unexpected saved values: %#v", response.Data)
	}
	configured["digital_cards"] = customer.Entitlement{Enabled: false, Limit: -2}
	body, _ = json.Marshal(map[string]any{"entitlements": configured})
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/plans/%d/entitlements", id), bytes.NewReader(body)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid negative limit returned %d: %s", invalid.Code, invalid.Body.String())
	}
}
