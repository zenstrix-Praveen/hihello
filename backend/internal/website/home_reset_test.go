package website

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHomeResetLifecycle(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL for rollback-only Home checks")
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
	h := &Handler{DB: db}
	request := func(fn http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest(method, "/api/v1/admin/website/home/reset", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
		}
		return w
	}
	get := func() HomeContent {
		t.Helper()
		var home HomeContent
		w := request(h.GetHome, "GET", "")
		if err := json.Unmarshal(w.Body.Bytes(), &home); err != nil {
			t.Fatal(err)
		}
		return home
	}
	request(h.UpdateHome, "PUT", `{"hero_title":"Temporary CMS home","hero_subtitle":"Saved copy","primary_button_text":"Custom primary","primary_button_url":"/custom","secondary_button_text":"Custom secondary","secondary_button_url":"/other","hero_image_url":"/custom.png"}`)
	if got := get(); got.HeroTitle != "Temporary CMS home" {
		t.Fatalf("save failed: %#v", got)
	}
	for range 2 {
		w := request(h.ResetHome, "POST", "")
		var result struct {
			Success bool        `json:"success"`
			Data    HomeContent `json:"data"`
		}
		json.Unmarshal(w.Body.Bytes(), &result)
		if !result.Success || result.Data.HeroTitle != "" || result.Data.HeroSubtitle != "" || result.Data.Translations != nil {
			t.Fatalf("reset response: %s", w.Body.String())
		}
		for range 2 {
			if got := get(); got.HeroTitle != "" || got.HeroSubtitle != "" || got.PrimaryButtonText != "" || got.Translations != nil {
				t.Fatalf("custom values returned after refresh: %#v", got)
			}
		}
	}
	var count int
	if err := db.QueryRow(t.Context(), "SELECT COUNT(*) FROM website_home WHERE id=1").Scan(&count); err != nil || count != 1 {
		t.Fatalf("singleton removed: count=%d err=%v", count, err)
	}
	request(h.UpdateHome, "PUT", `{"hero_title":"New Home Title","hero_subtitle":"New Home Subtitle"}`)
	if got := get(); got.HeroTitle != "New Home Title" || got.HeroSubtitle != "New Home Subtitle" {
		t.Fatalf("save after reset failed: %#v", got)
	}
}

func TestHomeResetUnauthorized(t *testing.T) {
	h := &Handler{}
	w := httptest.NewRecorder()
	middleware.AdminAuth(http.HandlerFunc(h.ResetHome)).ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/admin/website/home/reset", nil))
	if w.Code != 401 {
		t.Fatalf("unauthorized reset status %d", w.Code)
	}
	if got := middleware.RequiredAdminPermission("POST", "/api/v1/admin/website/home/reset"); got != "website.manage" {
		t.Fatalf("reset permission %q", got)
	}
}
func TestHomeResetDatabaseErrorIsSafe(t *testing.T) {
	db, err := pgxpool.New(t.Context(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	w := httptest.NewRecorder()
	(&Handler{DB: db}).ResetHome(w, httptest.NewRequest("POST", "/", nil))
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != 500 || result["success"] != false || result["error"] != "Could not reset Home Page content." {
		t.Fatalf("unsafe error: %s", w.Body.String())
	}
}
