package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	qrcode "github.com/skip2/go-qrcode"
)

type QRCodeHandler struct{ DB *pgxpool.Pool }

type qrCode struct {
	ID             int64      `json:"id"`
	PublicID       string     `json:"public_id"`
	CardID         *int64     `json:"card_id"`
	UserID         *int64     `json:"user_id"`
	OrganizationID *int64     `json:"organization_id"`
	Token          string     `json:"token"`
	Label          string     `json:"label"`
	DestinationURL string     `json:"destination_url"`
	RedirectURL    string     `json:"redirect_url"`
	QRImage        string     `json:"qr_image"`
	Status         string     `json:"status"`
	ScanCount      int64      `json:"scan_count"`
	LastScannedAt  *time.Time `json:"last_scanned_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	CardName       string     `json:"card_name"`
	CardSlug       string     `json:"card_slug"`
	OwnerName      string     `json:"owner_name"`
	OwnerEmail     string     `json:"owner_email"`
	Organization   string     `json:"organization"`
}

func (h *QRCodeHandler) Summary(w http.ResponseWriter, r *http.Request) {
	var total, active, scans, today int64
	err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*),COUNT(*) FILTER(WHERE status='active'),COALESCE(SUM(scan_count),0) FROM qr_codes`).Scan(&total, &active, &scans)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load QR code summary"})
		return
	}
	err = h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM analytics_events WHERE event_type='qr_scan' AND created_at >= CURRENT_DATE`).Scan(&today)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load today's scans"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": map[string]int64{"total_qr_codes": total, "active_qr_codes": active, "total_scans": scans, "scans_today": today}})
}

func (h *QRCodeHandler) List(w http.ResponseWriter, r *http.Request) {
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
	cardID := strings.TrimSpace(r.URL.Query().Get("card_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	where := `($1='' OR q.status=$1) AND ($2='' OR q.card_id::text=$2) AND ($3='' OR q.user_id::text=$3) AND ($4='' OR q.label ILIKE $5 OR q.code_value ILIKE $5 OR c.name ILIKE $5 OR u.name ILIKE $5 OR u.email ILIKE $5 OR q.destination_url ILIKE $5)`
	like := "%" + search + "%"
	var total int64
	if err := h.DB.QueryRow(r.Context(), `SELECT COUNT(*) FROM qr_codes q LEFT JOIN digital_cards c ON c.id=q.card_id LEFT JOIN users u ON u.id=q.user_id WHERE `+where, status, cardID, userID, search, like).Scan(&total); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not count QR codes"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT q.id,q.public_id::text,q.card_id,q.user_id,q.organization_id,q.code_value,q.label,q.destination_url,q.status,q.scan_count,q.last_scanned_at,q.created_at,q.updated_at,COALESCE(c.name,''),COALESCE(c.slug,''),COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'')
	FROM qr_codes q LEFT JOIN digital_cards c ON c.id=q.card_id LEFT JOIN users u ON u.id=q.user_id LEFT JOIN organizations o ON o.id=q.organization_id WHERE `+where+` ORDER BY q.created_at DESC,q.id DESC LIMIT $6 OFFSET $7`, status, cardID, userID, search, like, limit, (page-1)*limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load QR codes"})
		return
	}
	defer rows.Close()
	items := []qrCode{}
	for rows.Next() {
		var q qrCode
		if err := rows.Scan(&q.ID, &q.PublicID, &q.CardID, &q.UserID, &q.OrganizationID, &q.Token, &q.Label, &q.DestinationURL, &q.Status, &q.ScanCount, &q.LastScannedAt, &q.CreatedAt, &q.UpdatedAt, &q.CardName, &q.CardSlug, &q.OwnerName, &q.OwnerEmail, &q.Organization); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read QR codes"})
			return
		}
		if err := decorateQR(r.Context(), r, &q); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not generate QR preview"})
			return
		}
		items = append(items, q)
	}
	if rows.Err() != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not read QR codes"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": items, "pagination": map[string]int{"page": page, "limit": limit, "total": int(total), "pages": int((total + int64(limit) - 1) / int64(limit))}})
}

func (h *QRCodeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := qrID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid QR code ID"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "QR code not found"})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"data": item})
}

