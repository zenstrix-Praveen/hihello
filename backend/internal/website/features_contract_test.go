package website

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Only the database boundary is stubbed: requests run the real handlers and JSON writer.
type featureContractDB struct {
	rows                                           [][]any
	queryErr, scanErr, rowsErr, execErr, commitErr error
	affected                                       int64
	page                                           []byte
}

func (db *featureContractDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return &featureContractRows{values: db.rows, scanErr: db.scanErr, rowsErr: db.rowsErr}, db.queryErr
}
func (db *featureContractDB) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if strings.HasPrefix(sql, "INSERT") {
		return featureContractRow{values: []any{int64(7)}, err: db.execErr}
	}
	return featureContractRow{values: []any{db.page}}
}
func (db *featureContractDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	if db.affected == 0 {
		return pgconn.NewCommandTag("UPDATE 0"), db.execErr
	}
	return pgconn.NewCommandTag("UPDATE 1"), db.execErr
}
func (db *featureContractDB) Begin(context.Context) (pgx.Tx, error) {
	return &featureContractTx{db: db}, db.execErr
}

type featureContractTx struct {
	pgx.Tx
	db *featureContractDB
}

func (tx *featureContractTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.db.Exec(ctx, sql, args...)
}
func (tx *featureContractTx) Commit(context.Context) error { return tx.db.commitErr }
func (*featureContractTx) Rollback(context.Context) error  { return nil }

type featureContractRow struct {
	values []any
	err    error
}

func (row featureContractRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	for i, value := range row.values {
		reflect.ValueOf(dest[i]).Elem().Set(reflect.ValueOf(value))
	}
	return nil
}

type featureContractRows struct {
	pgx.Rows
	values           [][]any
	index            int
	scanErr, rowsErr error
}

func (*featureContractRows) Close()          {}
func (rows *featureContractRows) Err() error { return rows.rowsErr }
func (rows *featureContractRows) Next() bool { rows.index++; return rows.index <= len(rows.values) }
func (rows *featureContractRows) Scan(dest ...any) error {
	return (featureContractRow{values: rows.values[rows.index-1], err: rows.scanErr}).Scan(dest...)
}

func featureContractRequest(t *testing.T, handler http.Handler, method, body string) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(method, "/api/v1/admin/website/features/7", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "7")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	var result map[string]any
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("not JSON: %s", w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return w.Code, result
}

func TestFeatureSuccessContracts(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		status             int
		selectHandler      func(*FeatureHandler) http.HandlerFunc
	}{
		{"create", "POST", `{"title":"Test","is_enabled":true}`, 201, func(h *FeatureHandler) http.HandlerFunc { return h.Create }},
		{"update", "PUT", `{"title":"Test","is_enabled":true}`, 200, func(h *FeatureHandler) http.HandlerFunc { return h.Update }},
		{"delete", "DELETE", "", 200, func(h *FeatureHandler) http.HandlerFunc { return h.Delete }},
		{"order", "PUT", `{"ids":[7]}`, 200, func(h *FeatureHandler) http.HandlerFunc { return h.SetOrder }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &FeatureHandler{DB: &featureContractDB{affected: 1}}
			status, body := featureContractRequest(t, tc.selectHandler(h), tc.method, tc.body)
			if status != tc.status || body["success"] != true || body["message"] == nil || body["message"] == "" {
				t.Fatalf("unexpected success contract: %d %#v", status, body)
			}
			data, exists := body["data"]
			if !exists {
				t.Fatal("missing data key")
			}
			if tc.name == "create" || tc.name == "update" {
				record, ok := data.(map[string]any)
				if !ok || record["id"] != float64(7) || record["title"] != "Test" || body["id"] != record["id"] || body["title"] != record["title"] {
					t.Fatalf("record and legacy fields must agree: %#v", body)
				}
				for key, value := range record {
					if !reflect.DeepEqual(body[key], value) {
						t.Fatalf("legacy field %s differs from data: %#v", key, body)
					}
				}
			} else if data != nil {
				t.Fatalf("expected null data: %#v", data)
			}
		})
	}
}

func TestFeatureListContractAuthenticated(t *testing.T) {
	t.Setenv("JWT_SECRET", "feature-contract-test-secret")
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, middleware.AdminClaims{
		AdminID: 1, Role: "super_admin", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte("feature-contract-test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rows := range [][][]any{nil, {{int64(7), "Test", "Description", "Details", "icon", "/image.png", 0, true, "Read", "/features"}}} {
		h := &FeatureHandler{DB: &featureContractDB{rows: rows}}
		handler := middleware.AdminAuth(http.HandlerFunc(h.List))
		status, body := featureContractRequest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+token)
			handler.ServeHTTP(w, r)
		}), "GET", "")
		items, ok := body["items"].([]any)
		if status != 200 || !ok || len(items) != len(rows) || body["success"] != true || !reflect.DeepEqual(items, body["data"]) {
			t.Fatalf("frontend requires non-null items array, matching data: %d %#v", status, body)
		}
	}
}

