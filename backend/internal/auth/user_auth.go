package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type UserAuthHandler struct {
	DB  *pgxpool.Pool
	OTP *OTPService
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type userClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func createUserToken(id int64, email string) (string, error) {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		return "", nil
	}

	now := time.Now().UTC()

	claims := userClaims{
		UserID: id,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(secret))
}

func (h *UserAuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request",
		})
		return
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request"})
		return
	}

	name := strings.TrimSpace(req.Name)
	email := strings.ToLower(strings.TrimSpace(req.Email))
	phone := strings.TrimSpace(req.Phone)

	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Name is required",
		})
		return
	}

	if len(name) > 150 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Name is too long",
		})
		return
	}

	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email is required",
		})
		return
	}

	address, err := mail.ParseAddress(email)
	if err != nil || strings.ToLower(address.Address) != email {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Enter a valid email address",
		})
		return
	}

	if len(email) > 255 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email is too long",
		})
		return
	}

	if len(phone) > 50 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Phone number is too long",
		})
		return
	}

	if utf8.RuneCountInString(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Password must contain at least 8 characters",
		})
		return
	}

	if len(req.Password) > 72 {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Password must not exceed 72 bytes",
		})
		return
	}

	var exists bool
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "JWT configuration missing"})
		return
	}
	if h.OTP == nil || !h.OTP.EmailDeliveryConfigured() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Email verification delivery is not configured"})
		return
	}

	// Use the same defaults and persisted settings as the platform settings UI.
	registrationEnabled, defaultStatus := true, "active"
	rows, err := h.DB.Query(r.Context(), `SELECT setting_key, setting_value FROM platform_settings WHERE setting_key IN ('registration_enabled', 'default_user_status')`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load registration settings"})
		return
	}
	for rows.Next() {
		var key, value string
		if err = rows.Scan(&key, &value); err != nil {
			break
		}
		if key == "registration_enabled" {
			err = json.Unmarshal([]byte(value), &registrationEnabled)
		} else {
			if json.Unmarshal([]byte(value), &defaultStatus) != nil {
				defaultStatus = value
			}
		}
		if err != nil {
			break
		}
	}
	rows.Close()
	if err != nil || rows.Err() != nil || (defaultStatus != "active" && defaultStatus != "pending") {
		writeJSON(w, 500, map[string]string{"error": "Invalid registration settings"})
		return
	}
	if !registrationEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Account registration is currently disabled"})
		return
	}

	if err := h.DB.QueryRow(
		r.Context(),
		`SELECT EXISTS(
            SELECT 1
            FROM users
            WHERE LOWER(email) = $1
        )`,
		email,
	).Scan(&exists); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not validate email",
		})
		return
	}

	if exists {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "An account with this email already exists",
		})
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not secure password",
		})
		return
	}

	var (
		id        int64
		publicID  string
		status    string
		createdAt time.Time
	)

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create account"})
		return
	}
	defer tx.Rollback(r.Context())
	err = tx.QueryRow(
		r.Context(),
		`
        INSERT INTO users (
            name,
            email,
            phone,
            password_hash,
            status
        )
        VALUES ($1, $2, NULLIF($3, ''), $4, $5)
        RETURNING
            id,
            public_id::text,
            status,
            created_at
        `,
		name,
		email,
		phone,
		string(passwordHash),
		defaultStatus,
	).Scan(
		&id,
		&publicID,
		&status,
		&createdAt,
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "An account with this email already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not create account",
		})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create account"})
		return
	}
	if h.OTP == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "Email verification delivery is not configured"})
		return
	}
	if err := h.OTP.Issue("customer", id, email, "customer_registration"); err != nil {
		message := "Could not send verification email. Please try again."
		if errors.Is(err, ErrOTPThrottle) {
			message = "Please wait before requesting another code"
		}
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": message})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "Account created. Verify your email to continue.",
		"user": map[string]interface{}{
			"id":         id,
			"public_id":  publicID,
			"name":       name,
			"email":      email,
			"phone":      phone,
			"status":     status,
			"created_at": createdAt,
		},
	})
}

func (h *UserAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Invalid request",
		})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))

	if email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Email and password are required",
		})
		return
	}

	var (
		id           int64
		name         string
		storedEmail  string
		passwordHash string
		status       string
		verifiedAt   *time.Time
	)

	err := h.DB.QueryRow(
		r.Context(),
		`
        SELECT
            id,
            name,
            email,
            password_hash,
            status,
            email_verified_at
        FROM users
        WHERE LOWER(email) = $1 AND deleted_at IS NULL
        LIMIT 1
        `,
		email,
	).Scan(
		&id,
		&name,
		&storedEmail,
		&passwordHash,
		&status,
		&verifiedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid email or password",
		})
		return
	}

	if status != "active" {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "User account is not active",
		})
		return
	}
	if verifiedAt == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Verify your email before signing in"})
		return
	}

	if passwordHash == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Password login is not configured for this user",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(req.Password),
	); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "Invalid email or password",
		})
		return
	}

	tokenString, err := createUserToken(id, storedEmail)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Could not create user session",
		})
		return
	}

	if tokenString == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "JWT configuration missing",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":     id,
			"name":   name,
			"email":  storedEmail,
			"status": status,
		},
	})
}
