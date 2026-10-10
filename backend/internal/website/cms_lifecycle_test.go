package website

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Opt in only for a database where temporary CMS records may be created.
func TestCMSDatabaseLifecycle(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for temporary-record lifecycle checks")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	feature := &FeatureHandler{DB: db}
	pricing := &ContentHandler{DB: db}
	templates := &TemplateHandler{DB: db}
	publicTemplates := &PublicTemplateHandler{DB: db}
	request := func(t *testing.T, handler http.HandlerFunc, method string, id int64, payload any) any {
		t.Helper()
		raw, _ := json.Marshal(payload)
		r := httptest.NewRequest(method, "/", strings.NewReader(string(raw)))
		route := chi.NewRouteContext()
		route.URLParams.Add("id", strconv.FormatInt(id, 10))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code < 200 || w.Code >= 300 {
			t.Fatalf("%s: HTTP %d: %s", method, w.Code, w.Body.String())
		}
		var result any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	items := func(value any) []any {
		if rows, ok := value.([]any); ok {
			return rows
		}
		m := value.(map[string]any)
		if rows, ok := m["items"].([]any); ok {
			return rows
		}
		return m["data"].([]any)
	}
	for _, tc := range []struct {
		name, table, title                    string
		create, update, remove, admin, public http.HandlerFunc
		payload                               map[string]any
	}{
		{"features", "website_features", "title", feature.Create, feature.Update, feature.Delete, feature.List, feature.PublicList, map[string]any{"title": "temporary", "description": "original", "is_enabled": true}},
		{"pricing", "website_pricing_plans", "name", pricing.SavePricing, pricing.SavePricing, pricing.DeletePricing, pricing.ListPricing, pricing.PublicPricing, map[string]any{"name": "temporary", "description": "original", "currency": "USD", "price": 12, "pricing_type": "fixed", "billing_period": "monthly", "is_enabled": true, "cta_text": "Explore", "cta_url": "/contact"}},
		{"templates", "website_templates", "name", templates.Create, templates.Update, templates.Delete, templates.List, publicTemplates.List, map[string]any{"name": "temporary", "description": "original", "is_enabled": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := fmt.Sprintf("cms-regression-%s-%d", tc.name, time.Now().UnixNano())
			tc.payload[tc.title] = marker
			tc.payload["sort_order"] = 0
			if tc.name == "templates" {
				tc.payload["slug"] = marker
			}
			result := request(t, tc.create, "POST", 0, tc.payload).(map[string]any)
			id := int64(result["id"].(float64))
			defer func() {
				if _, err := db.Exec(context.Background(), "DELETE FROM "+tc.table+" WHERE id=$1", id); err != nil {
					t.Errorf("cleanup temporary record: %v", err)
				}
			}()
			verify := func(handler http.HandlerFunc, want bool, description string) {
				t.Helper()
				found := false
				for _, raw := range items(request(t, handler, "GET", 0, nil)) {
					item := raw.(map[string]any)
					if int64(item["id"].(float64)) == id {
						found = true
						if item["description"] != description {
							t.Fatalf("stale description: %#v", item)
						}
						if tc.name == "pricing" && item["cta_url"] != "/contact" {
							t.Fatalf("missing saved CTA: %#v", item)
						}
					}
				}
				if found != want {
					t.Fatalf("record visible=%v, want %v", found, want)
				}
			}
			verify(tc.admin, true, "original")
			verify(tc.public, true, "original")
			tc.payload[tc.title] = marker + " edited"
			tc.payload["description"] = "updated"
			request(t, tc.update, "PUT", id, tc.payload)
			verify(tc.admin, true, "updated")
			verify(tc.public, true, "updated")
			tc.payload["is_enabled"] = false
			request(t, tc.update, "PUT", id, tc.payload)
			verify(tc.admin, true, "updated")
			verify(tc.public, false, "")
			tc.payload["is_enabled"] = true
			request(t, tc.update, "PUT", id, tc.payload)
			verify(tc.public, true, "updated")
			request(t, tc.remove, "DELETE", id, nil)
			verify(tc.admin, false, "")
			verify(tc.public, false, "")
			verify(tc.public, false, "")
			if tc.name == "pricing" {
				var archived bool
				if err := db.QueryRow(t.Context(), `SELECT archived_at IS NOT NULL AND NOT is_enabled FROM website_pricing_plans WHERE id=$1`, id).Scan(&archived); err != nil || !archived {
					t.Fatalf("plan must be archived: %v", err)
				}
			}
		})
	}
}

// A single pooled connection holds a rollback-only transaction so settings are
// exercised through the real handlers without changing existing site copy.
func TestCMSSectionVisibilityDatabase(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for section lifecycle checks")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1
	db, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(t.Context(), "BEGIN"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), "ROLLBACK")
	admin := &ContentHandler{DB: db}
	public := &PublicContentHandler{DB: db}
	for _, section := range []string{"features", "templates", "pricing", "business", "contact", "header", "footer", "seo"} {
		t.Run(section, func(t *testing.T) {
			for _, enabled := range []bool{true, false, true} {
				data := map[string]any{"enabled": enabled}
				raw, _ := json.Marshal(data)
				w := httptest.NewRecorder()
				admin.SaveNamedSection(section)(w, httptest.NewRequest("PUT", "/", strings.NewReader(string(raw))))
				if w.Code != 200 {
					t.Fatalf("save: %d %s", w.Code, w.Body.String())
				}
				w = httptest.NewRecorder()
				admin.GetNamedSection(section)(w, httptest.NewRequest("GET", "/", nil))
				var saved map[string]any
				json.Unmarshal(w.Body.Bytes(), &saved)
				if w.Code != 200 || saved["enabled"] != enabled {
					t.Fatalf("admin lost unpublished record: %s", w.Body.String())
				}
				w = httptest.NewRecorder()
				public.Section(section)(w, httptest.NewRequest("GET", "/", nil))
				want := 200
				if !enabled {
					want = 404
				}
				if w.Code != want {
					t.Fatalf("public status %d, want %d", w.Code, want)
				}
			}
		})
	}
	home := &Handler{DB: db}
	w := httptest.NewRecorder()
	home.UpdateHome(w, httptest.NewRequest("PUT", "/", strings.NewReader(`{"hero_title":"Temporary CMS home","hero_subtitle":"Saved copy","primary_button_url":"/contact"}`)))
	if w.Code != 200 {
		t.Fatalf("home save: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	home.GetHome(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Temporary CMS home") {
		t.Fatalf("home did not persist: %s", w.Body.String())
	}
}
