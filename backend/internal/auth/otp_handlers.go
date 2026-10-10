package auth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type otpRequest struct {
	Email       string `json:"email"`
	Code        string `json:"code"`
	Purpose     string `json:"purpose"`
	ResetToken  string `json:"reset_token"`
	NewPassword string `json:"new_password"`
}

func decodeOTP(w http.ResponseWriter, r *http.Request) (otpRequest, bool) {
	var in otpRequest
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(&in) != nil || d.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]string{"error": "Invalid request"})
		return in, false
	}
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	return in, true
}
func validEmail(s string) bool {
	a, e := mail.ParseAddress(s)
	return e == nil && strings.ToLower(a.Address) == s && len(s) <= 255
}
func safePurpose(p string, allowed ...string) bool {
	for _, v := range allowed {
		if p == v {
			return true
		}
	}
	return false
}

func (h *UserAuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) || len(in.Code) != 6 {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid email and 6-digit code"})
		return
	}
	if !safePurpose(in.Purpose, "customer_registration", "customer_password_reset") {
		writeJSON(w, 400, map[string]string{"error": "Invalid verification purpose"})
		return
	}
	if in.Purpose == "customer_registration" {
		id, _, err := h.OTP.Verify("customer", in.Email, in.Purpose, in.Code)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "The code is invalid or expired"})
			return
		}
		var status string
		if err = h.DB.QueryRow(r.Context(), `UPDATE users SET email_verified_at=COALESCE(email_verified_at,NOW()),updated_at=NOW() WHERE id=$1 RETURNING status`, id).Scan(&status); err != nil {
			writeJSON(w, 400, map[string]string{"error": "The code is invalid or expired"})
			return
		}
		if status != "active" {
			writeJSON(w, 200, map[string]string{"message": "Email verified. Your account is awaiting approval."})
			return
		}
		var email, name string
		_ = h.DB.QueryRow(r.Context(), `SELECT email,name FROM users WHERE id=$1`, id).Scan(&email, &name)
		token, err := createUserToken(id, email)
		if err != nil || token == "" {
			writeJSON(w, 500, map[string]string{"error": "Could not create user session"})
			return
		}
		writeJSON(w, 200, map[string]any{"token": token, "user": map[string]string{"email": email, "name": name, "status": status}})
		return
	}
	id, resetToken, err := h.OTP.Verify("customer", in.Email, in.Purpose, in.Code)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "The code is invalid or expired"})
		return
	}
	writeJSON(w, 200, map[string]string{"reset_token": resetToken, "message": "Code verified"})
}

func (h *UserAuthHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) || !safePurpose(in.Purpose, "customer_registration", "customer_password_reset") {
		writeJSON(w, 400, map[string]string{"error": "Invalid request"})
		return
	}
	var id int64
	var verified bool
	if in.Purpose == "customer_registration" {
		err := h.DB.QueryRow(r.Context(), `SELECT id,email_verified_at IS NOT NULL FROM users WHERE LOWER(email)=$1`, in.Email).Scan(&id, &verified)
		if err != nil || verified {
			writeJSON(w, 200, map[string]string{"message": "If the account can be verified, a new code has been sent."})
			return
		}
	} else {
		err := h.DB.QueryRow(r.Context(), `SELECT id FROM users WHERE LOWER(email)=$1 AND status='active'`, in.Email).Scan(&id)
		if err != nil {
			writeJSON(w, 200, map[string]string{"message": "If the account exists, a verification code has been sent."})
			return
		}
	}
	if h.OTP == nil {
		writeJSON(w, 503, map[string]string{"error": "Email delivery is temporarily unavailable"})
		return
	}
	if err := h.OTP.Issue("customer", id, in.Email, in.Purpose); err != nil {
		status := http.StatusServiceUnavailable
		message := "Could not send verification email. Please try again."
		if errors.Is(err, ErrOTPThrottle) {
			status = http.StatusTooManyRequests
			message = "Please wait before requesting another code"
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "A new verification code has been sent."})
}

func (h *UserAuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) {
		writeJSON(w, 200, map[string]string{"message": "If an account exists, a verification code has been sent."})
		return
	}
	var id int64
	var verified bool
	err := h.DB.QueryRow(r.Context(), `SELECT id,email_verified_at IS NOT NULL FROM users WHERE LOWER(email)=$1 AND status='active'`, in.Email).Scan(&id, &verified)
	if err == nil && verified && h.OTP != nil {
		_ = h.OTP.Issue("customer", id, in.Email, "customer_password_reset")
	}
	writeJSON(w, 200, map[string]string{"message": "If an account exists, a verification code has been sent."})
}

