package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	appmiddleware "digital-card-platform/backend/internal/middleware"
	"digital-card-platform/backend/internal/payments"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type SecuritySettingsHandler struct{ DB *pgxpool.Pool }

var allowedAdminRoles = map[string]bool{"super_admin": true, "admin": true, "support": true, "billing": true, "content_manager": true, "read_only": true}
var countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)

type SecurityPolicy struct {
	PasswordMinLength      int  `json:"password_min_length"`
	RequireUppercase       bool `json:"require_uppercase"`
	RequireLowercase       bool `json:"require_lowercase"`
	RequireNumber          bool `json:"require_number"`
	RequireSpecial         bool `json:"require_special"`
	PasswordExpiryDays     *int `json:"password_expiry_days"`
	SessionTimeoutMinutes  int  `json:"session_timeout_minutes"`
	MaxFailedLoginAttempts int  `json:"max_failed_login_attempts"`
	LockoutDurationMinutes int  `json:"lockout_duration_minutes"`
}

func adminActor(r *http.Request) *int64 {
	if id, ok := r.Context().Value(appmiddleware.AdminIDKey).(int64); ok && id > 0 {
		return &id
	}
	return nil
}
func auditAdmin(r *http.Request, db *pgxpool.Pool, action, entity string, entityID interface{}, description string) {
	actor := adminActor(r)
	var id interface{}
	if entityID != nil {
		id = fmt.Sprint(entityID)
	}
	if actor == nil && entity == "admin_user" && entityID != nil {
		if n, err := strconv.ParseInt(fmt.Sprint(entityID), 10, 64); err == nil {
			actor = &n
		}
	}
	ip := r.RemoteAddr
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	_, _ = db.Exec(r.Context(), `INSERT INTO admin_security_activity(admin_user_id,event_type,action,entity_type,entity_id,description,ip_address,user_agent,metadata) VALUES($1,$2,$2,$3,$4,$5,NULLIF($6,'')::inet,$7,'{}'::jsonb)`, actor, action, entity, id, description, ip, r.UserAgent())
}

