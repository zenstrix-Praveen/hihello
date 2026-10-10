package website

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Uses an existing migrated database inside a rollback-only transaction.
func TestFeaturesPageContentDatabaseLifecycle(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for rollback-only Features checks")
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	db, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(t.Context(), "BEGIN"); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), "ROLLBACK")
	var before, after []byte
	snapshot := `SELECT COALESCE(jsonb_agg(to_jsonb(f) ORDER BY id),'[]'::jsonb) FROM website_features f`
	if err = db.QueryRow(t.Context(), snapshot).Scan(&before); err != nil {
		t.Fatal(err)
	}
	h := &ContentHandler{DB: db}
	call := func(fn http.HandlerFunc, method, body string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(method, "/", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%d: %s", w.Code, w.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, action := range []string{"reset", "delete"} {
		call(h.SaveNamedSection("features"), "PUT", `{"page_heading":"ADM Features Test Heading","page_subtitle":"ADM Features Test Subtitle","enabled":true}`)
		for range 2 {
			if call(h.GetNamedSection("features"), "GET", "")["page_heading"] != "ADM Features Test Heading" {
				t.Fatal("save persistence")
			}
		}
		if action == "reset" {
			call(h.ResetFeaturesPageContent, "POST", "")
		} else {
			call(h.DeleteFeaturesPageContent, "DELETE", "")
		}
		for range 2 {
			got := call(h.GetNamedSection("features"), "GET", "")
			if action == "reset" && !reflect.DeepEqual(got, defaultFeaturesPageContent()) {
				t.Fatalf("reset defaults: %#v", got)
			}
			if action == "delete" && len(got) != 0 {
				t.Fatalf("custom config survives delete: %#v", got)
			}
			public := call((&FeatureHandler{DB: db}).PublicList, "GET", "")
			if public["page_heading"] == "ADM Features Test Heading" || public["page_subtitle"] == "ADM Features Test Subtitle" {
				t.Fatal("stale public content")
			}
			if _, ok := public["items"].([]any); !ok {
				t.Fatal("public cards missing")
			}
		}
		if err = db.QueryRow(t.Context(), snapshot).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("feature card data changed")
		}
		call(h.SaveNamedSection("features"), "PUT", `{"page_heading":"Professional Digital Card Features","page_subtitle":"New Features subtitle","enabled":true}`)
		if call((&FeatureHandler{DB: db}).PublicList, "GET", "")["page_heading"] != "Professional Digital Card Features" {
			t.Fatal("save again failed")
		}
	}
}

type pageContentDB struct {
	featureContractDB
	statements []string
}

func (db *pageContentDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if db.page == nil {
		return featureContractRow{err: pgx.ErrNoRows}
	}
	if strings.Contains(sql, "section_key=$1") {
		return featureContractRow{values: []any{json.RawMessage(db.page)}}
	}
	return featureContractRow{values: []any{db.page}}
}
func (db *pageContentDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.statements = append(db.statements, sql)
	if db.execErr != nil {
		return pgconn.CommandTag{}, db.execErr
	}
	if strings.HasPrefix(sql, "DELETE FROM website_content_sections") {
		db.page = nil
	} else if strings.HasPrefix(sql, "INSERT INTO website_content_sections") {
		db.page = append([]byte(nil), args[1].([]byte)...)
	} else {
		return pgconn.CommandTag{}, errors.New("unexpected mutation")
	}
	return pgconn.NewCommandTag("UPDATE 1"), nil
}
func TestFeaturesPageContentLifecycle(t *testing.T) {
	db := &pageContentDB{featureContractDB: featureContractDB{rows: [][]any{{int64(7), "QR Sharing", "description", "detail", "qr", "", 3, true, "", ""}}}}
	h := &ContentHandler{DB: db}
	call := func(fn http.HandlerFunc, method, body string) map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(method, "/", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	defaults := map[string]any{"page_heading": "Everything Your Digital Card Needs", "page_subtitle": "Everything you need to create, share and grow your professional digital identity.", "enabled": true}
	for _, action := range []string{"reset", "delete"} {
		call(h.SaveNamedSection("features"), "PUT", `{"page_heading":"Everything Your Digital Card Needs hihello","page_subtitle":"Custom subtitle","enabled":false}`)
		if call(h.GetNamedSection("features"), "GET", "")["page_subtitle"] != "Custom subtitle" {
			t.Fatal("save did not persist")
		}
		var result map[string]any
		if action == "reset" {
			result = call(h.ResetFeaturesPageContent, "POST", "")
		} else {
			result = call(h.DeleteFeaturesPageContent, "DELETE", "")
		}
		if result["success"] != true {
			t.Fatalf("bad contract: %#v", result)
		}
		for range 2 {
			saved := call(h.GetNamedSection("features"), "GET", "")
			if action == "reset" && !reflect.DeepEqual(saved, defaults) {
				t.Fatalf("wrong defaults: %#v", saved)
			}
			if action == "delete" && len(saved) != 0 {
				t.Fatalf("custom values returned: %#v", saved)
			}
			public := call((&FeatureHandler{DB: db}).PublicList, "GET", "")
			if strings.Contains(string(db.page), "hihello") || len(public["items"].([]any)) != 1 {
				t.Fatal("stale content or missing cards")
			}
			if public["items"].([]any)[0].(map[string]any)["title"] != "QR Sharing" {
				t.Fatal("card changed")
			}
		}
		call(h.SaveNamedSection("features"), "PUT", `{"page_heading":"Professional Digital Card Features","page_subtitle":"New subtitle","enabled":true}`)
		if call((&FeatureHandler{DB: db}).PublicList, "GET", "")["page_heading"] != "Professional Digital Card Features" {
			t.Fatal("save again failed")
		}
	}
	for _, sql := range db.statements {
		if strings.Contains(sql, "website_features") {
			t.Fatalf("card mutation: %s", sql)
		}
	}
}
func TestFeaturesPageContentErrorsAndSecurity(t *testing.T) {
	pool, err := pgxpool.New(t.Context(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	for _, action := range []string{"reset", "delete"} {
		db := &pageContentDB{featureContractDB: featureContractDB{page: []byte(`{"page_heading":"Keep"}`), execErr: errors.New("SQL private exception")}}
		h := &ContentHandler{DB: db}
		fn := h.ResetFeaturesPageContent
		method, path := "POST", "/api/v1/admin/website/sections/features/reset"
		if action == "delete" {
			fn = h.DeleteFeaturesPageContent
			method, path = "DELETE", "/api/v1/admin/website/sections/features"
		}
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(method, path, nil))
		if w.Code != 500 || strings.Contains(w.Body.String(), "SQL") || !strings.Contains(w.Body.String(), `"success":false`) || string(db.page) != `{"page_heading":"Keep"}` {
			t.Fatalf("unsafe failure: %s", w.Body.String())
		}
		never := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("unauthorized mutation") })
		w = httptest.NewRecorder()
		middleware.AdminAuth(never).ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 401 {
			t.Fatal(w.Code)
		}
		r := httptest.NewRequest(method, path, nil)
		r = r.WithContext(context.WithValue(r.Context(), middleware.AdminIDKey, int64(1)))
		w = httptest.NewRecorder()
		middleware.AdminAuthorization(pool)(never).ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
		if middleware.RequiredAdminPermission(method, path) != "website.manage" {
			t.Fatal("missing permission")
		}
	}
}
