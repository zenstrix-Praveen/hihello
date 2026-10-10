package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userIDContextKey string

const UserIDKey userIDContextKey = "user_id"

type UserClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// ActiveCustomerAccount blocks deleted customer accounts even while an
// otherwise valid stateless JWT has not expired yet.
func ActiveCustomerAccount(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := r.Context().Value(UserIDKey).(int64)
			if !ok || id < 1 {
				userUnauthorized(w)
				return
			}
			var active bool
			if err := db.QueryRow(r.Context(), `SELECT deleted_at IS NULL AND status='active' FROM users WHERE id=$1`, id).Scan(&active); err != nil || !active {
				userUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserAuth accepts only a signed end-user JWT carrying a positive user_id.
// Admin tokens have admin_id only and cannot be used for consumer billing.
func UserAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		// Match customer token issuance, including configuration whitespace.
		secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if len(parts) != 2 || parts[0] != "Bearer" || secret == "" {
			userUnauthorized(w)
			return
		}
		claims := &UserClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
		if err != nil || token == nil || !token.Valid || claims.UserID < 1 {
			userUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "A valid user session is required"})
}