func (h *SecuritySettingsHandler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	search := "%" + strings.TrimSpace(r.URL.Query().Get("search")) + "%"
	rows, e := h.DB.Query(r.Context(), `SELECT id,public_id::text,name,email,role,status,last_login_at,created_at,updated_at FROM admin_users WHERE ($1='%%' OR name ILIKE $1 OR email ILIKE $1) ORDER BY created_at DESC`, search)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load admin users"})
		return
	}
	defer rows.Close()
	out := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var public, name, email, role, status string
		var last *time.Time
		var created, updated time.Time
		if rows.Scan(&id, &public, &name, &email, &role, &status, &last, &created, &updated) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read admin users"})
			return
		}
		out = append(out, map[string]interface{}{"id": id, "public_id": public, "name": name, "email": email, "role": role, "status": status, "last_login_at": last, "created_at": created, "updated_at": updated})
	}
	writeJSON(w, 200, map[string]interface{}{"data": out})
}
func (h *SecuritySettingsHandler) AdminUser(w http.ResponseWriter, r *http.Request) {
	id, e := securityRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin user id"})
		return
	}
	var public, name, email, role, status string
	var last *time.Time
	var created, updated time.Time
	e = h.DB.QueryRow(r.Context(), `SELECT public_id::text,name,email,role,status,last_login_at,created_at,updated_at FROM admin_users WHERE id=$1`, id).Scan(&public, &name, &email, &role, &status, &last, &created, &updated)
	if e != nil {
		if e == pgx.ErrNoRows {
			writeJSON(w, 404, map[string]string{"error": "Admin user not found"})
		} else {
			writeJSON(w, 500, map[string]string{"error": "Could not load admin user"})
		}
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]interface{}{"id": id, "public_id": public, "name": name, "email": email, "role": role, "status": status, "last_login_at": last, "created_at": created, "updated_at": updated}})
}
func (h *SecuritySettingsHandler) UpdateAdminUser(w http.ResponseWriter, r *http.Request) {
	id, e := securityRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin user id"})
		return
	}
	var in struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin user data"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	if in.Name == "" || len(in.Name) > 150 || !allowedAdminRoles[in.Role] {
		writeJSON(w, 400, map[string]string{"error": "Name and a supported role are required"})
		return
	}
	if _, e = mail.ParseAddress(in.Email); e != nil {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid email address"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE admin_users SET name=$1,email=$2,role=$3,role_id=(SELECT id FROM roles WHERE code=$3),updated_at=NOW() WHERE id=$4`, in.Name, in.Email, in.Role, id)
	if e != nil {
		if pe, ok := e.(*pgconn.PgError); ok && pe.Code == "23505" {
			writeJSON(w, 409, map[string]string{"error": "An admin user already uses this email"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "Could not update admin user"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Admin user not found"})
		return
	}
	auditAdmin(r, h.DB, "admin_user_updated", "admin_user", id, "Updated admin user profile or role")
	h.AdminUser(w, r)
}
func (h *SecuritySettingsHandler) AdminUserStatus(w http.ResponseWriter, r *http.Request) {
	id, e := securityRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin user id"})
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || !map[string]bool{"active": true, "inactive": true, "suspended": true}[in.Status] {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin status"})
		return
	}
	if actor := adminActor(r); actor != nil && *actor == id && in.Status != "active" {
		writeJSON(w, 400, map[string]string{"error": "You cannot disable your own account"})
		return
	}
	var isSuper bool
	e = h.DB.QueryRow(r.Context(), `SELECT role='super_admin' AND status='active' FROM admin_users WHERE id=$1`, id).Scan(&isSuper)
	if e != nil {
		writeJSON(w, 404, map[string]string{"error": "Admin user not found"})
		return
	}
	if isSuper && in.Status != "active" {
		var count int
		e = h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM admin_users WHERE role='super_admin' AND status='active'`).Scan(&count)
		if e != nil || count < 2 {
			writeJSON(w, 400, map[string]string{"error": "At least one active super administrator must remain"})
			return
		}
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE admin_users SET status=$1,updated_at=NOW() WHERE id=$2`, in.Status, id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update admin status"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Admin user not found"})
		return
	}
	auditAdmin(r, h.DB, "admin_user_status_changed", "admin_user", id, "Admin status changed to "+in.Status)
	h.AdminUser(w, r)
}
func validatePassword(p string, s SecurityPolicy) error {
	if len(p) > 72 {
		return errors.New("Password cannot exceed 72 bytes")
	}
	if len([]rune(p)) < s.PasswordMinLength {
		return fmt.Errorf("Password must contain at least %d characters", s.PasswordMinLength)
	}
	var up, low, num, special bool
	for _, c := range p {
		switch {
		case c >= 'A' && c <= 'Z':
			up = true
		case c >= 'a' && c <= 'z':
			low = true
		case c >= '0' && c <= '9':
			num = true
		default:
			special = true
		}
	}
	if s.RequireUppercase && !up {
		return errors.New("Password must include an uppercase letter")
	}
	if s.RequireLowercase && !low {
		return errors.New("Password must include a lowercase letter")
	}
	if s.RequireNumber && !num {
		return errors.New("Password must include a number")
	}
	if s.RequireSpecial && !special {
		return errors.New("Password must include a special character")
	}
	return nil
}
func (h *SecuritySettingsHandler) ResetAdminPassword(w http.ResponseWriter, r *http.Request) {
	id, e := securityRouteID(r)
	if e != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid admin user id"})
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "Password is required"})
		return
	}
	policy, e := h.securityPolicy(r)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load password policy"})
		return
	}
	if e = validatePassword(in.Password, policy); e != nil {
		writeJSON(w, 400, map[string]string{"error": e.Error()})
		return
	}
	hash, e := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not reset password"})
		return
	}
	tag, e := h.DB.Exec(r.Context(), `UPDATE admin_users SET password_hash=$1,password_changed_at=NOW(),failed_login_attempts=0,locked_until=NULL,updated_at=NOW() WHERE id=$2`, string(hash), id)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not reset password"})
		return
	}
	if tag.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "Admin user not found"})
		return
	}
	auditAdmin(r, h.DB, "admin_password_reset", "admin_user", id, "Administrator password was reset")
	writeJSON(w, 200, map[string]string{"message": "Password reset successfully"})
}

