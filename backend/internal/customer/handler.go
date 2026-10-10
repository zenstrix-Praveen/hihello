package customer

import (
	"digital-card-platform/backend/internal/middleware"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct{ DB *pgxpool.Pool }

func respond(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// Active checks the current database status as well as the existing JWT middleware.
func (h *Handler) Active(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := r.Context().Value(middleware.UserIDKey).(int64)
		if !ok || id < 1 {
			respond(w, 401, map[string]string{"error": "A valid user session is required"})
			return
		}
		var status string
		var verified bool
		err := h.DB.QueryRow(r.Context(), `SELECT status,email_verified_at IS NOT NULL FROM users WHERE id=$1`, id).Scan(&status, &verified)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (status != "active" || !verified)) {
			respond(w, 401, map[string]string{"error": "User account is not active"})
			return
		}
		if err != nil {
			respond(w, 500, map[string]string{"error": "Could not validate user account"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	var data json.RawMessage
	err := h.DB.QueryRow(r.Context(), `SELECT json_build_object('id',id,'public_id',public_id,'name',name,'email',email,'phone',phone,'status',status,'created_at',created_at) FROM users WHERE id=$1`, r.Context().Value(middleware.UserIDKey)).Scan(&data)
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not load profile"})
		return
	}
	respond(w, 200, data)
}

func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || userID < 1 {
		respond(w, 401, map[string]string{"error": "A valid user session is required"})
		return
	}
	var in struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
		respond(w, 400, map[string]string{"error": "Invalid profile request"})
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Phone = strings.TrimSpace(in.Phone)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 150 || len(in.Phone) > 50 {
		respond(w, 400, map[string]string{"error": "Enter a name up to 150 characters and a phone up to 50 characters"})
		return
	}
	var data json.RawMessage
	err := h.DB.QueryRow(r.Context(), `UPDATE users SET name=$1,phone=NULLIF($2,''),updated_at=NOW() WHERE id=$3 RETURNING json_build_object('id',id,'public_id',public_id,'name',name,'email',email,'phone',phone,'status',status,'created_at',created_at)`, in.Name, in.Phone, userID).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		respond(w, 404, map[string]string{"error": "User not found"})
		return
	}
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not update profile"})
		return
	}
	respond(w, 200, data)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || userID < 1 {
		respond(w, 401, map[string]string{"error": "A valid user session is required"})
		return
	}
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || in.CurrentPassword == "" {
		respond(w, 400, map[string]string{"error": "Current and new passwords are required"})
		return
	}
	if utf8.RuneCountInString(in.NewPassword) < 8 || len(in.NewPassword) > 72 {
		respond(w, 400, map[string]string{"error": "New password must be at least 8 characters and no more than 72 bytes"})
		return
	}
	var currentHash string
	err := h.DB.QueryRow(r.Context(), `SELECT COALESCE(password_hash,'') FROM users WHERE id=$1`, userID).Scan(&currentHash)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && currentHash == "") {
		respond(w, 400, map[string]string{"error": "Password change is unavailable for this account"})
		return
	}
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not verify current password"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(in.CurrentPassword)) != nil {
		respond(w, 400, map[string]string{"error": "Current password is incorrect"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not secure new password"})
		return
	}
	if _, err = h.DB.Exec(r.Context(), `UPDATE users SET password_hash=$1,updated_at=NOW() WHERE id=$2`, string(hash), userID); err != nil {
		respond(w, 500, map[string]string{"error": "Could not update password"})
		return
	}
	respond(w, 200, map[string]string{"message": "Password updated successfully"})
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	var data json.RawMessage
	err := h.DB.QueryRow(r.Context(), `SELECT json_build_object(
 'total_cards',(SELECT COUNT(*) FROM digital_cards WHERE user_id=$1),
 'published_cards',(SELECT COUNT(*) FROM digital_cards WHERE user_id=$1 AND status='published'),
 'draft_cards',(SELECT COUNT(*) FROM digital_cards WHERE user_id=$1 AND status='draft'),
	 'total_qr_scans',(SELECT COALESCE(SUM(q.scan_count),0) FROM qr_codes q JOIN digital_cards c ON c.id=q.card_id WHERE c.user_id=$1),
	 'total_nfc_taps',(SELECT COALESCE(SUM(n.tap_count),0) FROM nfc_tags n JOIN digital_cards c ON c.id=n.card_id WHERE c.user_id=$1),
 'total_contacts',(SELECT COUNT(*) FROM contacts WHERE user_id=$1),
 'current_plan',(SELECT json_build_object('plan_id',s.plan_id,'name',COALESCE(p.name,'Unavailable plan'),'status',s.status,'amount',s.amount,'currency',s.currency,'billing_period',s.billing_period,'started_at',s.started_at,'renews_at',s.renews_at,'ends_at',s.ends_at,'features',p.features,'limits',p.limits)
 FROM subscriptions s LEFT JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=$1 AND s.status IN ('active','trial','trialing','past_due') AND (s.ends_at IS NULL OR s.ends_at>NOW()) ORDER BY s.created_at DESC,s.id DESC LIMIT 1))`, r.Context().Value(middleware.UserIDKey)).Scan(&data)
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not load dashboard"})
		return
	}
	respond(w, 200, data)
}

