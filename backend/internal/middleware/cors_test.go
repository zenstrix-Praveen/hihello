package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSPreflightAllowsLocalFrontendWithoutAuth(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("FRONTEND_URL", "")
	called := false
	handler := CORS(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/user/me", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || called {
		t.Fatalf("preflight must return 204 before auth/mux, got status=%d next_called=%v", res.Code, called)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Fatalf("unexpected allowed origin: %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Headers"); got != "Authorization, Content-Type" {
		t.Fatalf("unexpected allowed headers: %q", got)
	}
}

func TestCORSAllowsConfiguredDevelopmentLANFrontend(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("FRONTEND_URL", "http://localhost:3000")
	t.Setenv("DEV_FRONTEND_URLS", "http://192.168.29.31:3000, http://10.0.0.12:3000/")
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, origin := range []string{"http://192.168.29.31:3000", "http://10.0.0.12:3000"} {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/user/dashboard", nil)
		req.Header.Set("Origin", origin)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if got := res.Header().Get("Access-Control-Allow-Origin"); got != origin {
			t.Errorf("development origin %q: got %q", origin, got)
		}
	}
}

func TestCORSIgnoresDevelopmentOriginsInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FRONTEND_URL", "https://cards.example.test")
	t.Setenv("DEV_FRONTEND_URLS", "http://192.168.29.31:3000")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://192.168.29.31:3000")
	res := httptest.NewRecorder()
	CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(res, req)
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("production must ignore development allowlist, got %q", got)
	}
}

func TestCORSPermitsConfiguredOriginOnly(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("FRONTEND_URL", "https://cards.example.test")
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, tc := range []struct{ origin, want string }{{"https://cards.example.test", "https://cards.example.test"}, {"http://localhost:3000", ""}} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Origin", tc.origin)
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if got := res.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("origin %q: got %q, want %q", tc.origin, got, tc.want)
		}
		if got := res.Header().Get("Access-Control-Allow-Credentials"); got != "" {
			t.Errorf("credentials must not be enabled for bearer-token CORS, got %q", got)
		}
	}
}

func TestCORSDoesNotReflectUnknownOrigin(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://untrusted.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("must not allow arbitrary origin, got %q", got)
	}
}
