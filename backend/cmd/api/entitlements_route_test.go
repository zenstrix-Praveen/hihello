package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"digital-card-platform/backend/internal/customer"
	appmiddleware "digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func TestEntitlementsRouteRequiresAuthentication(t *testing.T) {
	t.Setenv("JWT_SECRET", "CORE002-route-test-signing-secret-long-enough")
	router := chi.NewRouter()
	router.Route("/api/v1/user", func(r chi.Router) {
		r.Use(appmiddleware.UserAuth)
		registerEntitlementsRoute(r, &customer.Handler{})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/me/entitlements", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthenticated entitlement request to return 401, got %d: %s", response.Code, response.Body.String())
	}
}