// Analytics returns event totals only for cards owned by the current user.
func (h *Handler) Analytics(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(int64)
	if !ok || userID < 1 {
		respond(w, 401, map[string]string{"error": "A valid user session is required"})
		return
	}
	var cardID int64
	if raw := strings.TrimSpace(r.URL.Query().Get("card_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			respond(w, 400, map[string]string{"error": "Invalid card filter"})
			return
		}
		cardID = parsed
	}
	var data json.RawMessage
	err := h.DB.QueryRow(r.Context(), `SELECT json_build_object(
		'card_views',COUNT(*) FILTER (WHERE e.event_type='card_view'),
		'qr_scans',COUNT(*) FILTER (WHERE e.event_type='qr_scan'),
		'nfc_taps',COUNT(*) FILTER (WHERE e.event_type='nfc_tap'),
		'contact_exchanges',COUNT(*) FILTER (WHERE e.event_type='contact_exchange'),
		'link_clicks',COUNT(*) FILTER (WHERE e.event_type='link_click'))
		FROM analytics_events e JOIN digital_cards c ON c.id=e.card_id
		WHERE c.user_id=$1 AND ($2=0 OR c.id=$2)`, userID, cardID).Scan(&data)
	if err != nil {
		respond(w, 500, map[string]string{"error": "Could not load analytics"})
		return
	}
	respond(w, 200, data)
}

// Resource lists use fixed queries; no table or owner identifier comes from the client.
func (h *Handler) Resources(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := ""
		switch kind {
		case "contacts":
			query = `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT id,name,email,phone,source,created_at FROM contacts WHERE user_id=$1 ORDER BY created_at DESC LIMIT 200) x`
		case "qr-codes":
			query = `SELECT COALESCE(json_agg(x),'[]'::json) FROM (SELECT q.id,q.code_value,q.status,c.name AS card_name,c.slug AS card_slug,q.scan_count,q.created_at FROM qr_codes q JOIN digital_cards c ON c.id=q.card_id WHERE c.user_id=$1 ORDER BY q.created_at DESC LIMIT 200) x`
		default:
			respond(w, 404, map[string]string{"error": "Not found"})
			return
		}
		var data json.RawMessage
		if err := h.DB.QueryRow(r.Context(), query, r.Context().Value(middleware.UserIDKey)).Scan(&data); err != nil {
			respond(w, 500, map[string]string{"error": "Could not load records"})
			return
		}
		respond(w, 200, map[string]any{"data": data})
	}
}