func TestFeaturePublicContractPreservesPage(t *testing.T) {
	h := &FeatureHandler{DB: &featureContractDB{page: []byte(`{"enabled":true,"page_heading":"Heading","page_subtitle":"Subtitle"}`)}}
	status, body := featureContractRequest(t, http.HandlerFunc(h.PublicList), "GET", "")
	if status != 200 || body["success"] != true || body["page_heading"] != "Heading" || body["page_subtitle"] != "Subtitle" || body["enabled"] != true || !reflect.DeepEqual(body["items"], []any{}) || !reflect.DeepEqual(body["items"], body["data"]) {
		t.Fatalf("public page fields and items must survive: %d %#v", status, body)
	}
}

func TestFeatureFailureContracts(t *testing.T) {
	internal := errors.New("SQL failure: private connection details")
	for _, tc := range []struct {
		name          string
		db            featureContractDB
		handler       func(*FeatureHandler) http.HandlerFunc
		method, input string
		status        int
	}{
		{"list query", featureContractDB{queryErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.List }, "GET", "", 500},
		{"list scan", featureContractDB{rows: [][]any{{}}, scanErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.List }, "GET", "", 500},
		{"list stream", featureContractDB{rowsErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.List }, "GET", "", 500},
		{"public query", featureContractDB{queryErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.PublicList }, "GET", "", 500},
		{"public scan", featureContractDB{rows: [][]any{{}}, scanErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.PublicList }, "GET", "", 500},
		{"public stream", featureContractDB{rowsErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.PublicList }, "GET", "", 500},
		{"unpublished", featureContractDB{page: []byte(`{"enabled":false}`)}, func(h *FeatureHandler) http.HandlerFunc { return h.PublicList }, "GET", "", 404},
		{"create DB", featureContractDB{execErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.Create }, "POST", `{"title":"Test"}`, 500},
		{"update DB", featureContractDB{execErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.Update }, "PUT", `{"title":"Test"}`, 500},
		{"update missing", featureContractDB{}, func(h *FeatureHandler) http.HandlerFunc { return h.Update }, "PUT", `{"title":"Test"}`, 404},
		{"delete DB", featureContractDB{execErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.Delete }, "DELETE", "", 500},
		{"delete missing", featureContractDB{}, func(h *FeatureHandler) http.HandlerFunc { return h.Delete }, "DELETE", "", 404},
		{"order DB", featureContractDB{execErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.SetOrder }, "PUT", `{"ids":[7]}`, 500},
		{"order missing", featureContractDB{}, func(h *FeatureHandler) http.HandlerFunc { return h.SetOrder }, "PUT", `{"ids":[7]}`, 400},
		{"order commit", featureContractDB{affected: 1, commitErr: internal}, func(h *FeatureHandler) http.HandlerFunc { return h.SetOrder }, "PUT", `{"ids":[7]}`, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &FeatureHandler{DB: &tc.db}
			status, body := featureContractRequest(t, tc.handler(h), tc.method, tc.input)
			message, ok := body["message"].(string)
			if status != tc.status || body["success"] != false || !ok || message == "" || body["error"] != message || strings.Contains(message, "SQL") || strings.Contains(message, "private") {
				t.Fatalf("unsafe or inconsistent failure: %d %#v", status, body)
			}
		})
	}
}

func TestFeatureSecurityContracts(t *testing.T) {
	// A closed pool makes the real authorization middleware deny access without a live DB.
	db, err := pgxpool.New(t.Context(), "postgres://test:test@127.0.0.1:1/test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	never := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("denied request reached handler") })
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		status, body := featureContractRequest(t, middleware.AdminAuth(never), method, "")
		if status != 401 || body["error"] == nil {
			t.Fatalf("expected JSON 401, got %d %#v", status, body)
		}
		status, body = featureContractRequest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), middleware.AdminIDKey, int64(1)))
			middleware.AdminAuthorization(db)(never).ServeHTTP(w, r)
		}), method, "")
		if status != 403 || body["error"] == nil {
			t.Fatalf("expected fail-closed JSON 403, got %d %#v", status, body)
		}
		want := "website.manage"
		if method == "GET" {
			want = "website.view"
		}
		if got := middleware.RequiredAdminPermission(method, "/api/v1/admin/website/features/7"); got != want {
			t.Fatalf("permission = %q, want %q", got, want)
		}
	}
}