func (h *UserAuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) || utf8.RuneCountInString(in.NewPassword) < 8 || len(in.NewPassword) > 72 || strings.TrimSpace(in.ResetToken) == "" {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid verification and a password of at least 8 characters"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not secure password"})
		return
	}
	id, err := h.OTP.ConsumeResetToken("customer", in.Email, "customer_password_reset", in.ResetToken)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Verification is invalid or expired"})
		return
	}
	if _, err = h.DB.Exec(r.Context(), `UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2 AND LOWER(email)=$3`, string(hash), id, in.Email); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update password"})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Password reset successfully"})
}

// AdminOTPHandler uses the same OTP storage/engine but only reads and writes admin_users.
type AdminOTPHandler struct {
	DB  *pgxpool.Pool
	OTP *OTPService
}

func (h *AdminOTPHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if validEmail(in.Email) {
		var id int64
		err := h.DB.QueryRow(r.Context(), `SELECT id FROM admin_users WHERE LOWER(email)=$1 AND status='active'`, in.Email).Scan(&id)
		if err == nil {
			if h.OTP != nil {
				_ = h.OTP.Issue("admin", id, in.Email, "admin_password_reset")
			}
		}
	}
	writeJSON(w, 200, map[string]string{"message": "If an administrator account exists, a verification code has been sent."})
}
func (h *AdminOTPHandler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if validEmail(in.Email) {
		var id int64
		err := h.DB.QueryRow(r.Context(), `SELECT id FROM admin_users WHERE LOWER(email)=$1 AND status='active'`, in.Email).Scan(&id)
		if err == nil {
			if h.OTP == nil {
				writeJSON(w, 503, map[string]string{"error": "Email delivery is temporarily unavailable"})
				return
			}
			if e := h.OTP.Issue("admin", id, in.Email, "admin_password_reset"); e != nil {
				status := http.StatusServiceUnavailable
				message := "Could not send verification email. Please try again."
				if errors.Is(e, ErrOTPThrottle) {
					status = http.StatusTooManyRequests
					message = "Please wait before requesting another code"
				}
				writeJSON(w, status, map[string]string{"error": message})
				return
			}
		}
	}
	writeJSON(w, 200, map[string]string{"message": "If the administrator account exists, a verification code has been sent."})
}
func (h *AdminOTPHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) || len(in.Code) != 6 {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid email and 6-digit code"})
		return
	}
	id, token, err := h.OTP.Verify("admin", in.Email, "admin_password_reset", in.Code)
	if err != nil || id < 1 {
		writeJSON(w, 400, map[string]string{"error": "The code is invalid or expired"})
		return
	}
	writeJSON(w, 200, map[string]string{"reset_token": token, "message": "Code verified"})
}
func (h *AdminOTPHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeOTP(w, r)
	if !ok {
		return
	}
	if !validEmail(in.Email) || len(in.NewPassword) > 72 || strings.TrimSpace(in.ResetToken) == "" {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid verification and password"})
		return
	}
	var minLength int
	var upper, lower, number, special bool
	if err := h.DB.QueryRow(r.Context(), `SELECT password_min_length,require_uppercase,require_lowercase,require_number,require_special FROM admin_security_settings WHERE id=1`).Scan(&minLength, &upper, &lower, &number, &special); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Administrator security policy is unavailable"})
		return
	}
	if !adminPasswordMeetsPolicy(in.NewPassword, minLength, upper, lower, number, special) {
		writeJSON(w, 400, map[string]string{"error": "Password does not meet administrator security requirements"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not secure password"})
		return
	}
	id, err := h.OTP.ConsumeResetToken("admin", in.Email, "admin_password_reset", in.ResetToken)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Verification is invalid or expired"})
		return
	}
	_, err = h.DB.Exec(r.Context(), `UPDATE admin_users SET password_hash=$1,password_changed_at=NOW(),failed_login_attempts=0,locked_until=NULL,updated_at=NOW() WHERE id=$2 AND LOWER(email)=$3`, string(hash), id, in.Email)
	if err != nil { // password_changed_at is present in the security migration; keep the response generic.
		writeJSON(w, 500, map[string]string{"error": "Could not update administrator password"})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Administrator password reset successfully"})
}

func adminPasswordMeetsPolicy(password string, minLength int, requireUpper, requireLower, requireNumber, requireSpecial bool) bool {
	if minLength < 8 || utf8.RuneCountInString(password) < minLength || len(password) > 72 {
		return false
	}
	var upper, lower, number, special bool
	for _, r := range password {
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= '0' && r <= '9':
			number = true
		default:
			special = true
		}
	}
	return (!requireUpper || upper) && (!requireLower || lower) && (!requireNumber || number) && (!requireSpecial || special)
}
