package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NFCHandler struct{ DB *pgxpool.Pool }

type nfcTag struct {
	ID             int64      `json:"id"`
	PublicID       string     `json:"public_id"`
	UID            string     `json:"uid"`
	Label          string     `json:"label"`
	CardID         *int64     `json:"card_id"`
	UserID         *int64     `json:"user_id"`
	OrganizationID *int64     `json:"organization_id"`
	RedirectToken  string     `json:"redirect_token"`
	RedirectURL    string     `json:"redirect_url"`
	Status         string     `json:"status"`
	TapCount       int64      `json:"tap_count"`
	LastTappedAt   *time.Time `json:"last_tapped_at"`
	AssignedAt     *time.Time `json:"assigned_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CardName       string     `json:"card_name"`
	CardSlug       string     `json:"card_slug"`
	OwnerName      string     `json:"owner_name"`
	OwnerEmail     string     `json:"owner_email"`
	Organization   string     `json:"organization"`
}

var nfcStatuses = map[string]bool{"unassigned": true, "active": true, "disabled": true, "lost": true, "archived": true}

func (h *NFCHandler) Summary(w http.ResponseWriter, r *http.Request) {
	var total, active, unassigned, disabled, taps, today int64
	err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='active'),COUNT(*) FILTER(WHERE status='unassigned'),COUNT(*) FILTER(WHERE status='disabled'),COALESCE(SUM(tap_count),0) FROM nfc_tags`).Scan(&total, &active, &unassigned, &disabled, &taps)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC summary"})
		return
	}
	err = h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM analytics_events WHERE event_type='nfc_tap' AND metadata ? 'nfc_tag_id' AND created_at>=CURRENT_DATE`).Scan(&today)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load today's taps"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]int64{"total_tags": total, "active": active, "unassigned": unassigned, "disabled": disabled, "total_taps": taps, "taps_today": today}})
}

func (h *NFCHandler) List(w http.ResponseWriter, r *http.Request) {
	page, limit := 1, 25
	if n, e := strconv.Atoi(r.URL.Query().Get("page")); e == nil && n > 0 {
		page = n
	}
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n > 0 {
		limit = n
	}
	if limit > 100 {
		limit = 100
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	cardID := strings.TrimSpace(r.URL.Query().Get("card_id"))
	like := "%" + search + "%"
	where := `($1='' OR n.status=$1) AND ($2='' OR n.user_id::text=$2) AND ($3='' OR n.card_id::text=$3) AND ($4='' OR n.uid ILIKE $5 OR n.label ILIKE $5 OR c.name ILIKE $5 OR u.name ILIKE $5 OR u.email ILIKE $5)`
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM nfc_tags n LEFT JOIN digital_cards c ON c.id=n.card_id LEFT JOIN users u ON u.id=n.user_id WHERE `+where, status, userID, cardID, search, like).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count NFC tags"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT n.id,n.public_id::text,n.uid,n.label,n.card_id,n.user_id,n.organization_id,n.redirect_token,n.status,n.tap_count,n.last_tapped_at,n.assigned_at,n.created_at,n.updated_at,COALESCE(c.name,''),COALESCE(c.slug,''),COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'') FROM nfc_tags n LEFT JOIN digital_cards c ON c.id=n.card_id LEFT JOIN users u ON u.id=n.user_id LEFT JOIN organizations o ON o.id=n.organization_id WHERE `+where+` ORDER BY n.created_at DESC,n.id DESC LIMIT $6 OFFSET $7`, status, userID, cardID, search, like, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC tags"})
		return
	}
	defer rows.Close()
	items := []nfcTag{}
	for rows.Next() {
		var t nfcTag
		if err := scanNFCTag(rows, &t); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read NFC tags"})
			return
		}
		t.RedirectURL = nfcURL(r, t.RedirectToken)
		items = append(items, t)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read NFC tags"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items, "pagination": map[string]int{"page": page, "limit": limit, "total": int(total), "pages": int((total + int64(limit) - 1) / int64(limit))}})
}

type rowScanner interface{ Scan(...interface{}) error }

func scanNFCTag(row rowScanner, t *nfcTag) error {
	return row.Scan(&t.ID, &t.PublicID, &t.UID, &t.Label, &t.CardID, &t.UserID, &t.OrganizationID, &t.RedirectToken, &t.Status, &t.TapCount, &t.LastTappedAt, &t.AssignedAt, &t.CreatedAt, &t.UpdatedAt, &t.CardName, &t.CardSlug, &t.OwnerName, &t.OwnerEmail, &t.Organization)
}

