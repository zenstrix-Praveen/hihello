package middleware

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const (
	AdminIDKey   contextKey = "admin_id"
	AdminRoleKey contextKey = "admin_role"
)

type AdminClaims struct {
	AdminID int64  `json:"admin_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			unauthorized(w, "Authorization token is required")
			return
		}
		claims := &AdminClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) { return []byte(os.Getenv("JWT_SECRET")), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || token == nil || !token.Valid || claims.AdminID < 1 || strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
			unauthorized(w, "Invalid or expired token")
			return
		}
		ctx := context.WithValue(r.Context(), AdminIDKey, claims.AdminID)
		ctx = context.WithValue(ctx, AdminRoleKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(401)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
