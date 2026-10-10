package customer

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"digital-card-platform/backend/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// CustomerNFCItem is the customer-safe projection of an NFC record. It omits
// internal ownership IDs and the raw redirect token; redirect_url is the
// customer-facing URL needed to configure compatible NFC hardware.
type CustomerNFCItem struct {
	ID           int64      `json:"id"`
	PublicID     string     `json:"public_id"`
	Name         string     `json:"name"`
	TagHint      string     `json:"tag_hint"`
	CardID       int64      `json:"card_id"`
	CardName     string     `json:"card_name"`
	CardSlug     string     `json:"card_slug"`
	CardStatus   string     `json:"card_status"`
	TemplateName string     `json:"template_name"`
	Status       string     `json:"status"`
	TapCount     int64      `json:"tap_count"`
	LastTappedAt *time.Time `json:"last_tapped_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	RedirectURL  string     `json:"redirect_url"`
}

type customerNFCInput struct {
	Name   string `json:"name"`
	TagUID string `json:"tag_uid"`
	CardID int64  `json:"card_id"`
	Status string `json:"status"`
}

func (h *Handler) NFCList(w http.ResponseWriter, r *http.Request) {
	userID, ok := nfcCustomerID(r)
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "A valid user session is required"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT n.id,n.public_id::text,n.label,n.uid,n.card_id,c.name,c.slug,c.status,COALESCE(t.name,''),n.status,n.tap_count,n.last_tapped_at,n.created_at,n.updated_at,n.redirect_token FROM nfc_tags n JOIN digital_cards c ON c.id=n.card_id LEFT JOIN website_templates t ON t.id=c.template_id WHERE n.user_id=$1 AND n.status<>'archived' ORDER BY n.created_at DESC,n.id DESC LIMIT 200`, userID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not load NFC tags"})
		return
	}
	defer rows.Close()
	items := make([]CustomerNFCItem, 0)
	for rows.Next() {
		var item CustomerNFCItem
		var token, uid string
		if err := rows.Scan(&item.ID, &item.PublicID, &item.Name, &uid, &item.CardID, &item.CardName, &item.CardSlug, &item.CardStatus, &item.TemplateName, &item.Status, &item.TapCount, &item.LastTappedAt, &item.CreatedAt, &item.UpdatedAt, &token); err != nil {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not read NFC tags"})
			return
		}
		item.TagHint = maskNFCUID(uid)
		item.RedirectURL = customerNFCRedirectURL(r, token)
		items = append(items, item)
	}
	if rows.Err() != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not read NFC tags"})
		return
	}
	var limit int
	err = h.DB.QueryRow(r.Context(), `SELECT CASE WHEN COALESCE(NULLIF(p.limits->>'nfc_tags_unlimited','')::boolean,false) THEN -1 ELSE COALESCE(NULLIF(p.limits->>'nfc_tags_limit','')::int,0) END FROM subscriptions s JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=$1 AND s.status IN ('active','trial','trialing','past_due') AND (s.ends_at IS NULL OR s.ends_at>NOW()) ORDER BY s.created_at DESC,s.id DESC LIMIT 1`, userID).Scan(&limit)
	if errors.Is(err, pgx.ErrNoRows) {
		limit = 0
	} else if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not load the NFC allowance"})
		return
	}
	var used int
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM nfc_tags WHERE user_id=$1 AND status<>'archived'`, userID).Scan(&used); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not load NFC usage"})
		return
	}
	respond(w, http.StatusOK, map[string]any{"data": items, "limit": limit, "used": used})
}

func (h *Handler) NFCCreate(w http.ResponseWriter, r *http.Request) {
	userID, ok := nfcCustomerID(r)
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "A valid user session is required"})
		return
	}
	var input customerNFCInput
	if !decodeCustomerNFC(w, r, &input) {
		return
	}
	if err := validateCustomerNFC(&input); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not create NFC tag"})
		return
	}
	defer tx.Rollback(r.Context())
	// Serialize allowance checks for this customer to prevent parallel requests
	// from creating more tags than their plan allows.
	if _, err = tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock($1)`, userID); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate the NFC allowance"})
		return
	}
	if err = enforceNFCLimit(r, tx, userID); err != nil {
		writeNFCError(w, err)
		return
	}
	var cardStatus string
	err = tx.QueryRow(r.Context(), `SELECT status FROM digital_cards WHERE id=$1 AND user_id=$2`, input.CardID, userID).Scan(&cardStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		respond(w, http.StatusNotFound, map[string]string{"error": "Digital card not found"})
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate digital card"})
		return
	}
	if cardStatus != "published" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Choose a published digital card for NFC sharing"})
		return
	}
	uid := input.TagUID
	if uid == "" {
		generatedUID, tokenErr := randomCustomerNFCToken()
		if tokenErr != nil {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not generate tag identifier"})
			return
		}
		uid = "dbizcard:" + generatedUID
	}
	if len(uid) > 200 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Tag identifier must be 200 characters or fewer"})
		return
	}
	token, err := randomCustomerNFCToken()
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not generate NFC redirect URL"})
		return
	}
	status := input.Status
	var id int64
	err = tx.QueryRow(r.Context(), `INSERT INTO nfc_tags(uid,label,card_id,user_id,organization_id,redirect_token,status,assigned_at) SELECT $1,$2,c.id,c.user_id,c.organization_id,$3,$4,NOW() FROM digital_cards c WHERE c.id=$5 AND c.user_id=$6 RETURNING id`, uid, input.Name, token, status, input.CardID, userID).Scan(&id)
	if err != nil {
		if isDuplicateCustomerNFCUID(err) {
			respond(w, http.StatusConflict, map[string]string{"error": "That tag identifier is already registered"})
		} else {
			respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not create NFC tag"})
		}
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not save NFC tag"})
		return
	}
	item, err := h.customerNFCByID(r, userID, id)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "NFC tag was saved but could not be loaded"})
		return
	}
	respond(w, http.StatusCreated, map[string]any{"data": item})
}

