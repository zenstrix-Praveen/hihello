package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"digital-card-platform/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRegisterRejectsInvalidRequests(t *testing.T) {
	for _, body := range []string{
		`{`, `null`, `{}`, `{"name":" ","email":"a@example.com","password":"12345678"}`,
		`{"name":"A","email":"invalid","password":"12345678"}`,
		`{"name":"A","email":"a@example.com","password":"1234567"}`,
		`{"name":"A","email":"a@example.com","password":"界界界"}`,
		`{"name":"A","email":"a@example.com","password":"` + strings.Repeat("a", 73) + `"}`,
		`{"name":"A","email":"a@example.com","password":"12345678"} {}`,
	} {
		r := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(body))
		w := httptest.NewRecorder()
		(&UserAuthHandler{}).Register(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	}
}

func TestUserTokenCompatibility(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-signing-key")
	token, err := createUserToken(42, "visitor@example.com")
	if err != nil {
		t.Fatal(err)
	}
	claims := &userClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte("test-only-signing-key"), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		t.Fatal("invalid signed token")
	}
	if claims.UserID != 42 || claims.Subject != "visitor@example.com" || claims.ExpiresAt.Sub(claims.IssuedAt.Time).Hours() != 24 {
		t.Fatal("unexpected JWT claims")
	}
	r := httptest.NewRequest(http.MethodGet, "/billing/history", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	middleware.UserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(middleware.UserIDKey) != int64(42) {
			t.Error("user ID missing")
		}
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatal("existing middleware rejected token")
	}
}

func TestUserTokenCompatibilityWithPaddedSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "  test-only-signing-key\n")
	token, err := createUserToken(42, "visitor@example.com")
	if err != nil || token == "" {
		t.Fatal("could not issue customer token")
	}
	r := httptest.NewRequest(http.MethodGet, "/user/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	middleware.UserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("fresh customer token rejected: status %d", w.Code)
	}
}

// Set USER_AUTH_TEST_DATABASE_URL to run this read-only login-token/session
// integration regression against a migrated database with an active user.
func TestLoginTokenPassesProtectedCustomerSession(t *testing.T) {
	dsn := os.Getenv("USER_AUTH_TEST_DATABASE_URL")
	secret := os.Getenv("JWT_SECRET")
	if dsn == "" || secret == "" {
		t.Skip("set USER_AUTH_TEST_DATABASE_URL and JWT_SECRET to run the customer session integration test")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(db.Close)

	var id int64
	var email string
	if err := db.QueryRow(context.Background(), `SELECT id,email FROM users WHERE status='active' AND deleted_at IS NULL ORDER BY id LIMIT 1`).Scan(&id, &email); err != nil {
		t.Fatalf("find active customer for read-only session test: %v", err)
	}
	token, err := createUserToken(id, email)
	if err != nil || token == "" {
		t.Fatalf("create customer login token: %v", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/api/v1/user/me", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	protected := middleware.UserAuth(middleware.ActiveCustomerAccount(db)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Context().Value(middleware.UserIDKey); got != id {
			t.Errorf("authenticated user ID = %v, want %d", got, id)
		}
		w.WriteHeader(http.StatusNoContent)
	})))
	protected.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		var response map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		t.Fatalf("protected customer session returned %d: %v", w.Code, response["error"])
	}
}

func TestAdminPasswordResetPolicy(t *testing.T) {
	if !adminPasswordMeetsPolicy("StrongPass9!", 10, true, true, true, true) {
		t.Fatal("expected policy-compliant password")
	}
	for _, password := range []string{"short", "lowercase123!", "NoNumber!!", "NoSpecial123"} {
		if adminPasswordMeetsPolicy(password, 10, true, true, true, true) {
			t.Errorf("accepted password that violates policy: %q", password)
		}
	}
}