func (h *SecuritySettingsHandler) securityPolicy(r *http.Request) (SecurityPolicy, error) {
	var p SecurityPolicy
	e := h.DB.QueryRow(r.Context(), `SELECT password_min_length,require_uppercase,require_lowercase,require_number,require_special,password_expiry_days,session_timeout_minutes,max_failed_login_attempts,lockout_duration_minutes FROM admin_security_settings WHERE id=1`).Scan(&p.PasswordMinLength, &p.RequireUppercase, &p.RequireLowercase, &p.RequireNumber, &p.RequireSpecial, &p.PasswordExpiryDays, &p.SessionTimeoutMinutes, &p.MaxFailedLoginAttempts, &p.LockoutDurationMinutes)
	return p, e
}
func (h *SecuritySettingsHandler) SecuritySettings(w http.ResponseWriter, r *http.Request) {
	p, e := h.securityPolicy(r)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load security settings"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": p})
}
func (h *SecuritySettingsHandler) SaveSecuritySettings(w http.ResponseWriter, r *http.Request) {
	var p SecurityPolicy
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid security settings"})
		return
	}
	if p.PasswordMinLength < 8 || p.PasswordMinLength > 128 || p.SessionTimeoutMinutes < 5 || p.SessionTimeoutMinutes > 43200 || p.MaxFailedLoginAttempts < 1 || p.MaxFailedLoginAttempts > 100 || p.LockoutDurationMinutes < 1 || p.LockoutDurationMinutes > 10080 || p.PasswordExpiryDays != nil && (*p.PasswordExpiryDays < 1 || *p.PasswordExpiryDays > 3650) {
		writeJSON(w, 400, map[string]string{"error": "One or more security settings are outside the supported range"})
		return
	}
	_, e := h.DB.Exec(r.Context(), `UPDATE admin_security_settings SET password_min_length=$1,require_uppercase=$2,require_lowercase=$3,require_number=$4,require_special=$5,password_expiry_days=$6,session_timeout_minutes=$7,max_failed_login_attempts=$8,lockout_duration_minutes=$9,updated_at=NOW() WHERE id=1`, p.PasswordMinLength, p.RequireUppercase, p.RequireLowercase, p.RequireNumber, p.RequireSpecial, p.PasswordExpiryDays, p.SessionTimeoutMinutes, p.MaxFailedLoginAttempts, p.LockoutDurationMinutes)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save security settings"})
		return
	}
	auditAdmin(r, h.DB, "security_settings_updated", "security_settings", 1, "Updated password and session security settings")
	h.SecuritySettings(w, r)
}
func (h *SecuritySettingsHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && v > 0 {
		if v > 500 {
			v = 500
		}
		limit = v
	}
	rows, e := h.DB.Query(r.Context(), `SELECT a.id,a.admin_user_id,COALESCE(u.name,''),COALESCE(a.action,a.event_type),COALESCE(a.entity_type,''),COALESCE(a.entity_id,''),a.description,COALESCE(a.ip_address::text,''),COALESCE(a.user_agent,''),a.created_at FROM admin_security_activity a LEFT JOIN admin_users u ON u.id=a.admin_user_id ORDER BY a.created_at DESC LIMIT $1`, limit)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load audit log"})
		return
	}
	defer rows.Close()
	items := []map[string]interface{}{}
	for rows.Next() {
		var id int64
		var adminID *int64
		var name, action, entity, entityID, desc, ip, ua string
		var created time.Time
		if rows.Scan(&id, &adminID, &name, &action, &entity, &entityID, &desc, &ip, &ua, &created) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read audit log"})
			return
		}
		items = append(items, map[string]interface{}{"id": id, "admin_user_id": adminID, "admin_name": name, "action": action, "entity_type": entity, "entity_id": entityID, "description": desc, "ip_address": ip, "user_agent": ua, "created_at": created})
	}
	writeJSON(w, 200, map[string]interface{}{"data": items})
}

