package middleware

import (
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionSeparation(t *testing.T) {
	t.Setenv("JWT_SECRET", "session-test-key")
	for _, tc := range []struct {
		name            string
		claims          jwt.MapClaims
		userOK, adminOK bool
	}{
		{"customer", jwt.MapClaims{"user_id": 42, "exp": time.Now().Add(time.Hour).Unix()}, true, false},
		{"administrator", jwt.MapClaims{"admin_id": 7, "role": "super_admin", "exp": time.Now().Add(time.Hour).Unix()}, false, true},
		{"expired customer", jwt.MapClaims{"user_id": 42, "exp": time.Now().Add(-time.Hour).Unix()}, false, false},
		{"missing user expiry", jwt.MapClaims{"user_id": 42}, false, false},
		{"empty claims", jwt.MapClaims{}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, tc.claims).SignedString([]byte("session-test-key"))
			if err != nil {
				t.Fatal(err)
			}
			for _, gate := range []struct {
				name string
				wrap func(http.Handler) http.Handler
				want bool
			}{{"user", UserAuth, tc.userOK}, {"admin", AdminAuth, tc.adminOK}} {
				r := httptest.NewRequest("GET", "/", nil)
				r.Header.Set("Authorization", "Bearer "+token)
				w := httptest.NewRecorder()
				gate.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
				want := 401
				if gate.want {
					want = 204
				}
				if w.Code != want {
					t.Errorf("%s gate got %d want %d", gate.name, w.Code, want)
				}
			}
		})
	}
}

func TestAdminAuthRejectsMissingToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "session-test-key")
	r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/website/home", nil)
	w := httptest.NewRecorder()
	AdminAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing admin token got %d, want 401", w.Code)
	}
}

func TestUserAuthRejectsInvalidTokens(t *testing.T) {
	t.Setenv("JWT_SECRET", "session-test-key")
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		key    string
	}{
		{"wrong signature", jwt.SigningMethodHS256, "different-key"},
		{"wrong algorithm", jwt.SigningMethodHS384, "session-test-key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, jwt.MapClaims{"user_id": 42, "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte(tc.key))
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/user/me", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			UserAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })).ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401", w.Code)
			}
		})
	}
}