func (h *Handler) NFCUpdate(w http.ResponseWriter, r *http.Request) {
	userID, ok := nfcCustomerID(r)
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "A valid user session is required"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid NFC tag"})
		return
	}
	var input customerNFCInput
	if !decodeCustomerNFC(w, r, &input) {
		return
	}
	if err := validateCustomerNFC(&input); err != nil {
		respond(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var cardStatus string
	err = h.DB.QueryRow(r.Context(), `SELECT status FROM digital_cards WHERE id=$1 AND user_id=$2`, input.CardID, userID).Scan(&cardStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		respond(w, http.StatusNotFound, map[string]string{"error": "Digital card not found"})
		return
	}
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate digital card"})
		return
	}
	if cardStatus != "published" {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Choose a published digital card for NFC sharing"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET label=$1,card_id=$2,status=$3,assigned_at=CASE WHEN card_id IS DISTINCT FROM $2 THEN NOW() ELSE assigned_at END,updated_at=NOW() WHERE id=$4 AND user_id=$5 AND status<>'archived'`, input.Name, input.CardID, input.Status, id, userID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not update NFC tag"})
		return
	}
	if result.RowsAffected() == 0 {
		respond(w, http.StatusNotFound, map[string]string{"error": "NFC tag not found"})
		return
	}
	item, err := h.customerNFCByID(r, userID, id)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not load NFC tag"})
		return
	}
	respond(w, http.StatusOK, map[string]any{"data": item})
}

func (h *Handler) NFCArchive(w http.ResponseWriter, r *http.Request) {
	userID, ok := nfcCustomerID(r)
	if !ok {
		respond(w, http.StatusUnauthorized, map[string]string{"error": "A valid user session is required"})
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid NFC tag"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE nfc_tags SET status='archived',updated_at=NOW() WHERE id=$1 AND user_id=$2 AND status<>'archived'`, id, userID)
	if err != nil {
		respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not archive NFC tag"})
		return
	}
	if result.RowsAffected() == 0 {
		respond(w, http.StatusNotFound, map[string]string{"error": "NFC tag not found"})
		return
	}
	respond(w, http.StatusOK, map[string]string{"message": "NFC tag archived"})
}

func (h *Handler) customerNFCByID(r *http.Request, userID, id int64) (CustomerNFCItem, error) {
	var item CustomerNFCItem
	var token, uid string
	err := h.DB.QueryRow(r.Context(), `SELECT n.id,n.public_id::text,n.label,n.uid,n.card_id,c.name,c.slug,c.status,COALESCE(t.name,''),n.status,n.tap_count,n.last_tapped_at,n.created_at,n.updated_at,n.redirect_token FROM nfc_tags n JOIN digital_cards c ON c.id=n.card_id LEFT JOIN website_templates t ON t.id=c.template_id WHERE n.id=$1 AND n.user_id=$2`, id, userID).Scan(&item.ID, &item.PublicID, &item.Name, &uid, &item.CardID, &item.CardName, &item.CardSlug, &item.CardStatus, &item.TemplateName, &item.Status, &item.TapCount, &item.LastTappedAt, &item.CreatedAt, &item.UpdatedAt, &token)
	if err != nil {
		return item, err
	}
	item.TagHint = maskNFCUID(uid)
	item.RedirectURL = customerNFCRedirectURL(r, token)
	return item, nil
}

func maskNFCUID(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "Not provided"
	}
	if strings.HasPrefix(uid, "dbizcard:") {
		return "Auto ID"
	}
	if len(uid) <= 2 {
		return "••••" + uid
	}
	return "••••" + uid[len(uid)-2:]
}