type PlatformSettingsHandler struct{ DB *pgxpool.Pool }
type PlatformSettings struct {
	PlatformName        string   `json:"platform_name"`
	SupportEmail        string   `json:"support_email"`
	SupportPhone        string   `json:"support_phone"`
	DefaultCurrency     string   `json:"default_currency"`
	Timezone            string   `json:"timezone"`
	DateFormat          string   `json:"date_format"`
	TimeFormat          string   `json:"time_format"`
	DefaultCountryCode  string   `json:"default_country_code"`
	DefaultCountryName  string   `json:"default_country_name"`
	DefaultStateCode    string   `json:"default_state_code"`
	DefaultStateName    string   `json:"default_state_name"`
	DefaultCity         string   `json:"default_city"`
	PostalCode          string   `json:"postal_code"`
	DefaultLanguage     string   `json:"default_language"`
	Locale              string   `json:"locale"`
	EnabledLanguages    []string `json:"enabled_languages"`
	LogoURL             string   `json:"logo_url"`
	FaviconURL          string   `json:"favicon_url"`
	CompanyName         string   `json:"company_name"`
	FrontendURL         string   `json:"frontend_url"`
	PublicBaseURL       string   `json:"public_base_url"`
	SupportURL          string   `json:"support_url"`
	PrivacyPolicyURL    string   `json:"privacy_policy_url"`
	TermsURL            string   `json:"terms_url"`
	MaintenanceMode     bool     `json:"maintenance_mode"`
	RegistrationEnabled bool     `json:"registration_enabled"`
	DefaultUserStatus   string   `json:"default_user_status"`
	DefaultCardStatus   string   `json:"default_card_status"`
	DefaultTemplateID   *int64   `json:"default_template_id"`
}

