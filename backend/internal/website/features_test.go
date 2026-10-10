package website

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFeatureValidationErrorContract(t *testing.T) {
	h := &FeatureHandler{}
	for _, tc := range []struct {
		name, method, id, body, message string
		status                          int
		handler                         http.HandlerFunc
	}{
		{"missing title", "POST", "", `{}`, "Feature title is required", http.StatusUnprocessableEntity, h.Create},
		{"invalid JSON", "POST", "", `{`, "Feature title is required", http.StatusUnprocessableEntity, h.Create},
		{"unknown field", "POST", "", `{"title":"Test","unknown":true}`, "Feature title is required", http.StatusUnprocessableEntity, h.Create},
		{"unsafe URL", "POST", "", `{"title":"Test","cta_url":"javascript:alert(1)"}`, "Feature URLs must be safe HTTP(S) or relative URLs", http.StatusUnprocessableEntity, h.Create},
		{"update title", "PUT", "1", `{}`, "Feature title is required", http.StatusUnprocessableEntity, h.Update},
		{"update id", "PUT", "bad", `{}`, "Invalid feature id", http.StatusBadRequest, h.Update},
		{"delete id", "DELETE", "0", ``, "Invalid feature id", http.StatusBadRequest, h.Delete},
		{"order JSON", "PUT", "", `{`, "Invalid feature order", http.StatusUnprocessableEntity, h.SetOrder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/", strings.NewReader(tc.body))
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tc.id)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
			w := httptest.NewRecorder()
			tc.handler(w, r)
			var body map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body["success"] != false || body["message"] != tc.message || body["error"] != tc.message {
				t.Fatalf("expected HTTP %d with structured error and legacy error field, got %d: %s", tc.status, w.Code, w.Body.String())
			}
		})
	}
}

// Set WEBSITE_TEST_DATABASE_URL to run this read-only contract regression
// against the existing website_features and website_content_sections tables.
func TestFeatureAdminAndPublicResponseShapes(t *testing.T) {
	dsn := os.Getenv("WEBSITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_TEST_DATABASE_URL to run feature response contract test")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(db.Close)

	t.Run("admin feature list uses items envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&FeatureHandler{DB: db}).List(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 200 {
			t.Fatalf("admin list returned %d", w.Code)
		}
		var response featureListResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Items == nil {
			t.Fatalf("admin feature response must include a non-null items array: %s (%v)", w.Body.String(), err)
		}
	})

	t.Run("public features keep page settings and items together", func(t *testing.T) {
		w := httptest.NewRecorder()
		(&FeatureHandler{DB: db}).PublicList(w, httptest.NewRequest("GET", "/", nil))
		if w.Code != 200 {
			t.Fatalf("public list returned %d", w.Code)
		}
		var response struct {
			Items []Feature `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Items == nil {
			t.Fatalf("public features response must include an items array: %s (%v)", w.Body.String(), err)
		}
	})
}

func TestFeatureListEmptyItemsSerializeAsArray(t *testing.T) {
	encoded, err := json.Marshal(featureListResponse{Items: make([]Feature, 0)})
	if err != nil {
		t.Fatal(err)
	}
	var response featureListResponse
	if err := json.Unmarshal(encoded, &response); err != nil || response.Items == nil || len(response.Items) != 0 {
		t.Fatalf("empty admin feature list must serialize as items: []: %s (%v)", encoded, err)
	}
}

// WEBSITE_WRITE_TEST_DATABASE_URL must point to a disposable migrated DB.
// The test creates and then removes only its own feature through the handler.
func TestCreateFeatureThenListReturnsCreatedItem(t *testing.T) {
	dsn := os.Getenv("WEBSITE_WRITE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set WEBSITE_WRITE_TEST_DATABASE_URL to run write endpoint integration tests against a disposable database")
	}
	db, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatalf("connect to disposable test database: %v", err)
	}
	t.Cleanup(db.Close)
	h := &FeatureHandler{DB: db}
	createBody, _ := json.Marshal(Feature{Title: "Feature list contract regression", Description: "test-only"})
	created := httptest.NewRecorder()
	h.Create(created, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(createBody)))
	if created.Code != http.StatusCreated {
		t.Fatalf("create returned %d: %s", created.Code, created.Body.String())
	}
	var item Feature
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.ID == 0 {
		t.Fatalf("create response did not contain an item: %s (%v)", created.Body.String(), err)
	}
	deleteCreated := func() {
		r := httptest.NewRequest(http.MethodDelete, "/", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", fmt.Sprint(item.ID))
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		h.Delete(httptest.NewRecorder(), r)
	}
	t.Cleanup(deleteCreated)
	listed := httptest.NewRecorder()
	h.List(listed, httptest.NewRequest(http.MethodGet, "/", nil))
	if listed.Code != http.StatusOK {
		t.Fatalf("list returned %d: %s", listed.Code, listed.Body.String())
	}
	var response featureListResponse
	if err := json.Unmarshal(listed.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, got := range response.Items {
		if got.ID == item.ID {
			return
		}
	}
	t.Fatalf("created feature %d was missing from list response: %s", item.ID, listed.Body.String())
}
