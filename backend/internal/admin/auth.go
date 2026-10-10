package admin

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	appmiddleware "digital-card-platform/backend/internal/middleware"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{ DB *pgxpool.Pool }

// Me returns the authenticated administrator's current profile without exposing credentials.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := r.Context().Value(appmiddleware.AdminIDKey).(int64)
	if !ok || id < 1 {
		writeJSON(w, 401, map[string]string{"error": "Unauthorized"})
		return
	}
	var a adminPayload
	err := h.DB.QueryRow(r.Context(), `SELECT id,public_id::text,name,email,role,status FROM admin_users WHERE id=$1`, id).Scan(&a.ID, &a.PublicID, &a.Name, &a.Email, &a.Role, &a.Status)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "Administrator not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": a})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type adminPayload struct {
	ID       int64  `json:"id"`
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}
type loginResponse struct {
	Token string       `json:"token"`
	Admin adminPayload `json:"admin"`
}
type Claims struct {
	AdminID int64  `json:"admin_id"`
	Role    string `json:"role"`
	jwt.RegisteredClaims
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		writeJSON(w, 400, map[string]string{"error": "Email and password are required"})
		return
	}
	var a adminPayload
	var hash string
	var failed int
	var lockedUntil, passwordChanged *time.Time
	err := h.DB.QueryRow(r.Context(), `SELECT id,public_id::text,name,email,password_hash,role,status,failed_login_attempts,locked_until,password_changed_at FROM admin_users WHERE email=$1 LIMIT 1`, strings.TrimSpace(strings.ToLower(req.Email))).Scan(&a.ID, &a.PublicID, &a.Name, &a.Email, &hash, &a.Role, &a.Status, &failed, &lockedUntil, &passwordChanged)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "Invalid email or password"})
		return
	}
	var policy SecurityPolicy
	err = h.DB.QueryRow(r.Context(), `SELECT password_min_length,require_uppercase,require_lowercase,require_number,require_special,password_expiry_days,session_timeout_minutes,max_failed_login_attempts,lockout_duration_minutes FROM admin_security_settings WHERE id=1`).Scan(&policy.PasswordMinLength, &policy.RequireUppercase, &policy.RequireLowercase, &policy.RequireNumber, &policy.RequireSpecial, &policy.PasswordExpiryDays, &policy.SessionTimeoutMinutes, &policy.MaxFailedLoginAttempts, &policy.LockoutDurationMinutes)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Security policy is unavailable"})
		return
	}
	if lockedUntil != nil && lockedUntil.After(time.Now()) {
		writeJSON(w, 429, map[string]string{"error": "Admin account is temporarily locked. Try again later."})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		_, _ = h.DB.Exec(r.Context(), `UPDATE admin_users SET failed_login_attempts=failed_login_attempts+1,locked_until=CASE WHEN failed_login_attempts+1 >= $1 THEN NOW()+($2 * INTERVAL '1 minute') ELSE locked_until END,updated_at=NOW() WHERE id=$3`, policy.MaxFailedLoginAttempts, policy.LockoutDurationMinutes, a.ID)
		auditAdmin(r, h.DB, "admin_login_failed", "admin_user", a.ID, "Failed administrator sign-in")
		writeJSON(w, 401, map[string]string{"error": "Invalid email or password"})
		return
	}
	if a.Status != "active" {
		writeJSON(w, 403, map[string]string{"error": "Admin account is not active"})
		return
	}
	if policy.PasswordExpiryDays != nil && passwordChanged != nil && passwordChanged.AddDate(0, 0, *policy.PasswordExpiryDays).Before(time.Now()) {
		auditAdmin(r, h.DB, "admin_password_expired", "admin_user", a.ID, "Sign-in blocked because password expired")
		writeJSON(w, 403, map[string]string{"error": "Password expired. Contact an administrator to reset it."})
		return
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		writeJSON(w, 500, map[string]string{"error": "JWT configuration missing"})
		return
	}
	now := time.Now()
	claims := Claims{AdminID: a.ID, Role: a.Role, RegisteredClaims: jwt.RegisteredClaims{Subject: a.PublicID, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(policy.SessionTimeoutMinutes) * time.Minute))}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create token"})
		return
	}
	_, _ = h.DB.Exec(r.Context(), `UPDATE admin_users SET last_login_at=NOW(),failed_login_attempts=0,locked_until=NULL, updated_at=NOW() WHERE id=$1`, a.ID)
	auditAdmin(r, h.DB, "admin_login", "admin_user", a.ID, "Administrator signed in")
	writeJSON(w, 200, loginResponse{Token: tokenString, Admin: a})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