func (h *QRCodeHandler) get(ctx context.Context, r *http.Request, id int64) (qrCode, error) {
	var q qrCode
	err := h.DB.QueryRow(ctx, `SELECT q.id,q.public_id::text,q.card_id,q.user_id,q.organization_id,q.code_value,q.label,q.destination_url,q.status,q.scan_count,q.last_scanned_at,q.created_at,q.updated_at,COALESCE(c.name,''),COALESCE(c.slug,''),COALESCE(u.name,''),COALESCE(u.email,''),COALESCE(o.name,'')
	FROM qr_codes q LEFT JOIN digital_cards c ON c.id=q.card_id LEFT JOIN users u ON u.id=q.user_id LEFT JOIN organizations o ON o.id=q.organization_id WHERE q.id=$1`, id).Scan(&q.ID, &q.PublicID, &q.CardID, &q.UserID, &q.OrganizationID, &q.Token, &q.Label, &q.DestinationURL, &q.Status, &q.ScanCount, &q.LastScannedAt, &q.CreatedAt, &q.UpdatedAt, &q.CardName, &q.CardSlug, &q.OwnerName, &q.OwnerEmail, &q.Organization)
	if err != nil {
		return q, err
	}
	err = decorateQR(ctx, r, &q)
	return q, err
}

type qrInput struct {
	CardID         int64  `json:"card_id"`
	Label          string `json:"label"`
	DestinationURL string `json:"destination_url"`
	Status         string `json:"status"`
}

func (h *QRCodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input qrInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	input.Label = strings.TrimSpace(input.Label)
	if input.CardID < 1 {
		writeJSON(w, 400, map[string]string{"error": "A digital card is required"})
		return
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if !validQRStatus(input.Status) {
		writeJSON(w, 400, map[string]string{"error": "Status must be active, disabled, or archived"})
		return
	}
	var ownerID int64
	var orgID *int64
	var slug string
	var cardName, ownerName, ownerEmail, orgName string
	err := h.DB.QueryRow(r.Context(), `SELECT c.user_id,c.organization_id,c.slug,c.name,u.name,u.email,COALESCE(o.name,'') FROM digital_cards c JOIN users u ON u.id=c.user_id LEFT JOIN organizations o ON o.id=c.organization_id WHERE c.id=$1`, input.CardID).Scan(&ownerID, &orgID, &slug, &cardName, &ownerName, &ownerEmail, &orgName)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Digital card not found"})
		return
	}
	destination := strings.TrimSpace(input.DestinationURL)
	if destination == "" {
		destination = defaultCardURL(r, slug)
	}
	if !validDestination(destination) {
		writeJSON(w, 400, map[string]string{"error": "Destination URL must be an absolute HTTP or HTTPS URL"})
		return
	}
	token, err := randomQRToken()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not generate QR token"})
		return
	}
	var id int64
	err = h.DB.QueryRow(r.Context(), `INSERT INTO qr_codes(code_value,card_id,user_id,organization_id,destination_url,label,status,updated_at) VALUES($1,$2,$3,$4,$5,$6,$7,NOW()) RETURNING id`, token, input.CardID, ownerID, orgID, destination, input.Label, input.Status).Scan(&id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not create QR code"})
		return
	}
	cardID := input.CardID
	item := qrCode{ID: id, CardID: &cardID, UserID: &ownerID, OrganizationID: orgID, Token: token, Label: input.Label, DestinationURL: destination, Status: input.Status, CardName: cardName, CardSlug: slug, OwnerName: ownerName, OwnerEmail: ownerEmail, Organization: orgName}
	item, err = h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "QR code created but could not be loaded"})
		return
	}
	auditAdmin(r, h.DB, "qr_code_created", "qr_code", id, "Created QR code for a digital card")
	writeJSON(w, 201, map[string]interface{}{"data": item})
}

