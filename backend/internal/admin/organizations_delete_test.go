package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The only committed fixture is the explicitly disposable, unused organization.
func TestOrganizationDeleteCommittedPersistence(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for organization persistence check")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h := &ResourceHandler{DB: db}
	w := httptest.NewRecorder()
	h.CreateOrganization(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"name":"ADM ORG DELETE TEST","status":"active"}`)))
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var created struct {
		ID int64 `json:"id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &created); err != nil || created.ID == 0 {
		t.Fatalf("create response: %s", w.Body.String())
	}
	defer db.Exec(context.Background(), `DELETE FROM organizations WHERE id=$1`, created.ID)
	if w = organizationDeleteRequest(h, fmt.Sprint(created.ID)); w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	fresh, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	var exists bool
	if err = fresh.QueryRow(t.Context(), `SELECT EXISTS(SELECT 1 FROM organizations WHERE id=$1)`, created.ID).Scan(&exists); err != nil || exists {
		t.Fatalf("deletion did not commit: %v", err)
	}
	if w = organizationDeleteRequest(&ResourceHandler{DB: fresh}, fmt.Sprint(created.ID)); w.Code != 404 {
		t.Fatalf("deleted org returned: %d", w.Code)
	}
}

func organizationDeleteRequest(h *ResourceHandler, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("DELETE", "/api/v1/admin/organizations/"+id, nil)
	rc := chi.NewRouteContext()
	rc.URLParams.Add("id", id)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rc))
	w := httptest.NewRecorder()
	h.DeleteOrganization(w, r)
	return w
}