func defaultPlatformSettings() PlatformSettings {
	return PlatformSettings{PlatformName: "Digital Card Platform", DefaultCurrency: "USD", Timezone: "UTC", DateFormat: "YYYY-MM-DD", TimeFormat: "24h", DefaultCountryCode: "US", DefaultCountryName: "United States", DefaultLanguage: "en", Locale: "en-US", EnabledLanguages: []string{"en"}, RegistrationEnabled: true, DefaultUserStatus: "active", DefaultCardStatus: "draft"}
}
func settingsMap(p PlatformSettings) map[string]interface{} {
	b, _ := json.Marshal(p)
	m := map[string]interface{}{}
	_ = json.Unmarshal(b, &m)
	return m
}
func (h *PlatformSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	p := defaultPlatformSettings()
	rows, e := h.DB.Query(r.Context(), `SELECT setting_key,setting_value FROM platform_settings`)
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load platform settings"})
		return
	}
	defer rows.Close()
	m := settingsMap(p)
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read platform settings"})
			return
		}
		if _, ok := m[k]; !ok {
			continue
		}
		var decoded interface{}
		if json.Unmarshal([]byte(v), &decoded) == nil {
			m[k] = decoded
		} else {
			m[k] = v
		}
	}
	// Payment credentials are read only from the backend environment. Never
	// serialize the secret values into an admin settings response.
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RAZORPAY_MODE")))
	if mode != "live" {
		mode = "test"
	}
	m["payment_provider"] = "razorpay"
	m["razorpay_mode"] = mode
	m["razorpay_enabled"] = payments.ValidateConfig() == nil
	m["razorpay_key_id_configured"] = strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")) != ""
	m["razorpay_key_secret_configured"] = strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET")) != ""
	m["razorpay_webhook_secret_configured"] = strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET")) != ""
	m["razorpay_supported_currencies"] = payments.SupportedCurrencies("razorpay")
	m["razorpay_currency_configuration_source"] = payments.CurrencyConfigurationSource("razorpay")
	m["razorpay_merchant_currencies_verified"] = false
	var configuredRaw, verifiedRaw string
	if h.DB.QueryRow(r.Context(), `SELECT setting_value FROM platform_settings WHERE setting_key='razorpay_supported_currencies'`).Scan(&configuredRaw) == nil {
		var configured []string
		var verified bool
		_ = json.Unmarshal([]byte(configuredRaw), &configured)
		if h.DB.QueryRow(r.Context(), `SELECT setting_value FROM platform_settings WHERE setting_key='razorpay_merchant_currencies_verified'`).Scan(&verifiedRaw) == nil {
			_ = json.Unmarshal([]byte(verifiedRaw), &verified)
		}
		if verified {
			m["razorpay_supported_currencies"] = configured
			m["razorpay_currency_configuration_source"] = "Admin Payment Settings (merchant currency confirmation recorded)"
			m["razorpay_merchant_currencies_verified"] = true
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": m})
}
func validOptionalURL(s string) bool {
	if s == "" {
		return true
	}
	u, e := url.ParseRequestURI(s)
	return e == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
func (h *PlatformSettingsHandler) Save(w http.ResponseWriter, r *http.Request) {
	var p PlatformSettings
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid platform settings"})
		return
	}
	p.PlatformName = strings.TrimSpace(p.PlatformName)
	p.DefaultCurrency = strings.ToUpper(strings.TrimSpace(p.DefaultCurrency))
	p.DefaultCountryCode = strings.ToUpper(strings.TrimSpace(p.DefaultCountryCode))
	if p.PlatformName == "" || len(p.DefaultCurrency) != 3 || !asciiLetters(p.DefaultCurrency) || !countryCodePattern.MatchString(p.DefaultCountryCode) {
		writeJSON(w, 400, map[string]string{"error": "Platform name, three-letter currency and ISO country code are required"})
		return
	}
	if p.SupportEmail != "" {
		if _, e := mail.ParseAddress(p.SupportEmail); e != nil {
			writeJSON(w, 400, map[string]string{"error": "Enter a valid support email"})
			return
		}
	}
	for _, u := range []string{p.LogoURL, p.FaviconURL, p.FrontendURL, p.PublicBaseURL, p.SupportURL, p.PrivacyPolicyURL, p.TermsURL} {
		if !validOptionalURL(u) {
			writeJSON(w, 400, map[string]string{"error": "URLs must include http:// or https://"})
			return
		}
	}
	if p.DefaultLanguage == "" || p.Locale == "" || p.Timezone == "" || len(p.EnabledLanguages) == 0 {
		writeJSON(w, 400, map[string]string{"error": "Default language, locale, timezone and at least one enabled language are required"})
		return
	}
	if _, e := time.LoadLocation(p.Timezone); e != nil {
		writeJSON(w, 400, map[string]string{"error": "Enter a valid IANA timezone, such as UTC or America/New_York"})
		return
	}
	included := false
	for _, v := range p.EnabledLanguages {
		if v == p.DefaultLanguage {
			included = true
		}
	}
	if !included {
		writeJSON(w, 400, map[string]string{"error": "Default language must be enabled"})
		return
	}
	if p.DefaultUserStatus != "active" && p.DefaultUserStatus != "pending" {
		writeJSON(w, 400, map[string]string{"error": "Invalid default user status"})
		return
	}
	if p.DefaultCardStatus != "draft" && p.DefaultCardStatus != "published" {
		writeJSON(w, 400, map[string]string{"error": "Invalid default card status"})
		return
	}
	if p.DefaultTemplateID != nil {
		var exists bool
		if e := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM website_templates WHERE id=$1)`, *p.DefaultTemplateID).Scan(&exists); e != nil || !exists {
			writeJSON(w, 400, map[string]string{"error": "Selected default template was not found"})
			return
		}
	}
	m := settingsMap(p)
	tx, e := h.DB.Begin(r.Context())
	if e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save platform settings"})
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	for key, value := range m {
		b, _ := json.Marshal(value)
		if _, e := tx.Exec(r.Context(), `INSERT INTO platform_settings(setting_key,setting_value,updated_at) VALUES($1,$2,NOW()) ON CONFLICT(setting_key) DO UPDATE SET setting_value=EXCLUDED.setting_value,updated_at=NOW()`, key, string(b)); e != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not save platform settings"})
			return
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not save platform settings"})
		return
	}
	auditAdmin(r, h.DB, "platform_settings_updated", "platform_settings", nil, "Updated platform settings")
	h.Get(w, r)
}

type option struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var countries = []option{{"US", "United States"}, {"CA", "Canada"}, {"GB", "United Kingdom"}, {"IN", "India"}, {"AU", "Australia"}, {"NZ", "New Zealand"}, {"SG", "Singapore"}, {"MY", "Malaysia"}, {"LK", "Sri Lanka"}, {"BD", "Bangladesh"}, {"PK", "Pakistan"}, {"AE", "United Arab Emirates"}, {"SA", "Saudi Arabia"}, {"ZA", "South Africa"}, {"NG", "Nigeria"}, {"KE", "Kenya"}, {"DE", "Germany"}, {"FR", "France"}, {"IT", "Italy"}, {"ES", "Spain"}, {"NL", "Netherlands"}, {"SE", "Sweden"}, {"NO", "Norway"}, {"BR", "Brazil"}, {"MX", "Mexico"}, {"JP", "Japan"}, {"KR", "South Korea"}, {"CN", "China"}, {"ID", "Indonesia"}, {"PH", "Philippines"}, {"TH", "Thailand"}, {"ZZ", "Other / enter manually"}}
var languages = []option{{"en", "English"}, {"ta", "Tamil"}, {"hi", "Hindi"}, {"es", "Spanish"}, {"fr", "French"}, {"de", "German"}, {"ar", "Arabic"}, {"zh", "Chinese"}, {"ja", "Japanese"}, {"pt", "Portuguese"}}

func (h *PlatformSettingsHandler) Countries(w http.ResponseWriter, r *http.Request) {
	writeOptionList(w, r, h.DB, "country_options", countries)
}
func (h *PlatformSettingsHandler) Languages(w http.ResponseWriter, r *http.Request) {
	writeOptionList(w, r, h.DB, "language_options", languages)
}
func writeOptionList(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, key string, fallback []option) {
	var raw string
	if db.QueryRow(r.Context(), `SELECT setting_value FROM platform_settings WHERE setting_key=$1`, key).Scan(&raw) == nil {
		var configured []option
		if json.Unmarshal([]byte(raw), &configured) == nil && configured != nil {
			writeJSON(w, 200, map[string]interface{}{"data": configured})
			return
		}
	}
	writeJSON(w, 200, map[string]interface{}{"data": fallback})
}
func (h *PlatformSettingsHandler) States(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("country")))
	if !countryCodePattern.MatchString(code) {
		writeJSON(w, 400, map[string]string{"error": "Use an ISO country code"})
		return
	}
	key := "state_options:" + code
	var raw string
	e := h.DB.QueryRow(r.Context(), `SELECT setting_value FROM platform_settings WHERE setting_key=$1`, key).Scan(&raw)
	if e == nil {
		var opts []option
		if json.Unmarshal([]byte(raw), &opts) != nil {
			writeJSON(w, 500, map[string]string{"error": "Invalid state option data"})
			return
		}
		writeJSON(w, 200, map[string]interface{}{"data": opts})
		return
	}
	defaults := map[string][]option{"US": {{"CA", "California"}, {"NY", "New York"}, {"TX", "Texas"}, {"FL", "Florida"}, {"WA", "Washington"}}, "IN": {{"TN", "Tamil Nadu"}, {"KA", "Karnataka"}, {"MH", "Maharashtra"}, {"DL", "Delhi"}, {"KL", "Kerala"}}, "CA": {{"ON", "Ontario"}, {"QC", "Quebec"}, {"BC", "British Columbia"}, {"AB", "Alberta"}}, "AU": {{"NSW", "New South Wales"}, {"VIC", "Victoria"}, {"QLD", "Queensland"}}}
	writeJSON(w, 200, map[string]interface{}{"data": defaults[code]})
}

func securityRouteID(r *http.Request) (int64, error) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil || id < 1 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}