func (h *NFCHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": item})
}
func (h *NFCHandler) get(ctx context.Context, r *http.Request, id int64) (nfcTag, error) {
	var t nfcTag
	err := scanNFCTag(h.DB.QueryRow(ctx, `SELECT n.id,n.public_id::text,n.uid,n.label,n.card_id,n.user_id,n.organization_id,n.redirect_token,n.status,n.tap_count,n.last_tapped_at,n.assigned_at,n.created_at,n.updated_at,COALESCE(c.name,''),COALESCE(c.slug,''),COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'') FROM nfc_tags n LEFT JOIN digital_cards c ON c.id=n.card_id LEFT JOIN users u ON u.id=n.user_id LEFT JOIN organizations o ON o.id=n.organization_id WHERE n.id=$1`, id), &t)
	if err != nil {
		return t, err
	}
	t.RedirectURL = nfcURL(r, t.RedirectToken)
	return t, nil
}

type nfcInput struct {
	UID    string `json:"uid"`
	Label  string `json:"label"`
	CardID *int64 `json:"card_id"`
	Status string `json:"status"`
}

func (h *NFCHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input nfcInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	input.UID = strings.TrimSpace(input.UID)
	input.Label = strings.TrimSpace(input.Label)
	if input.UID == "" || len(input.UID) > 255 {
		writeJSON(w, 400, map[string]string{"error": "NFC UID is required and must be 255 characters or fewer"})
		return
	}
	status := input.Status
	if status == "" {
		if input.CardID == nil {
			status = "unassigned"
		} else {
			status = "active"
		}
	}
	if !nfcStatuses[status] {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC status"})
		return
	}
	if input.CardID == nil && status == "active" {
		writeJSON(w, 400, map[string]string{"error": "An active NFC tag must be assigned to a Digital Card"})
		return
	}
	if input.CardID != nil && status == "unassigned" {
		writeJSON(w, 400, map[string]string{"error": "Choose a different status for an assigned NFC tag"})
		return
	}
	var userID, orgID *int64
	var cardName, slug, ownerName, ownerEmail, orgName string
	if input.CardID != nil {
		err := h.DB.QueryRow(r.Context(), `SELECT c.user_id,c.organization_id,c.name,c.slug,u.name,u.email,COALESCE(o.name,'') FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id WHERE c.id=$1`, *input.CardID).Scan(&userID, &orgID, &cardName, &slug, &ownerName, &ownerEmail, &orgName)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "Digital card not found"})
			return
		}
	}
	token, err := randomNFCToken()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not generate NFC redirect token"})
		return
	}
	var id int64
	assigned := input.CardID != nil
	var assignedAt interface{}
	if assigned {
		assignedAt = time.Now()
	}
	err = h.DB.QueryRow(r.Context(), `INSERT INTO nfc_tags(uid,label,card_id,user_id,organization_id,redirect_token,status,assigned_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, input.UID, input.Label, input.CardID, userID, orgID, token, status, assignedAt).Scan(&id)
	if err != nil {
		if isDuplicateUID(err) {
			writeJSON(w, 409, map[string]string{"error": "NFC UID already exists"})
			return
		}
		writeJSON(w, 500, map[string]string{"error": "Could not create NFC tag"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "NFC tag created but could not be loaded"})
		return
	}
	auditAdmin(r, h.DB, "nfc_tag_created", "nfc_tag", id, "Created NFC tag")
	writeJSON(w, 201, map[string]interface{}{"data": item})
}

func (h *NFCHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	var input struct {
		Label  string `json:"label"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	input.Label = strings.TrimSpace(input.Label)
	if !nfcStatuses[input.Status] || input.Status == "unassigned" {
		writeJSON(w, 400, map[string]string{"error": "Invalid status for an edit"})
		return
	}
	var cardID *int64
	if err := h.DB.QueryRow(r.Context(), `SELECT card_id FROM nfc_tags WHERE id=$1`, id).Scan(&cardID); err != nil {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	if input.Status == "active" && cardID == nil {
		writeJSON(w, 400, map[string]string{"error": "Assign this NFC tag to a Digital Card before enabling it"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET label=$1,status=$2,updated_at=NOW() WHERE id=$3`, input.Label, input.Status, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update NFC tag"})
		return
	}
	if result.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC tag"})
		return
	}
	auditAdmin(r, h.DB, "nfc_tag_updated", "nfc_tag", id, "Updated NFC tag")
	writeJSON(w, 200, map[string]interface{}{"data": item})
}

func (h *NFCHandler) Assign(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	var input struct {
		CardID int64 `json:"card_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.CardID < 1 {
		writeJSON(w, 400, map[string]string{"error": "A valid Digital Card is required"})
		return
	}
	var userID int64
	var orgID *int64
	err = h.DB.QueryRow(r.Context(), `SELECT user_id,organization_id FROM digital_cards WHERE id=$1`, input.CardID).Scan(&userID, &orgID)
	if err != nil && err != pgx.ErrNoRows {
		writeJSON(w, 500, map[string]string{"error": "Could not validate Digital Card"})
		return
	}
	if err == pgx.ErrNoRows {
		writeJSON(w, 400, map[string]string{"error": "Digital card not found"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET card_id=$1,user_id=$2,organization_id=$3,assigned_at=NOW(),status='active',updated_at=NOW() WHERE id=$4`, input.CardID, userID, orgID, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not assign NFC tag"})
		return
	}
	if result.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC tag"})
		return
	}
	auditAdmin(r, h.DB, "nfc_tag_assigned", "nfc_tag", id, "Assigned NFC tag to a digital card")
	writeJSON(w, 200, map[string]interface{}{"data": item})
}

func (h *NFCHandler) Unassign(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET card_id=NULL,user_id=NULL,organization_id=NULL,assigned_at=NULL,status='unassigned',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not unassign NFC tag"})
		return
	}
	if result.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC tag"})
		return
	}
	auditAdmin(r, h.DB, "nfc_tag_unassigned", "nfc_tag", id, "Unassigned NFC tag from a digital card")
	writeJSON(w, 200, map[string]interface{}{"data": item})
}

func (h *NFCHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	res, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET status='archived',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not archive NFC tag"})
		return
	}
	if res.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	auditAdmin(r, h.DB, "nfc_tag_archived", "nfc_tag", id, "Archived NFC tag")
	writeJSON(w, 200, map[string]string{"message": "NFC tag archived"})
}

func (h *NFCHandler) Taps(w http.ResponseWriter, r *http.Request) {
	id, err := nfcID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid NFC tag ID"})
		return
	}
	var exists bool
	if err := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM nfc_tags WHERE id=$1)`, id).Scan(&exists); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC activity"})
		return
	}
	if !exists {
		writeJSON(w, 404, map[string]string{"error": "NFC tag not found"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT created_at,metadata->>'source',metadata->>'user_agent' FROM analytics_events WHERE event_type='nfc_tap' AND metadata->>'nfc_tag_id'=$1 ORDER BY created_at DESC LIMIT 50`, strconv.FormatInt(id, 10))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load NFC activity"})
		return
	}
	defer rows.Close()
	type tap struct {
		Timestamp time.Time `json:"timestamp"`
		Source    string    `json:"source"`
		UserAgent string    `json:"user_agent"`
	}
	data := []tap{}
	for rows.Next() {
		var t tap
		if err := rows.Scan(&t.Timestamp, &t.Source, &t.UserAgent); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read NFC activity"})
			return
		}
		data = append(data, t)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read NFC activity"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": data})
}

func (h *NFCHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "NFC service unavailable", 500)
		return
	}
	defer tx.Rollback(r.Context())
	var id, cardID, userID int64
	var orgID *int64
	var slug string
	testOnly := r.URL.Query().Get("test") == "1"
	if testOnly {
		err = tx.QueryRow(r.Context(), `SELECT n.id,n.card_id,COALESCE(n.user_id,c.user_id),n.organization_id,c.slug FROM nfc_tags n JOIN digital_cards c ON c.id=n.card_id WHERE n.redirect_token=$1 AND n.status='active' AND c.status='published'`, token).Scan(&id, &cardID, &userID, &orgID, &slug)
	} else {
		err = tx.QueryRow(r.Context(), `UPDATE nfc_tags n SET tap_count=n.tap_count+1,last_tapped_at=NOW(),updated_at=NOW() FROM digital_cards c WHERE n.redirect_token=$1 AND n.status='active' AND n.card_id IS NOT NULL AND c.id=n.card_id AND c.status='published' RETURNING n.id,n.card_id,n.user_id,n.organization_id,c.slug`, token).Scan(&id, &cardID, &userID, &orgID, &slug)
	}
	if err != nil {
		var status string
		var assignedCard *int64
		lookup := tx.QueryRow(r.Context(), `SELECT status,card_id FROM nfc_tags WHERE redirect_token=$1`, token).Scan(&status, &assignedCard)
		if lookup != nil {
			http.NotFound(w, r)
		} else if status != "active" {
			http.Error(w, "NFC tag is not active", http.StatusGone)
		} else if assignedCard == nil {
			http.Error(w, "NFC tag is unassigned", http.StatusConflict)
		} else {
			http.Error(w, "NFC destination unavailable", http.StatusGone)
		}
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "NFC service unavailable", 500)
		return
	}
	if !testOnly {
		metadata, _ := json.Marshal(map[string]string{"nfc_tag_id": strconv.FormatInt(id, 10), "organization_id": nullableIDString(orgID), "source": "nfc_redirect", "user_agent": r.UserAgent()})
		recordPublicEventAsync(h.DB, "nfc_tap", cardID, userID, string(metadata))
	}
	http.Redirect(w, r, defaultCardURL(r, slug), http.StatusFound)
}

func nfcURL(r *http.Request, token string) string {
	base := strings.TrimRight(strings.TrimSpace(osEnv("API_PUBLIC_URL")), "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/n/" + url.PathEscape(token)
}
func randomNFCToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func nfcID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}
func isDuplicateUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
func nullableIDString(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}
func osEnv(key string) string { return strings.TrimSpace(os.Getenv(key)) }