func TestOrganizationDeleteValidationAndSecurity(t *testing.T) {
	for _, id := range []string{"bad", "0", "-1", "9223372036854775808"} {
		if w := organizationDeleteRequest(&ResourceHandler{}, id); w.Code != 400 {
			t.Fatalf("%s: %d", id, w.Code)
		}
	}
	db, err := pgxpool.New(t.Context(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	w := organizationDeleteRequest(&ResourceHandler{DB: db}, "1")
	if w.Code != 500 || strings.Contains(w.Body.String(), "pool") || !strings.Contains(w.Body.String(), "Could not delete organization.") {
		t.Fatalf("unsafe error: %d %s", w.Code, w.Body.String())
	}
	never := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized delete reached handler") })
	path := "/api/v1/admin/organizations/1"
	w = httptest.NewRecorder()
	middleware.AdminAuth(never).ServeHTTP(w, httptest.NewRequest("DELETE", path, nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("DELETE", path, nil)
	r = r.WithContext(context.WithValue(r.Context(), middleware.AdminIDKey, int64(1)))
	w = httptest.NewRecorder()
	middleware.AdminAuthorization(db)(never).ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	if middleware.RequiredAdminPermission("DELETE", path) != "organizations.delete" {
		t.Fatal("wrong permission")
	}
}

func TestOrganizationDeleteDatabaseLifecycle(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for rollback-only organization checks")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	migration, err := os.ReadFile("../../migrations/000028_protect_organization_cards.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(t.Context(), string(migration)); err != nil {
		t.Fatal(err)
	}
	h := &ResourceHandler{DB: tx}
	var id int64
	if err = tx.QueryRow(t.Context(), `INSERT INTO organizations(name,status) VALUES('ADM ORG DELETE TEST','active') RETURNING id`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	list := func() map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		h.ListOrganizations(w, httptest.NewRequest("GET", "/?search=ADM+ORG+DELETE+TEST&status=active", nil))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		var v map[string]any
		json.Unmarshal(w.Body.Bytes(), &v)
		return v
	}
	before := list()["pagination"].(map[string]any)["total"].(float64)
	w := organizationDeleteRequest(h, fmt.Sprint(id))
	if w.Code != 200 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	for range 2 {
		v := list()
		if v["pagination"].(map[string]any)["total"].(float64) != before-1 {
			t.Fatal("count did not refresh")
		}
		for _, item := range v["data"].([]any) {
			if item.(map[string]any)["id"] == float64(id) {
				t.Fatal("deleted org returned")
			}
		}
	}
	if w = organizationDeleteRequest(h, fmt.Sprint(id)); w.Code != 404 {
		t.Fatalf("missing: %d", w.Code)
	}

	for _, kind := range []string{"cards", "subscriptions", "payments", "qr", "nfc"} {
		t.Run(kind, func(t *testing.T) {
			fixture, err := tx.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer fixture.Rollback(context.Background())
			exec := func(sql string, args ...any) {
				t.Helper()
				if _, err := fixture.Exec(t.Context(), sql, args...); err != nil {
					t.Fatal(err)
				}
			}
			rowID := func(sql string, args ...any) int64 {
				t.Helper()
				var n int64
				if err := fixture.QueryRow(t.Context(), sql, args...).Scan(&n); err != nil {
					t.Fatal(err)
				}
				return n
			}
			org := rowID(`INSERT INTO organizations(name) VALUES('ADM ORG DELETE TEST REFERENCED') RETURNING id`)
			user := rowID(`INSERT INTO users(name,email) VALUES('Organization deletion test',gen_random_uuid()::text||'@example.test') RETURNING id`)
			switch kind {
			case "cards":
				exec(`INSERT INTO digital_cards(user_id,organization_id,name,slug) VALUES($1,$2,'Protected card',gen_random_uuid()::text)`, user, org)
				guard, err := fixture.Begin(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				_, err = guard.Exec(t.Context(), `DELETE FROM organizations WHERE id=$1`, org)
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != "23503" {
					t.Fatalf("card reference must restrict deletion: %v", err)
				}
				guard.Rollback(t.Context())
			case "subscriptions":
				exec(`INSERT INTO subscriptions(organization_id) VALUES($1)`, org)
			case "payments":
				plan := rowID(`INSERT INTO website_pricing_plans(name,code) VALUES('Org delete test',gen_random_uuid()::text) RETURNING id`)
				exec(`INSERT INTO payments(user_id,organization_id,plan_id,provider_order_id,amount,currency) VALUES($1,$2,$3,gen_random_uuid()::text,0,'INR')`, user, org, plan)
			case "qr":
				exec(`INSERT INTO qr_codes(organization_id,code_value) VALUES($1,gen_random_uuid()::text)`, org)
			case "nfc":
				exec(`INSERT INTO nfc_tags(organization_id,uid,status) VALUES($1,gen_random_uuid()::text,'unassigned')`, org)
			}
			snapshot := func() string {
				t.Helper()
				var value string
				err := fixture.QueryRow(t.Context(), `SELECT jsonb_build_object('users',(SELECT jsonb_agg(to_jsonb(u)) FROM users u WHERE id=$1),'cards',(SELECT jsonb_agg(to_jsonb(c)) FROM digital_cards c WHERE organization_id=$2),'subscriptions',(SELECT jsonb_agg(to_jsonb(s)) FROM subscriptions s WHERE organization_id=$2),'payments',(SELECT jsonb_agg(to_jsonb(p)) FROM payments p WHERE organization_id=$2),'qr',(SELECT jsonb_agg(to_jsonb(q)) FROM qr_codes q WHERE organization_id=$2),'nfc',(SELECT jsonb_agg(to_jsonb(n)) FROM nfc_tags n WHERE organization_id=$2))::text`, user, org).Scan(&value)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
			before := snapshot()
			w := organizationDeleteRequest(&ResourceHandler{DB: fixture}, fmt.Sprint(org))
			if w.Code != 409 || !strings.Contains(w.Body.String(), "associated") {
				t.Fatalf("expected safe conflict: %d %s", w.Code, w.Body.String())
			}
			if before != snapshot() {
				t.Fatal("linked records changed")
			}
			if rowID(`SELECT id FROM organizations WHERE id=$1`, org) != org {
				t.Fatal("referenced org removed")
			}
		})
	}
}