func decodeCustomerNFC(w http.ResponseWriter, r *http.Request, input *customerNFCInput) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(input) != nil || decoder.Decode(new(any)) != io.EOF {
		respond(w, http.StatusBadRequest, map[string]string{"error": "Invalid NFC tag request"})
		return false
	}
	return true
}

func validateCustomerNFC(input *customerNFCInput) error {
	input.Name = strings.TrimSpace(input.Name)
	input.TagUID = strings.TrimSpace(input.TagUID)
	input.Status = strings.TrimSpace(input.Status)
	if input.Name == "" || len(input.Name) > 150 {
		return errors.New("Tag name is required and must be 150 characters or fewer")
	}
	if len(input.TagUID) > 200 {
		return errors.New("Tag identifier must be 200 characters or fewer")
	}
	if input.CardID < 1 {
		return errors.New("Choose a published digital card")
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Status != "active" && input.Status != "disabled" {
		return errors.New("Status must be active or disabled")
	}
	return nil
}

func enforceNFCLimit(r *http.Request, tx pgx.Tx, userID int64) error {
	var limit int
	err := tx.QueryRow(r.Context(), `SELECT CASE WHEN COALESCE(NULLIF(p.limits->>'nfc_tags_unlimited','')::boolean,false) THEN -1 ELSE COALESCE(NULLIF(p.limits->>'nfc_tags_limit','')::int,0) END FROM subscriptions s JOIN website_pricing_plans p ON p.id=s.plan_id WHERE s.user_id=$1 AND s.status IN ('active','trial','trialing','past_due') AND (s.ends_at IS NULL OR s.ends_at>NOW()) ORDER BY s.created_at DESC,s.id DESC LIMIT 1`, userID).Scan(&limit)
	if errors.Is(err, pgx.ErrNoRows) {
		limit = 0
	} else if err != nil {
		return err
	}
	if limit == 0 {
		return errNFCLimit
	}
	if limit < -1 {
		return errNFCLimit
	}
	if limit == -1 {
		return nil
	}
	var used int
	if err := tx.QueryRow(r.Context(), `SELECT COUNT(*) FROM nfc_tags WHERE user_id=$1 AND status<>'archived'`, userID).Scan(&used); err != nil {
		return err
	}
	if used >= limit {
		return errNFCLimit
	}
	return nil
}

var errNFCLimit = errors.New("You have reached the NFC tag limit for your current plan.")

func writeNFCError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNFCLimit) {
		respond(w, http.StatusConflict, map[string]string{"error": errNFCLimit.Error(), "upgrade_url": "/dashboard/billing"})
		return
	}
	respond(w, http.StatusInternalServerError, map[string]string{"error": "Could not validate the NFC allowance"})
}

func nfcCustomerID(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(middleware.UserIDKey).(int64)
	return id, ok && id > 0
}

func randomCustomerNFCToken() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func customerNFCRedirectURL(r *http.Request, token string) string {
	base := publicAPIOrigin(os.Getenv("API_PUBLIC_URL"))
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/n/" + url.PathEscape(token)
}

func publicAPIOrigin(value string) string {
	value = strings.TrimSpace(value)
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return ""
	}
	return strings.TrimRight(u.String(), "/")
}

func isDuplicateCustomerNFCUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