func (h *QRCodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := qrID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid QR code ID"})
		return
	}
	var input qrInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	input.Label = strings.TrimSpace(input.Label)
	if !validQRStatus(input.Status) {
		writeJSON(w, 400, map[string]string{"error": "Status must be active, disabled, or archived"})
		return
	}
	if !validDestination(input.DestinationURL) {
		writeJSON(w, 400, map[string]string{"error": "Destination URL must be an absolute HTTP or HTTPS URL"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE qr_codes SET label=$1,destination_url=$2,status=$3,updated_at=NOW() WHERE id=$4`, input.Label, strings.TrimSpace(input.DestinationURL), input.Status, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not update QR code"})
		return
	}
	if result.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "QR code not found"})
		return
	}
	item, err := h.get(r.Context(), r, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load QR code"})
		return
	}
	auditAdmin(r, h.DB, "qr_code_updated", "qr_code", id, "Updated QR code")
	writeJSON(w, 200, map[string]interface{}{"data": item})
}

func (h *QRCodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := qrID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid QR code ID"})
		return
	}
	result, err := h.DB.Exec(r.Context(), `UPDATE qr_codes SET status='archived',updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not archive QR code"})
		return
	}
	if result.RowsAffected() == 0 {
		writeJSON(w, 404, map[string]string{"error": "QR code not found"})
		return
	}
	auditAdmin(r, h.DB, "qr_code_archived", "qr_code", id, "Archived QR code")
	writeJSON(w, 200, map[string]string{"message": "QR code archived"})
}

func (h *QRCodeHandler) Scans(w http.ResponseWriter, r *http.Request) {
	id, err := qrID(r)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "Invalid QR code ID"})
		return
	}
	var exists bool
	if err := h.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM qr_codes WHERE id=$1)`, id).Scan(&exists); err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load QR scans"})
		return
	}
	if !exists {
		writeJSON(w, 404, map[string]string{"error": "QR code not found"})
		return
	}
	rows, err := h.DB.Query(r.Context(), `SELECT created_at,metadata->>'source',metadata->>'user_agent' FROM analytics_events WHERE event_type='qr_scan' AND metadata->>'qr_code_id'=$1 ORDER BY created_at DESC LIMIT 50`, strconv.FormatInt(id, 10))
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": "Could not load QR scans"})
		return
	}
	defer rows.Close()
	type scan struct {
		At        time.Time `json:"timestamp"`
		Source    string    `json:"source"`
		UserAgent string    `json:"user_agent"`
	}
	events := []scan{}
	for rows.Next() {
		var item scan
		if err := rows.Scan(&item.At, &item.Source, &item.UserAgent); err != nil {
			writeJSON(w, 500, map[string]string{"error": "Could not read QR scans"})
			return
		}
		events = append(events, item)
	}
	writeJSON(w, 200, map[string]interface{}{"data": events})
}

func (h *QRCodeHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, "QR service unavailable", 500)
		return
	}
	defer tx.Rollback(r.Context())
	var id int64
	var cardID, userID *int64
	var dest, cardSlug string
	err = tx.QueryRow(r.Context(), `UPDATE qr_codes q SET scan_count=q.scan_count+1,last_scanned_at=NOW(),updated_at=NOW() WHERE q.code_value=$1 AND q.status='active' AND (q.card_id IS NULL OR EXISTS (SELECT 1 FROM digital_cards c WHERE c.id=q.card_id AND c.status='published')) RETURNING q.id,q.card_id,q.user_id,q.destination_url,COALESCE((SELECT c.slug FROM digital_cards c WHERE c.id=q.card_id),'')`, token).Scan(&id, &cardID, &userID, &dest, &cardSlug)
	if err != nil {
		var status string
		lookupErr := tx.QueryRow(r.Context(), `SELECT status FROM qr_codes WHERE code_value=$1`, token).Scan(&status)
		if lookupErr == nil {
			http.Error(w, "QR code is not active", http.StatusGone)
		} else {
			http.NotFound(w, r)
		}
		return
	}
	if !validDestination(dest) {
		http.Error(w, "QR destination is unavailable", http.StatusGone)
		return
	}
	// Older QR codes used /cards/{slug}; route those defaults to the current public page.
	if cardSlug != "" {
		if parsed, parseErr := url.Parse(dest); parseErr == nil && strings.HasSuffix(strings.TrimRight(parsed.Path, "/"), "/cards/"+cardSlug) {
			dest = defaultCardURL(r, cardSlug)
		}
	}
	metadata, _ := json.Marshal(map[string]string{"qr_code_id": strconv.FormatInt(id, 10), "source": "redirect", "user_agent": r.UserAgent()})
	if err = tx.Commit(r.Context()); err != nil {
		http.Error(w, "QR service unavailable", 500)
		return
	}
	recordPublicEventAsync(h.DB, "qr_scan", cardID, userID, string(metadata))
	http.Redirect(w, r, dest, http.StatusFound)
}

func decorateQR(ctx context.Context, r *http.Request, q *qrCode) error {
	q.RedirectURL = redirectURL(r, q.Token)
	image, err := qrcode.Encode(q.RedirectURL, qrcode.Medium, 240)
	if err != nil {
		return err
	}
	q.QRImage = "data:image/png;base64," + base64.StdEncoding.EncodeToString(image)
	return nil
}
func redirectURL(r *http.Request, token string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("API_PUBLIC_URL")), "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + r.Host
	}
	return base + "/q/" + url.PathEscape(token)
}
func defaultCardURL(r *http.Request, slug string) string {
	base := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(os.Getenv("FRONTEND_URL")), "/")
	}
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/c/" + url.PathEscape(slug)
}
func validDestination(value string) bool {
	u, err := url.ParseRequestURI(value)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
func validQRStatus(status string) bool {
	return status == "active" || status == "disabled" || status == "archived"
}
func randomQRToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
func qrID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id")
	}
	return id, nil
}
