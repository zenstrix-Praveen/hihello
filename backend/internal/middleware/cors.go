package middleware

import (
	"net/http"
	"os"
	"strings"
)

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		allowedOrigin := strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/")
		appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		development := appEnv == "local" || appEnv == "development" || appEnv == ""
		if allowedOrigin == "" && development {
			allowedOrigin = "http://localhost:3000"
		}
		allowed := origin != "" && origin == allowedOrigin
		// Mobile devices used for local LAN testing have a different browser
		// origin from localhost. Keep these extra origins opt-in and development
		// only; production continues to accept only FRONTEND_URL.
		if !allowed && development && origin != "" {
			for _, candidate := range strings.Split(os.Getenv("DEV_FRONTEND_URLS"), ",") {
				if origin == strings.TrimRight(strings.TrimSpace(candidate), "/") {
					allowed = true
					break
				}
			}
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PUT, PATCH, DELETE, OPTIONS",
		)

		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Authorization, Content-Type",
		)

		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
